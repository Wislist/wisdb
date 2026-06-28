package logger

import (
	"errors"
	
	"mydb/src/main/backend/utils"
	"os"
	"sync"
)

type Logger interface {
	Log(data []byte) error
	Truncate(x int64) error
	//读取下一个log
	Next() ([]byte, bool, error)
	//将日志指针移动到第一条指针的位置
	Rewind()
	Close() error
}

var (
	ErrBadLogFile = errors.New("Bad log File.")
)

/*
*
 */
const (
	_SEED        = 13331
	_OF_SIZE     = 0
	_OF_CHECKSUM = _OF_SIZE + 4
	_OF_DATA     = _OF_CHECKSUM + 4

	SUFFIX_LOG  = ".log"
)

type logger struct {
	file *os.File
	lock sync.Mutex

	pos       int64
	fileSize  int64
	xChecksum uint32
}

func Open(path string) (*logger, error) {
	file, err := os.OpenFile(path+SUFFIX_LOG, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	lg := new(logger)
	lg.file = file

	err = lg.init()
	if err != nil {
		return nil, err
	}

	return lg, nil
}

func Create(path string) (*logger, error) {
	file, err := os.OpenFile(path+SUFFIX_LOG, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	xChecksum := utils.Uint32ToRaw(0)
	_, err = file.Write(xChecksum)
	if err != nil {
		return nil, err
	}
	err = file.Sync()
	if err != nil {
		return nil, err
	}

	log := new(logger)
	log.file = file
	log.xChecksum = 0

	return log, nil
}

// 这里需要锁 在更新
func (lg *logger) updateXChecksum(log []byte) error {
	lg.xChecksum = calChecksum(lg.xChecksum, log)
	_, err := lg.file.WriteAt(utils.Uint32ToRaw(lg.xChecksum), 0)
	if err != nil {
		return err
	}
	err = lg.file.Sync()
	if err != nil {
		return err
	}
	return nil
}

func calChecksum(accumulation uint32, data []byte) uint32 {
	for _, b := range data {
		accumulation = uint32(b) + accumulation*_SEED
	}
	return accumulation
}

func (lg *logger) Log(data []byte) error {
	//包装日志数据 添加校验和长度头
	log := warpLog(data)

	lg.lock.Lock()
	defer lg.lock.Unlock()

	_, err := lg.file.Write(log)
	if err != nil {
		return err
	}

	// Sync()会在updateXChecksum内进行
	// 更新校验和
	return lg.updateXChecksum(log)
}

func warpLog(data []byte) []byte {

	log := make([]byte, len(data)+_OF_DATA)
	utils.PutUint32(log[_OF_SIZE:], uint32(len(data)))
	copy(log[_OF_DATA:], data)
	checksum := calChecksum(0, data)
	utils.PutUint32(log[_OF_CHECKSUM:], checksum)
	return log
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

	if lg.pos+_OF_DATA >= lg.fileSize {
		return nil, false, nil
	}

	tmp := make([]byte, 4)
	_, err := lg.file.ReadAt(tmp, lg.pos)
	if err != nil {
		return nil, false, err
	}

	size := int64(utils.ParseUint32(tmp))
	if lg.pos+size+_OF_DATA > lg.fileSize {
		return nil, false, err
	}

	log := make([]byte, _OF_DATA+size)
	_, err = lg.file.ReadAt(log, lg.pos)
	if err != nil {
		return nil, false, err
	}

	checksum1 := calChecksum(0, log[_OF_DATA:])
	checksum2 := utils.ParseUint32(log[_OF_CHECKSUM:])
	if checksum1 != checksum2 {
		return nil, false, nil
	}
	lg.pos += int64(len(log))

	return log, true, nil
}

func (lg *logger) Next() ([]byte, bool, error) {
	lg.lock.Lock()
	defer lg.lock.Unlock()

	log, ok, err := lg.next()
	if err != nil {
		return nil, false, err
	}
	if ok == false {
		return nil, false, nil
	}

	return log[_OF_DATA:], true, nil
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

	xChecksum := utils.ParseUint32(raw)
	lg.fileSize = fileSize
	lg.xChecksum = xChecksum

	return lg.checkAndRemoveTail()

}

func (lg *logger) checkAndRemoveTail() error {
	lg.Rewind()

	var xCheckSum uint32
	for {
		log, ok, err := lg.next()
		if err != nil {
			return err
		}
		if ok == false {
			break
		}
		xCheckSum = calChecksum(xCheckSum, log)

	}

	if xCheckSum == lg.xChecksum {
		err := lg.file.Truncate(lg.pos)
		if err != nil {
			return err
		}
		_, err = lg.file.Seek(lg.pos, 0)
		if err != nil {
			return err
		}
		lg.Rewind()
		return nil
	} else {
		return ErrBadLogFile
	}
}



func (lg *logger) Close() error {
	return lg.file.Close()
}
