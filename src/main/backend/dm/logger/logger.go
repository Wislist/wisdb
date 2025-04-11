package logger

import (
	"bytes"
	"errors"
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
	xChecksum := utils.IntToBytes(0)
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
	_, err := lg.file.WriteAt(utils.IntToBytes(lg.xChecksum), 0)
	if err != nil {
		panic(err)
	}
	err = lg.file.Sync()
	if err != nil {
		panic(err)
	}

}

func calChecksum(accumulation uint32, data []byte) int32 {
	for _, b := range data {
		accumulation = uint32(b) + accumulation*SEED
	}
	return int32(accumulation)
}

func (lg *logger) Log(data []byte) {
	//包装日志数据 添加校验和长度头
	log := warpLog(data)

	lg.lock.Lock()
	defer lg.lock.Unlock()

	_, err := lg.file.WriteAt(log)
	//如果这里写入失败 那么程序是没办法继续运行的 日志输入非常重要
	if err != nil {
		panic(err)
	}

	// Sync()会在updateXChecksum内进行
	// 更新校验和
	lg.updateXChecksum(log)
}

func warpLog(data []byte) []byte {

	checksum := utils.IntToBytes(calChecksum(0,data))
	size := utils.IntToBytes(int32(len(data)))

	var buf bytes.Buffer
	buf.Write(size)
	buf.Write(checksum)
	buf.Write(data)


	return buf.Bytes()
}


func checkAnd