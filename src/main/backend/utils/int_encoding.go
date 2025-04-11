package utils

import "bytes"

func PutUint32(buf []byte, num uint32) {
	buffer := bytes.NewBuffer(buf)
	buffer.Reset()

}
