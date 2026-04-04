package sm

import (
	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)

const (
	_ENTRY_OF_XMIN = 0
	_ENTRY_OF_XMAX = _ENTRY_OF_XMIN + tm.LEN_XID
	_ENTRY_DATA    = _ENTRY_OF_XMAX + tm.LEN_XID
)

type entry struct {
	selfUUID utils.UUID
	dataitem dm.Dataitem

	sm *serializabilityManager
}

func newEntry(sm *serializabilityManager, di dm.Dataitem, uuid utils.UUID) *entry {
	return &entry{
		selfUUID: uuid,
		dataitem: di,
		sm:       sm,
	}
}

func LoadEntry(sm *serializabilityManager, uuid utils.UUID) (*entry, bool, error) {
	di, ok, err := sm.DM.Read(uuid)
	if err != nil {
		return nil, false, err
	}
	if ok == false {
		return nil, false, nil
	}
	return newEntry(sm, di, uuid), true, nil
}

// WrapEntryRaw 将xid和data包裹成entry的二进制数据.
func WrapEntryRaw(xid tm.XID, data []byte) []byte {
	raw := make([]byte, _ENTRY_DATA+len(data))
	tm.PutXID(raw[_ENTRY_OF_XMIN:], xid)
	copy(raw[_ENTRY_DATA:], data)
	return raw
}

// Release 释放一个entry的引用
func (e *entry) Release() {
	e.sm.ReleaseEntry(e)
}

func (e *entry) Remove() {
	e.dataitem.Release()
}

func (e *entry) Data() []byte {
	e.dataitem.RLock()
	defer e.dataitem.RUnlock()
	data := make([]byte, len(e.dataitem.Data())-_ENTRY_DATA)
	copy(data, e.dataitem.Data()[_ENTRY_DATA:])
	return data
}

func (e *entry) XMIN() tm.XID {
	e.dataitem.RLock()
	defer e.dataitem.RUnlock()

	return tm.ParseXID(e.dataitem.Data()[_ENTRY_OF_XMIN:_ENTRY_OF_XMAX])
}

func (e *entry) XMAX() tm.XID {
	e.dataitem.RLock()
	defer e.dataitem.RUnlock()

	return tm.ParseXID(e.dataitem.Data()[_ENTRY_OF_XMAX:])
}

func (e *entry) SetXMAX(xid tm.XID) {
	e.dataitem.Before()
	defer e.dataitem.After(xid)
	tm.PutXID(e.dataitem.Data()[_ENTRY_OF_XMAX:], xid)
}
