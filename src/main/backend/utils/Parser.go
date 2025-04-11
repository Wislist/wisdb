package utils

import (
	"encoding/binary"
)

// ShortToBytes converts int16 to byte slice (big-endian)
func ShortToBytes(value int16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(value))
	return buf
}

// BytesToShort converts byte slice to int16 (big-endian)
func BytesToShort(buf []byte) int16 {
	return int16(binary.BigEndian.Uint16(buf))
}

// IntToBytes converts int32 to byte slice (big-endian)
func IntToBytes(value int32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(value))
	return buf
}

// BytesToInt converts byte slice to int32 (big-endian)
func BytesToInt(buf []byte) int32 {
	return int32(binary.BigEndian.Uint32(buf))
}

// LongToBytes converts int64 to byte slice (big-endian)
func LongToBytes(value int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(value))
	return buf
}

// BytesToLong converts byte slice to int64 (big-endian)
func BytesToLong(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}

// ParseStringResult contains parsed string and bytes consumed
type ParseStringResult struct {
	Str  string
	Size int
}

// StringToBytes converts string to byte slice with length prefix
func StringToBytes(str string) []byte {
	length := IntToBytes(uint32(int32(len(str))))
	strBytes := []byte(str)
	return append(length, strBytes...)
}

// BytesToString parses length-prefixed string from byte slice
func BytesToString(raw []byte) ParseStringResult {
	length := BytesToInt(raw[:4])
	str := string(raw[4 : 4+length])
	return ParseStringResult{
		Str:  str,
		Size: 4 + int(length),
	}
}

// StrToUID converts string to unique ID using polynomial hash
func StrToUID(key string) int64 {
	const seed = 13331
	var res int64
	for _, b := range []byte(key) {
		res = res*seed + int64(b)
	}
	return res
}
