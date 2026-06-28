package client

import (
	"mydb/src/main/transporter"
	"sync"
)

type RoundTripper interface {
	RoundTrip(pkg transporter.Package) (transporter.Package, error)
	Close() error
	Disconnected() <-chan struct{}
}

type recvResult struct {
	pkg transporter.Package
	err error
}

type roundTripper struct {
	p         transporter.Packager
	recvCh    chan recvResult
	disconnCh chan struct{}
	once      sync.Once
}

func NewRoundTripper(packager transporter.Packager) *roundTripper {
	rt := &roundTripper{
		p:         packager,
		recvCh:    make(chan recvResult, 1),
		disconnCh: make(chan struct{}),
	}
	go rt.receiveLoop()
	return rt
}

func (rt *roundTripper) receiveLoop() {
	for {
		pkg, err := rt.p.Receive()
		if err != nil {
			rt.recvCh <- recvResult{nil, err}
			rt.markDisconnected()
			return
		}
		rt.recvCh <- recvResult{pkg, nil}
	}
}

func (rt *roundTripper) markDisconnected() {
	rt.once.Do(func() { close(rt.disconnCh) })
}

func (rt *roundTripper) RoundTrip(pkg transporter.Package) (transporter.Package, error) {
	if err := rt.p.Send(pkg); err != nil {
		rt.markDisconnected()
		return nil, err
	}
	res := <-rt.recvCh
	return res.pkg, res.err
}

func (rt *roundTripper) Close() error {
	return rt.p.Close()
}

func (rt *roundTripper) Disconnected() <-chan struct{} {
	return rt.disconnCh
}
