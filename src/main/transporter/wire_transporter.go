/*
	wire_transporter.go 实现基于 WisDB Wire Protocol 的 Transporter。

	与 hexTransporter 的区别：
	  - 直接传输二进制，无 Hex 编码开销（传输效率提升 50%）
	  - 帧头包含 Magic + Version，连接建立时可做握手校验
	  - 帧头包含 RequestID，为 pipeline 和异步 driver 预留
	  - 用 io.ReadFull 读取固定长度头部，避免依赖分隔符
*/
package transporter

import (
	"encoding/binary"
	"io"
	"net"
)

// wireFrame 是从网络读取后解析出的原始帧，供 wireProtocoler 使用
type wireFrame struct {
	requestID uint32
	frameType byte
	flag      byte   // 仅 Response 帧有效
	payload   []byte
}

type wireTransporter struct {
	conn net.Conn
}

func NewWireTransporter(conn net.Conn) Transporter {
	return &wireTransporter{conn: conn}
}

// Send 将 data 写入连接。
// data 由 wireProtocoler.Encode 生成，已经是完整的帧字节。
func (t *wireTransporter) Send(data []byte) error {
	_, err := t.conn.Write(data)
	return err
}

// Receive 从连接读取一个完整帧，返回原始帧字节（含帧头）。
// wireProtocoler.Decode 负责解析。
func (t *wireTransporter) Receive() ([]byte, error) {
	// 先读 4 字节 Magic + 1 字节 Version + 1 字节 Type = 6 字节前缀
	prefix := make([]byte, 6)
	if _, err := io.ReadFull(t.conn, prefix); err != nil {
		return nil, err
	}

	magic := binary.BigEndian.Uint32(prefix[0:4])
	if magic != WireMagic {
		return nil, ErrWireBadMagic
	}
	if prefix[4] != WireVersion {
		return nil, ErrWireBadVersion
	}
	frameType := prefix[5]

	switch frameType {
	case WireTypeRequest:
		// 剩余头部：RequestID(4) + BodyLen(4) = 8 字节
		rest := make([]byte, 8)
		if _, err := io.ReadFull(t.conn, rest); err != nil {
			return nil, err
		}
		bodyLen := binary.BigEndian.Uint32(rest[4:8])
		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(t.conn, body); err != nil {
			return nil, err
		}
		// 拼回完整帧字节，交给 Decode 统一解析
		frame := make([]byte, 6+8+bodyLen)
		copy(frame[0:6], prefix)
		copy(frame[6:14], rest)
		copy(frame[14:], body)
		return frame, nil

	case WireTypeResponse:
		// 剩余头部：RequestID(4) + Flag(1) + BodyLen(4) = 9 字节
		rest := make([]byte, 9)
		if _, err := io.ReadFull(t.conn, rest); err != nil {
			return nil, err
		}
		bodyLen := binary.BigEndian.Uint32(rest[5:9])
		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(t.conn, body); err != nil {
			return nil, err
		}
		frame := make([]byte, 6+9+bodyLen)
		copy(frame[0:6], prefix)
		copy(frame[6:15], rest)
		copy(frame[15:], body)
		return frame, nil

	default:
		return nil, ErrWireBadType
	}
}

func (t *wireTransporter) Close() error {
	return t.conn.Close()
}
