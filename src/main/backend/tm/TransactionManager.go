package tm

import (
	"errors"
	"os"
	"sync"

	"mydb/src/main/backend/utils"
)

var (
	ErrBadXIDFile    = errors.New("bad XID file")
	ErrFileExists    = errors.New("file already exists")
	ErrFileCannotRW  = errors.New("file cannot read/write")
	ErrFileNotExists = errors.New("file does not exist")
)

const (
	// XID文件头长度
	LEN_XID_HEADER_LENGTH = 8
	// 每个事务占用的长度
	XID_FIELD_SIZE = 1

	// 表示三种状态
	FIELD_TRAN_ACTIVE    = 0
	FIELD_TRAN_COMMITTED = 1
	FIELD_TRAN_ABORTED   = 2

	// 超级事务
	SUPER_XID  = 0
	XID_SUFFIX = ".xid"
)

type TansactionManager interface {
	Begin() int64
	Commit(xid int64)
	Abort(xid int64)
	IsActive(xid int64) bool
	IsCommitted(xid int64) bool
	IsAborted(xid int64) bool
	Close()
}

type transactionManager struct {
	file *os.File

	xidCounter  int64
	counterLock sync.Mutex
}

func Create(path string) *transactionManager {
	file, err := os.OpenFile(path+XID_SUFFIX, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}

	xidCounterInit := make([]byte, LEN_XID_HEADER_LENGTH)
	_, err = file.WriteAt(xidCounterInit, 0)

	if err != nil {
		panic(err)
	}

	return newTransactionManager(file)
}

func Open(path string) *transactionManager {
	file, err := os.OpenFile(path+XID_SUFFIX, os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
	return newTransactionManager(file)
}

func newTransactionManager(file *os.File) *transactionManager {
	tm := new(transactionManager)
	tm.file = file
	tm.checkXIDCounter()
	return tm
}

func (tm *transactionManager) checkXIDCounter() {
	stat, err := tm.file.Stat()

	if err != nil {
		panic(err)
	}

	if stat.Size() < LEN_XID_HEADER_LENGTH {
		panic(ErrBadXIDFile)
	}

	header := make([]byte, LEN_XID_HEADER_LENGTH)
	_, err = tm.file.ReadAt(header, 0)
	if err != nil {
		panic(err)
	}
	tm.xidCounter = utils.BytesToLong(header)

	end := xidPosition(tm.xidCounter + 1)

	if end != stat.Size() {
		panic(ErrBadXIDFile)
	}
}

func xidPosition(xid int64) int64 {
	return LEN_XID_HEADER_LENGTH + (xid-1)*XID_FIELD_SIZE
}
