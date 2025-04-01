package tm

import (
	"errors"
	"os"
	"sync"

	"mydb/src/main/backend/utils"
)

var (
	ErrBadXIDFile = errors.New("bad XID file")
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

type TransactionManager interface {
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

// 检查XID文件是否合法
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

// 更新事务状态 updateXID
func (t *transactionManager) updateXID(xid int64, status byte) {
	offset := xidPosition(xid)
	tmp := make([]byte, XID_FIELD_SIZE)
	tmp[0] = status
	_, err := t.file.WriteAt(tmp, offset)
	if err != nil {
		panic(err)
	}
	err = t.file.Sync()
	if err != nil {
		panic(err)
	}
}

// XID+1 并更新XID Header incrXIDCounter
func (t *transactionManager) incrXIDCounter() {
	t.xidCounter++
	buf := utils.LongToBytes(t.xidCounter)
	_, err := t.file.WriteAt(buf, 0)
	if err != nil {
		panic(err)
	}
	err = t.file.Sync()
	if err != nil {
		panic(err)
	}
}

/*
*
返回一个XID作为Header
*/
func (t *transactionManager) Begin() int64 {
	//先上锁
	t.counterLock.Lock()
	defer t.counterLock.Unlock()

	xid := t.xidCounter + 1
	//事务置为0 表示开始事务
	t.updateXID(xid, FIELD_TRAN_ACTIVE)
	t.incrXIDCounter()
	return xid
}

/**
commit事务
*/

func (t *transactionManager) Commit(xid int64) {
	//表示提交
	t.counterLock.Lock()
	defer t.counterLock.Unlock()
	t.updateXID(xid, byte(FIELD_TRAN_COMMITTED))
}

/*
*
回滚事务
*/
func (t *transactionManager) Abort(xid int64) {
	t.updateXID(xid, byte(FIELD_TRAN_ABORTED))
}

/*
*checkXID 检查这个xid的status
 */
func (t *transactionManager) checkXID(xid int64, status byte) bool {
	offset := xidPosition(xid)
	tmp := make([]byte, XID_FIELD_SIZE)
	_, err := t.file.ReadAt(tmp, offset)
	if err != nil {
		panic(err)
	}
	return tmp[0] == status
}

func (t *transactionManager) IsActive(xid int64) bool {
	if xid == SUPER_XID {
		return false
	}
	return t.checkXID(xid, FIELD_TRAN_ACTIVE)
}

func (t *transactionManager) IsCommitted(xid int64) bool {
	if xid == SUPER_XID {
		return true
	}
	return t.checkXID(xid, FIELD_TRAN_COMMITTED)
}

func (t *transactionManager) IsAborted(xid int64) bool {
	if xid == SUPER_XID {
		return true
	}
	return t.checkXID(xid, FIELD_TRAN_ABORTED)
}

func (t *transactionManager) Close() {
	err := t.file.Close()
	if err != nil {
		panic(err)
	}
}
