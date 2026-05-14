/*
	wire.go 定义 WisDB 自定义二进制协议（Wire Protocol v1）。

	帧格式：

	  Request Frame:
	    [4B] Magic     : 0x57 0x49 0x53 0x44 ("WISD")
	    [1B] Version   : 0x01
	    [1B] Type      : 0x01 = Request
	    [4B] RequestID : 请求序号，支持 pipeline
	    [4B] BodyLen   : Payload 字节数
	    [NB] Payload   : SQL 语句字节

	  Response Frame:
	    [4B] Magic
	    [1B] Version   : 0x01
	    [1B] Type      : 0x02 = Response
	    [4B] RequestID : 与请求对应
	    [1B] Flag      : 0x00 = OK, 0x01 = Error
	    [4B] BodyLen
	    [NB] Payload   : 结果数据或错误信息字节

	所有多字节整数均为大端序（Big-Endian）。
*/
package transporter

import "errors"

const (
	WireMagic   = uint32(0x57495344) // "WISD"
	WireVersion = byte(0x01)

	WireTypeRequest  = byte(0x01)
	WireTypeResponse = byte(0x02)

	WireFlagOK    = byte(0x00)
	WireFlagError = byte(0x01)

	// 帧头固定长度
	wireReqHeaderLen  = 14 // 4+1+1+4+4
	wireRespHeaderLen = 15 // 4+1+1+4+1+4
)

var (
	ErrWireBadMagic   = errors.New("wire: bad magic number")
	ErrWireBadVersion = errors.New("wire: unsupported version")
	ErrWireBadType    = errors.New("wire: unexpected frame type")
)
