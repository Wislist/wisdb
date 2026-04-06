package utils

import "strconv"

// StrToUUID 将字符串转换为UUID，用了简单的hash算法
// 转化后UUID将无序
func StrToUUID(str string) UUID {
	var seed uint64 = 13331
	var result uint64
	for _, b := range str {
		result = result*seed + uint64(b)
	}
	return UUID(result)
}

func VarStrToRaw(str string) []byte {
	length := len(str)
	raw := Uint32ToRaw(uint32(length))
	raw = append(raw, []byte(str)...)
	return raw
}

func ParseVarStr(raw []byte) (string, int) {
	l := GetUint32(raw[:4])
	size := 4 + int(l)
	return string(raw[4:size]), size
}

func StrToUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func StrToUint32(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	return uint32(v), err
}

func StrToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func Int64ToStr(v int64) string {
	return strconv.FormatInt(v, 10)
}

func Uint32ToStr(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

func Uint64ToStr(num uint64) string {
	return strconv.FormatUint(num, 10)
}

