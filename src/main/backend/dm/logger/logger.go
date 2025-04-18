package logger

import (
	"bytes"
	"errors"
	"fmt"
	"mydb/src/main/backend/utils"
	"os"
	"sync"
)

type Logger interface {
	Log(data []byte)
	Truncate(x int64) error
	//读取下一个log
	Next() ([]byte, bool)
	//将日志指针移动到第一条指针的位置
	Rewind()
	Close()
}

var (
	ErrBadLogFile = errors.New("Bad log File.")
)

/*
*
 */
const (
	SEED        = 13331
	OF_SIZE     = 0
	OF_CHECKSUM = OF_SIZE + 4
	OF_DATA     = OF_CHECKSUM + 4
	SUFFIX_LOG  = ".log"
)

type logger struct {
	file *os.File
	lock sync.Mutex

	pos       int64
	fileSize  int64
	xChecksum uint32
}

func Open(path string) *logger {
	file, err := os.OpenFile(path+SUFFIX_LOG, os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}

	lg := new(logger)
	lg.file = file

	err = lg.init()
	if err != nil {
		panic(err)
	}

	return lg
}

func Create(path string) *logger {
	file, err := os.OpenFile(path+SUFFIX_LOG, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	xChecksum := utils.Uint32ToBytes(0)
	_, err = file.Write(xChecksum)
	if err != nil {
		panic(err)
	}
	err = file.Sync()
	if err != nil {
		panic(err)
	}

	log := new(logger)
	log.file = file
	log.xChecksum = 0

	return log
}

// 这里需要锁 在更新
func (lg *logger) updateXChecksum(log []byte) {
	lg.xChecksum = calChecksum(lg.xChecksum, log)
	_, err := lg.file.WriteAt(utils.IntToBytes(int32(lg.xChecksum)), 0)
	if err != nil {
		panic(err)
	}
	err = lg.file.Sync()
	if err != nil {
		panic(err)
	}

}

func calChecksum(accumulation uint32, data []byte) uint32 {
	for _, b := range data {
		accumulation = uint32(b) + accumulation*SEED
	}
	return accumulation
}

func (lg *logger) Log(data []byte) {
	//包装日志数据 添加校验和长度头
	log := warpLog(data)

	lg.lock.Lock()
	defer lg.lock.Unlock()

	_, err := lg.file.Write(log)
	//如果这里写入失败 那么程序是没办法继续运行的 日志输入非常重要
	if err != nil {
		panic(err)
	}

	// Sync()会在updateXChecksum内进行
	// 更新校验和
	lg.updateXChecksum(log)
}

func warpLog(data []byte) []byte {

	checksum := utils.IntToBytes(int32(calChecksum(0, data)))
	size := utils.IntToBytes(int32(len(data)))

	var buf bytes.Buffer
	buf.Write(size)
	buf.Write(checksum)
	buf.Write(data)

	return buf.Bytes()
}

func (lg *logger) Truncate(x int64) error {
	lg.lock.Lock()
	defer lg.lock.Unlock()
	return lg.file.Truncate(x)
}

// 用于跳过文件头部
func (lg *logger) Rewind() {
	lg.pos = 4
}

func (lg *logger) next() ([]byte, bool, error) {

	if lg.pos+OF_DATA >= lg.fileSize {
		return nil, false, nil
	}

	tmp := make([]byte, 4)
	_, err := lg.file.ReadAt(tmp, lg.pos)
	if err != nil {
		return nil, false, err
	}

	size := int64(utils.BytesToUint32(tmp))
	if lg.pos+size+OF_DATA > lg.fileSize {
		return nil, false, err
	}

	log := make([]byte, OF_DATA+size)
	_, err = lg.file.ReadAt(log, lg.pos)
	if err != nil {
		return nil, false, err
	}

	checksum1 := calChecksum(0, log[OF_DATA:])
	checksum2 := utils.BytesToUint32(log[OF_CHECKSUM:])
	if checksum1 != checksum2 {
		return nil, false, nil
	}
	lg.pos += int64(len(log))

	return log, true, nil
}

func (lg *logger) Next() ([]byte, bool) {
	lg.lock.Lock()
	defer lg.lock.Unlock()

	log, ok, err := lg.next()
	if err != nil {
		panic(err)
	}
	if ok == false {
		return nil, false
	}

	return log[OF_DATA:], true
}

// init方法
func (lg *logger) init() error {
	info, err := lg.file.Stat()
	if err != nil {
		return err
	}

	fileSize := info.Size()
	if fileSize < 4 {
		return ErrBadLogFile
	}

	raw := make([]byte, 4)
	_, err = lg.file.ReadAt(raw, 0)
	if err != nil {
		return err
	}

	xChecksum := utils.BytesToUint32(raw)
	lg.fileSize = fileSize
	lg.xChecksum = xChecksum

	return lg.checkAndRemoveTail()

}

func (lg *logger) checkAndRemoveTail() error {
	lg.Rewind()

	var xCheckSum uint32
	var lastGoodPos int64 = lg.pos
	for {
		log, ok, err := lg.next()
		if err != nil {
			return err
		}
		if ok != true {
			break
		}
		xCheckSum = calChecksum(xCheckSum, log)
		lastGoodPos = lg.pos
	}

	//原子化校验和更新流程
	/** TODO
	原始文件:
	[HEADER][REC1][REC2][CORRUPTED_TAIL]
	处理后的文件：
	[HEADER][REC1][REC2]  # 截断损坏部分

	*/
	if err := lg.safeUpdateChecksum(lastGoodPos, xCheckSum); err != nil {
		return fmt.Errorf("update checksum failed: %v", err)
	}

	// 当前强制跳过校验和验证(见下方NOTE)
	//if true {
	//	/*
	//		// TODO：用safeUpdateChecksum解决
	//		由于更新xCheckSum的时候数据库发生崩溃, 则会导致整个log文件不能使用.
	//		所以暂时放弃xCheckSum, 之后将xCheckSum改为由booter管理.
	//	*/
	//
	//}
	return lg.file.Truncate(lastGoodPos)

}

func (lg *logger) safeUpdateChecksum(pos int64, sum uint32) error {
	tmpPath := lg.file.Name() + ".tmp"
	if err := os.Link(lg.file.Name(), tmpPath); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	lg.lock.Lock()
	defer lg.lock.Unlock()

	if _, err := lg.file.Seek(0, 0); err != nil {
		return err
	}
	if _, err := lg.file.Write(utils.Uint32ToBytes(sum)); err != nil {
		return err
	}
	return lg.file.Sync()
}

func (lg *logger) Close() {
	err := lg.file.Close()
	if err != nil {
		panic(err)
	}
}
