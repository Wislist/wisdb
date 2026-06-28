package dm

import (
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
	"sync"
)

type Dataitem interface {
	Data() []byte     //Data以共享形式返回该dataitem的数据内容
	UUID() utils.UUID //handle返回该dataitem的handle

	Before()
	UnBefore()
	After(xid tm.XID)
	Release()

	//锁操作
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

const (
	_OF_VALID_FLAG = 0
	_OF_DATA_SIZE  = 1
	_OF_DATA       = 3
)

type dataitem struct {
	raw    []byte
	oldraw []byte

	rwlock sync.RWMutex

	dm  *dataManager
	uid utils.UUID
	pg  pcacher.Page
}

func WrapDataitemRaw(data []byte) []byte {
	raw := make([]byte, _OF_DATA+len(data))
	utils.PutUint16(raw[_OF_DATA_SIZE:], uint16(len(data)))
	copy(raw[_OF_DATA:], data)
	return raw
}

// 将Dataitem标记为非法
func InValidRawDataitem(raw []byte) {
	raw[_OF_VALID_FLAG] = byte(1)
}

// ParseDataitem从pg的offset位移处，解析出相应的dataitem
func ParseDataitem(pg pcacher.Page, offset Offset, dm *dataManager) *dataitem {
	raw := pg.Data()[offset:]
	size := utils.GetUint16(raw[_OF_DATA_SIZE:])
	length := _OF_DATA + int(size)

	// 防止损坏数据导致越界：size 超出页剩余空间时截断
	if length > len(raw) {
		length = len(raw)
	}

	uid := Address2UUID(pg.Pgno(), Offset(offset))

	di := &dataitem{
		raw:    raw[:length],
		oldraw: make([]byte, length),
		pg:     pg,
		uid:    uid,
		dm:     dm,
	}
	return di
}

func (di *dataitem) IsValid() bool {
	return di.raw[_OF_VALID_FLAG] == byte(0)
}

func (di *dataitem) Data() []byte {
	return di.raw[_OF_DATA:]
}

func (di *dataitem) UUID() utils.UUID {
	return di.uid
}
func (di *dataitem) Before() {
	di.rwlock.Lock()
	di.pg.Dirty()
	copy(di.oldraw, di.raw)
}

func (di *dataitem) UnBefore() {
	copy(di.raw, di.oldraw)
	di.rwlock.Unlock()
}

func (di *dataitem) After(xid tm.XID) {
	di.dm.logDataitem(xid, di)
	di.rwlock.Unlock()
}
func (di *dataitem) Release() {
	di.dm.ReleaseDataitem(di)
}
func (di *dataitem) Lock() {
	di.rwlock.Lock()
}
func (di *dataitem) Unlock() {
	di.rwlock.Unlock()
}
func (di *dataitem) RLock() {
	di.rwlock.RLock()
}
func (di *dataitem) RUnlock() {
	di.rwlock.RUnlock()
}
