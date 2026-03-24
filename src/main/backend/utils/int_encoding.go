package utils

import (
	"bytes"
	"encoding/binary"
)

func PutUint16(buf []byte, num uint16) {
	b := bytes.NewBuffer(buf[:0])
	_ = binary.Write(b, binary.BigEndian, num)
}

func GetUint16(buf []byte) uint16 {
	return binary.BigEndian.Uint16(buf)
}

func ParseUint16(raw []byte) uint16 {
	return GetUint16(raw)
}

func Uint16ToRaw(num uint16) []byte {
	b := make([]byte, 2)
	buf := bytes.NewBuffer(b[:0])
	_ = binary.Write(buf, binary.BigEndian, num)
	return b
}

func PutUint32(buf []byte, num uint32) {
	b := bytes.NewBuffer(buf[:0])
	_ = binary.Write(b, binary.BigEndian, num)
}

func GetUint32(buf []byte) uint32 {
	return binary.BigEndian.Uint32(buf)
}

func ParseUint32(raw []byte) uint32 {
	return GetUint32(raw)
}

func Uint32ToRaw(num uint32) []byte {
	b := make([]byte, 4)
	buf := bytes.NewBuffer(b[:0])
	_ = binary.Write(buf, binary.BigEndian, num)
	return b
}

func PutUint64(buf []byte, num uint64) {
	b := bytes.NewBuffer(buf[:0])
	_ = binary.Write(b, binary.BigEndian, num)
}

func GetUint64(buf []byte) uint64 {
	return binary.BigEndian.Uint64(buf)
}

func ParseUint64(raw []byte) uint64 {
	return GetUint64(raw)
}

func Uint64ToRaw(num uint64) []byte {
	b := make([]byte, 8)
	buf := bytes.NewBuffer(b[:0])
	_ = binary.Write(buf, binary.BigEndian, num)
	return b
}
