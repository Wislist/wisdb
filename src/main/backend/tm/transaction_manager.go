// 事务管理器
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
	Begin() (XID, error)
	Commit(xid XID) error
	Abort(xid XID) error
	IsActive(xid XID) bool
	IsCommitted(xid XID) bool
	IsAborted(xid XID) bool
	Close() error
}

type transactionManager struct {
	file *os.File

	xidCounter  XID
	counterLock sync.Mutex
}

func Create(path string) (*transactionManager, error) {
	file, err := os.OpenFile(path+XID_SUFFIX, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}

	xidCounterInit := make([]byte, LEN_XID)
	_, err = file.WriteAt(xidCounterInit, 0)

	if err != nil {
		return nil, err
	}

	return newTransactionManager(file)
}

func Open(path string) (*transactionManager, error) {
	file, err := os.OpenFile(path+XID_SUFFIX, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	return newTransactionManager(file)
}

func newTransactionManager(file *os.File) (*transactionManager, error) {
	tm := new(transactionManager)
	tm.file = file
	if err := tm.checkXIDCounter(); err != nil {
		return nil, err
	}
	return tm, nil
}

// 检查XID文件是否合法
func (tm *transactionManager) checkXIDCounter() error {
	stat, err := tm.file.Stat()

	if err != nil {
		return err
	}

	if stat.Size() < _XID_FILE_HEADER_SIZE {
		return ErrBadXIDFile
	}

	header := make([]byte, _XID_FILE_HEADER_SIZE)
	_, err = tm.file.ReadAt(header, 0)
	if err != nil {
		return err
	}
	tm.xidCounter = XID(utils.ParseUUID(header))

	end, _ := xidPosition(XID(tm.xidCounter + 1))

	if end != stat.Size() {
		return ErrBadXIDFile
	}

	return nil
}

func xidPosition(xid XID) (int64, int) {
	offset := _XID_FILE_HEADER_SIZE + int64(xid-1)*XID_FIELD_SIZE
	return int64(offset) , XID_FIELD_SIZE
}

// 更新事务状态 updateXID
func (t *transactionManager) updateXID(xid XID, status byte) error {
	offset, length  := xidPosition(xid)
	tmp := make([]byte, length)
	tmp[0] = status
	_, err := t.file.WriteAt(tmp, offset)
	if err != nil {
		return err
	}
	return t.file.Sync()
}

// XID+1 并更新XID Header incrXIDCounter
func (t *transactionManager) incrXIDCounter() error {
	t.xidCounter++
	buf := utils.Uint64ToRaw(uint64(t.xidCounter))
	_, err := t.file.WriteAt(buf, 0)
	if err != nil {
		return err
	}
	return t.file.Sync()
}

/*
*
返回一个XID作为Header
*/
func (t *transactionManager) Begin() (XID, error) {
	t.counterLock.Lock()
	defer t.counterLock.Unlock()

	xid := t.xidCounter + 1
	if err := t.updateXID(xid, FIELD_TRAN_ACTIVE); err != nil {
		return 0, err
	}
	if err := t.incrXIDCounter(); err != nil {
		return 0, err
	}
	return xid, nil
}

/**
commit事务
*/

func (t *transactionManager) Commit(xid XID) error {
	t.counterLock.Lock()
	defer t.counterLock.Unlock()
	return t.updateXID(xid, byte(FIELD_TRAN_COMMITTED))
}

/*
*
回滚事务
*/
func (t *transactionManager) Abort(xid XID) error {
	t.counterLock.Lock()
	defer t.counterLock.Unlock()
	return t.updateXID(xid, byte(FIELD_TRAN_ABORTED))
}

/*
*checkXID 检查这个xid的status
 */
func (t *transactionManager) checkXID(xid XID, status byte) bool {
	offset, length := xidPosition(xid)
	tmp := make([]byte, length)
	_, err := t.file.ReadAt(tmp, offset)
	if err != nil {
		return false
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

func (t *transactionManager) Close() error {
	return t.file.Close()
}
