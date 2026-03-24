package dm

import (
	"mydb/src/main/backend/utils"
	"sync"
)

type DataItem interface {
	Data() []byte  //Data以共享形式返回该dataitem的数据内容
	UUID() utils.UUID //handle返回该dataitem的handle

	Before()
	UnBefore()
	After()
	Release()

	//锁操作
	Lock()
	Unlock()
	RLock()
	RUnlock()

}

const (
	_OF_VALID_FLAG = 0
	_OF_DATA_SIZE = 1
	_OF_DATA = 3
)

type dataItem struct {
	raw []byte
	oldraw []byte

	rwlock sync.RWMutex
	
	dm *DataManager
	uid utils.UUID
	pg pcacher.Page
}

func WrapDataItemRaw(data []byte) []byte {
	raw := make([]byte, _OF_DATA+len(data))
    utils.PutUint16(raw[_OF_DATA_SIZE:],uint16(len(data)))
	copy(raw[_OF_DATA:],data)
	return raw
}


//将Dataitem标记为非法
func InValidRawDataItem(raw []byte) {
	raw[_OF_VALID_FLAG] = byte(1)
}

//ParseDataItem从pg的offset位移处，解析出相应的dataitem
func ParseDataItem(pg pcacher.Page, offset int,dm *DataManager) *dataItem {
	raw := pg.Data()[offset:]
	size := utils.GetUint16(raw[_OF_DATA_SIZE:])
	length := _OF_DATA + int(size)
	uid := Address2UUID(pg.Pgno(),offset)

	di := &dataItem{
		raw: raw[:length],
		oldraw: make([]byte, length),
		pg: pg,
		uid: uid,
		dm: dm,
	}
	return di
}
