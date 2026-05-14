package transporter

import (
	"net"
	"testing"
)

// TestWireProtocol 验证 wire protocol 的编解码和端到端收发
func TestWireProtocol(t *testing.T) {
	t.Run("encode_decode_request", func(t *testing.T) {
		p := NewWireProtocoler()
		sql := []byte("insert into user values 1 'alice' 20")
		frame := p.Encode(NewPackage(sql, nil))

		// 验证 Magic
		if frame[0] != 0x57 || frame[1] != 0x49 || frame[2] != 0x53 || frame[3] != 0x44 {
			t.Fatal("bad magic")
		}
		if frame[4] != WireVersion {
			t.Fatal("bad version")
		}
		if frame[5] != WireTypeRequest {
			t.Fatal("expected request type")
		}

		pkg, err := p.Decode(frame)
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if string(pkg.Data()) != string(sql) {
			t.Fatalf("payload mismatch: got %q want %q", pkg.Data(), sql)
		}
	})

	t.Run("encode_decode_response_ok", func(t *testing.T) {
		sp := NewWireServerProtocoler()
		// 先 Decode 一个 Request，让服务端记录 RequestID
		cp := NewWireProtocoler()
		reqFrame := cp.Encode(NewPackage([]byte("select 1"), nil))
		if _, err := sp.Decode(reqFrame); err != nil {
			t.Fatalf("server decode request: %v", err)
		}

		// 服务端编码 Response
		result := []byte("ok result")
		respFrame := sp.Encode(NewPackage(result, nil))
		if respFrame[5] != WireTypeResponse {
			t.Fatal("expected response type")
		}
		if respFrame[10] != WireFlagOK {
			t.Fatal("expected OK flag")
		}

		// 客户端解码 Response
		pkg, err := cp.Decode(respFrame)
		if err != nil {
			t.Fatalf("client decode response: %v", err)
		}
		if string(pkg.Data()) != string(result) {
			t.Fatalf("result mismatch: got %q want %q", pkg.Data(), result)
		}
	})

	t.Run("encode_decode_response_error", func(t *testing.T) {
		sp := NewWireServerProtocoler()
		cp := NewWireProtocoler()
		reqFrame := cp.Encode(NewPackage([]byte("bad sql"), nil))
		sp.Decode(reqFrame)

		errMsg := "syntax error near 'bad'"
		respFrame := sp.Encode(NewPackage(nil, &testErr{errMsg}))
		if respFrame[10] != WireFlagError {
			t.Fatal("expected error flag")
		}

		pkg, err := cp.Decode(respFrame)
		if err != nil {
			t.Fatalf("decode error frame: %v", err)
		}
		if pkg.Err() == nil || pkg.Err().Error() != errMsg {
			t.Fatalf("error mismatch: got %v want %q", pkg.Err(), errMsg)
		}
	})

	t.Run("end_to_end_over_pipe", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()

		serverTr := NewWireTransporter(serverConn)
		serverPr := NewWireServerProtocoler()
		serverPkg := NewPackager(serverTr, serverPr)

		clientTr := NewWireTransporter(clientConn)
		clientPr := NewWireProtocoler()
		clientPkg := NewPackager(clientTr, clientPr)

		sql := []byte("read * from user where id = 42")
		result := []byte("id=42 name=alice age=20")

		done := make(chan error, 1)
		go func() {
			pkg, err := serverPkg.Receive()
			if err != nil {
				done <- err
				return
			}
			if string(pkg.Data()) != string(sql) {
				done <- &testErr{"server received wrong sql: " + string(pkg.Data())}
				return
			}
			done <- serverPkg.Send(NewPackage(result, nil))
		}()

		if err := clientPkg.Send(NewPackage(sql, nil)); err != nil {
			t.Fatalf("client send: %v", err)
		}
		pkg, err := clientPkg.Receive()
		if err != nil {
			t.Fatalf("client receive: %v", err)
		}
		if string(pkg.Data()) != string(result) {
			t.Fatalf("result mismatch: got %q want %q", pkg.Data(), result)
		}
		if err := <-done; err != nil {
			t.Fatalf("server error: %v", err)
		}
	})

	t.Run("request_id_increments", func(t *testing.T) {
		cp := NewWireProtocoler()
		sp := NewWireServerProtocoler()

		for i := uint32(1); i <= 5; i++ {
			frame := cp.Encode(NewPackage([]byte("sql"), nil))
			pkg, err := sp.Decode(frame)
			if err != nil || pkg == nil {
				t.Fatalf("decode failed at i=%d: %v", i, err)
			}
			// 验证 RequestID 递增
			gotID := uint32(frame[6])<<24 | uint32(frame[7])<<16 | uint32(frame[8])<<8 | uint32(frame[9])
			if gotID != i {
				t.Fatalf("requestID mismatch: got %d want %d", gotID, i)
			}
		}
	})

	t.Run("bad_magic_rejected", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()

		tr := NewWireTransporter(serverConn)
		done := make(chan error, 1)
		go func() {
			_, err := tr.Receive()
			done <- err
		}()

		// 在 goroutine 里写，避免 net.Pipe 同步写阻塞
		go func() {
			clientConn.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x03, 'b', 'a', 'd'})
			clientConn.Close()
		}()

		err := <-done
		if err != ErrWireBadMagic {
			t.Fatalf("expected ErrWireBadMagic, got %v", err)
		}
	})
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
