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
	_XID_FILE_HEADER_SIZE = LEN_XID
	// 每个事务占用的长度
	XID_FIELD_SIZE = 1

	// 表示三种状态
	FIELD_TRAN_ACTIVE    = 0
	FIELD_TRAN_COMMITTED = 1
	FIELD_TRAN_ABORTED   = 2

	// 超级事务
	XID_SUFFIX = ".xid"
)

type TransactionManager interface {
	Begin() XID
	Commit(xid XID)
	Abort(xid XID)
	IsActive(xid XID) bool
	IsCommitted(xid XID) bool
	IsAborted(xid XID) bool
	Close()
}

type transactionManager struct {
	file *os.File

	xidCounter  XID
	counterLock sync.Mutex
}

func Create(path string) *transactionManager {
	file, err := os.OpenFile(path+XID_SUFFIX, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}

	xidCounterInit := make([]byte, LEN_XID)
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

	if stat.Size() < _XID_FILE_HEADER_SIZE {
		panic(ErrBadXIDFile)
	}

	header := make([]byte, _XID_FILE_HEADER_SIZE)
	_, err = tm.file.ReadAt(header, 0)
	if err != nil {
		panic(err)
	}
	tm.xidCounter = XID(utils.ParseUUID(header))

	end, _ := xidPosition(XID(tm.xidCounter + 1))

	if end != stat.Size() {
		panic(ErrBadXIDFile)
	}
}

func xidPosition(xid XID) (int64, int) {
	offset := _XID_FILE_HEADER_SIZE + int64(xid-1)*XID_FIELD_SIZE
	return int64(offset) , XID_FIELD_SIZE
}

// 更新事务状态 updateXID
func (t *transactionManager) updateXID(xid XID, status byte) {
	offset, length  := xidPosition(xid)
	tmp := make([]byte, length)
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
	buf := utils.Uint64ToRaw(uint64(t.xidCounter))
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
func (t *transactionManager) Begin() XID {
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

func (t *transactionManager) Commit(xid XID) {
	//表示提交
	t.counterLock.Lock()
	defer t.counterLock.Unlock()
	t.updateXID(xid, byte(FIELD_TRAN_COMMITTED))
}

/*
*
回滚事务
*/
func (t *transactionManager) Abort(xid XID) {
	t.updateXID(xid, byte(FIELD_TRAN_ABORTED))
}

/*
*checkXID 检查这个xid的status
 */
func (t *transactionManager) checkXID(xid XID, status byte) bool {
	offset, length := xidPosition(xid)
	tmp := make([]byte, length)
	_, err := t.file.ReadAt(tmp, offset)
	if err != nil {
		panic(err)
	}
	return tmp[0] == status
}

func (t *transactionManager) IsActive(xid XID) bool {
	if xid == SUPER_XID {
		return false
	}
	return t.checkXID(xid, FIELD_TRAN_ACTIVE)
}

func (t *transactionManager) IsCommitted(xid XID) bool {
	if xid == SUPER_XID {
		return true
	}
	return t.checkXID(xid, FIELD_TRAN_COMMITTED)
}

func (t *transactionManager) IsAborted(xid XID) bool {
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
