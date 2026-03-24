package utils



type UUID uint64


var (
	INF UUID = UUID(0xFFFFFFFFFFFFFFFF)
	NilUUID UUID = UUID(0x0000000000000000)
)

const (
	UUID_SIZE = 8
)

func PutUUID(uuid UUID, buf []byte) {
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
