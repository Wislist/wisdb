package dm

import (
	"errors"
	"mydb/src/main/backend/utils"
)

var (
	ErrBusy = errors.New("data manager is busy")
	ErrDataTooLarge = errors.New("data is too large")
)

type DataManager interface {
	Read(uid utils.UUID) (DataItem,bool,error)
	Insert(xid tm.XID, data []byte) (utils.UUID,error)

	Close()
}

