package wisdb

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
)

// startMockServer 启动一个 mock WisDB 服务端，返回已连接的客户端 *Conn。
// handler 接收 SQL 字符串，返回结果字节或错误。
func startMockServer(t *testing.T, handler func(sql string) ([]byte, error)) *Conn {
	t.Helper()
	serverConn, clientConn := net.Pipe()

	go func() {
		defer serverConn.Close()
		for {
			sql, err := readRequestFrame(serverConn)
			if err != nil {
				return
			}
			result, sqlErr := handler(string(sql))
			if err := writeResponseFrame(serverConn, result, sqlErr); err != nil {
				return
			}
		}
	}()

	return &Conn{
		wc:   newWireConn(clientConn),
		addr: "pipe",
	}
}

// readRequestFrame 从连接读取一个 Request 帧，返回 SQL payload
func readRequestFrame(conn net.Conn) ([]byte, error) {
	prefix := make([]byte, 6)
	if _, err := io.ReadFull(conn, prefix); err != nil {
		return nil, err
	}
	// RequestID(4) + BodyLen(4)
	rest := make([]byte, 8)
	if _, err := io.ReadFull(conn, rest); err != nil {
		return nil, err
	}
	bodyLen := binary.BigEndian.Uint32(rest[4:8])
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	return body, nil
}

// writeResponseFrame 向连接写入一个 Response 帧
func writeResponseFrame(conn net.Conn, data []byte, err error) error {
	var flag byte
	var payload []byte
	if err != nil {
		flag = wireFlagError
		payload = []byte(err.Error())
	} else {
		flag = wireFlagOK
		payload = data
	}
	bodyLen := uint32(len(payload))
	frame := make([]byte, wireRespHeaderLen+bodyLen)
	binary.BigEndian.PutUint32(frame[0:4], wireMagic)
	frame[4] = wireVersion
	frame[5] = wireTypeResponse
	// RequestID = 0（mock 不做 pipeline 校验）
	frame[10] = flag
	binary.BigEndian.PutUint32(frame[11:15], bodyLen)
	copy(frame[15:], payload)
	_, writeErr := conn.Write(frame)
	return writeErr
}
