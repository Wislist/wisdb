/*
	wire_protocoler.go 实现基于 WisDB Wire Protocol 的 Protocoler。

	Encode：将 Package 编码为完整的帧字节（含帧头）。
	Decode：将帧字节解析为 Package。

	RequestID 管理：
	  - 客户端侧：每次 Encode(Request) 时自增，通过 wireProtocoler.nextID 维护。
	  - 服务端侧：Decode 读取请求的 RequestID，Encode(Response) 时原样回写。
	  - 当前为同步 RoundTrip 模式，RequestID 主要为未来 pipeline 预留。
*/
package transporter

import (
	"encoding/binary"
	"errors"
	"sync/atomic"
)

type wireProtocoler struct {
	// nextID 用于客户端生成递增的 RequestID（原子操作，并发安全）
	nextID uint32
	// isServer 标记当前是服务端还是客户端，影响 Encode 生成的帧类型
	isServer bool
	// lastRequestID 服务端记录最近一次收到的 RequestID，用于构造 Response
	lastRequestID uint32
}

// NewWireProtocoler 创建客户端侧 Protocoler
func NewWireProtocoler() Protocoler {
	return &wireProtocoler{isServer: false}
}

// NewWireServerProtocoler 创建服务端侧 Protocoler
func NewWireServerProtocoler() Protocoler {
	return &wireProtocoler{isServer: true}
}

func (p *wireProtocoler) Encode(pkg Package) []byte {
	if p.isServer {
		return p.encodeResponse(pkg)
	}
	return p.encodeRequest(pkg)
}

func (p *wireProtocoler) encodeRequest(pkg Package) []byte {
	reqID := atomic.AddUint32(&p.nextID, 1)
	payload := pkg.Data()
	bodyLen := uint32(len(payload))

	// 帧总长：Magic(4)+Version(1)+Type(1)+RequestID(4)+BodyLen(4)+Payload
	frame := make([]byte, wireReqHeaderLen+bodyLen)
	binary.BigEndian.PutUint32(frame[0:4], WireMagic)
	frame[4] = WireVersion
	frame[5] = WireTypeRequest
	binary.BigEndian.PutUint32(frame[6:10], reqID)
	binary.BigEndian.PutUint32(frame[10:14], bodyLen)
	copy(frame[14:], payload)
	return frame
}

func (p *wireProtocoler) encodeResponse(pkg Package) []byte {
	reqID := atomic.LoadUint32(&p.lastRequestID)

	var flag byte
	var payload []byte
	if pkg.Err() != nil {
		flag = WireFlagError
		payload = []byte(pkg.Err().Error())
	} else {
		flag = WireFlagOK
		payload = pkg.Data()
	}
	bodyLen := uint32(len(payload))

	// 帧总长：Magic(4)+Version(1)+Type(1)+RequestID(4)+Flag(1)+BodyLen(4)+Payload
	frame := make([]byte, wireRespHeaderLen+bodyLen)
	binary.BigEndian.PutUint32(frame[0:4], WireMagic)
	frame[4] = WireVersion
	frame[5] = WireTypeResponse
	binary.BigEndian.PutUint32(frame[6:10], reqID)
	frame[10] = flag
	binary.BigEndian.PutUint32(frame[11:15], bodyLen)
	copy(frame[15:], payload)
	return frame
}

func (p *wireProtocoler) Decode(data []byte) (Package, error) {
	if len(data) < 6 {
		return nil, ErrInvalidPkgData
	}
	frameType := data[5]

	switch frameType {
	case WireTypeRequest:
		if len(data) < wireReqHeaderLen {
			return nil, ErrInvalidPkgData
		}
		reqID := binary.BigEndian.Uint32(data[6:10])
		bodyLen := binary.BigEndian.Uint32(data[10:14])
		if uint32(len(data)) < uint32(wireReqHeaderLen)+bodyLen {
			return nil, ErrInvalidPkgData
		}
		// 服务端记录 RequestID，用于构造 Response
		atomic.StoreUint32(&p.lastRequestID, reqID)
		payload := data[wireReqHeaderLen : wireReqHeaderLen+bodyLen]
		return NewPackage(payload, nil), nil

	case WireTypeResponse:
		if len(data) < wireRespHeaderLen {
			return nil, ErrInvalidPkgData
		}
		flag := data[10]
		bodyLen := binary.BigEndian.Uint32(data[11:15])
		if uint32(len(data)) < uint32(wireRespHeaderLen)+bodyLen {
			return nil, ErrInvalidPkgData
		}
		payload := data[wireRespHeaderLen : wireRespHeaderLen+bodyLen]
		if flag == WireFlagError {
			return NewPackage(nil, errors.New(string(payload))), nil
		}
		return NewPackage(payload, nil), nil

	default:
		return nil, ErrWireBadType
	}
}
