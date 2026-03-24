package tm

import "mydb/src/main/backend/utils"

type XID utils.UUID

const (
	LEN_XID = utils.UUID_SIZE
	SUPER_XID = 0
)

func PutXID(buf []byte, xid XID) {
	utils.PutUUID(utils.UUID(xid), buf)
}

func ParseXID(raw []byte) XID {
	return XID(utils.ParseUUID(raw))
}

func XIDToRaw(xid XID) []byte {
	return utils.UUIDToRaw(utils.UUID(xid))
}