package utils



type UUID uint64


var (
	INF UUID = (1 << 63) - 1 + (1 << 63)
	NilUUID UUID = 0
)

const (
	LEN_UUID = 8
)

func PutUUID(buf []byte,uuid UUID) {
	PutUint64(buf, uint64(uuid))
}
func GetUUID(buf []byte) UUID {
	return UUID(GetUint64(buf))
}
func ParseUUID(raw []byte) UUID{
	return UUID(ParseUint64(raw))
}
func UUIDToRaw(uuid UUID) []byte {
	return Uint64ToRaw(uint64(uuid))
}
