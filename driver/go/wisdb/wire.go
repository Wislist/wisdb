// wire.go — WisDB Wire Protocol v1 编解码（自包含，不依赖服务端代码）
package wisdb

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync/atomic"
)

const (
	wireMagic   = uint32(0x57495344) // "WISD"
	wireVersion = byte(0x01)

	wireTypeRequest  = byte(0x01)
	wireTypeResponse = byte(0x02)

	wireFlagOK    = byte(0x00)
	wireFlagError = byte(0x01)

	wireReqHeaderLen  = 14 // Magic(4)+Version(1)+Type(1)+RequestID(4)+BodyLen(4)
	wireRespHeaderLen = 15 // Magic(4)+Version(1)+Type(1)+RequestID(4)+Flag(1)+BodyLen(4)
)

var (
	ErrBadMagic   = errors.New("wisdb: bad magic number, not a WisDB server?")
	ErrBadVersion = errors.New("wisdb: unsupported protocol version")
	ErrBadFrame   = errors.New("wisdb: malformed response frame")
)

// wireConn 封装了一条 TCP 连接上的 Wire Protocol 收发
type wireConn struct {
	conn   net.Conn
	nextID uint32
}

func newWireConn(conn net.Conn) *wireConn {
	return &wireConn{conn: conn}
}

// send 将 SQL 字节编码为 Request 帧并发送
func (w *wireConn) send(sql []byte) error {
	reqID := atomic.AddUint32(&w.nextID, 1)
	bodyLen := uint32(len(sql))

	frame := make([]byte, wireReqHeaderLen+bodyLen)
	binary.BigEndian.PutUint32(frame[0:4], wireMagic)
	frame[4] = wireVersion
	frame[5] = wireTypeRequest
	binary.BigEndian.PutUint32(frame[6:10], reqID)
	binary.BigEndian.PutUint32(frame[10:14], bodyLen)
	copy(frame[14:], sql)

	_, err := w.conn.Write(frame)
	return err
}

// recv 从连接读取一个完整的 Response 帧，返回 (data, err)
func (w *wireConn) recv() ([]byte, error) {
	// 读前缀：Magic(4) + Version(1) + Type(1) = 6 字节
	prefix := make([]byte, 6)
	if _, err := io.ReadFull(w.conn, prefix); err != nil {
		return nil, err
	}

	magic := binary.BigEndian.Uint32(prefix[0:4])
	if magic != wireMagic {
		return nil, ErrBadMagic
	}
	if prefix[4] != wireVersion {
		return nil, ErrBadVersion
	}
	if prefix[5] != wireTypeResponse {
		return nil, ErrBadFrame
	}

	// 读剩余头部：RequestID(4) + Flag(1) + BodyLen(4) = 9 字节
	rest := make([]byte, 9)
	if _, err := io.ReadFull(w.conn, rest); err != nil {
		return nil, err
	}

	flag := rest[4]
	bodyLen := binary.BigEndian.Uint32(rest[5:9])

	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(w.conn, body); err != nil {
		return nil, err
	}

	if flag == wireFlagError {
		return nil, errors.New(string(body))
	}
	return body, nil
}

func (w *wireConn) close() error {
	return w.conn.Close()
}
