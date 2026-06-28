package main

import (
	"fmt"
	"net"
	"mydb/src/main/client/client"
	"mydb/src/main/transporter"
	"time"
)

const (
	_NET     = "tcp"
	_ADDRESS = ":3307"
)

func dial() transporter.Packager {
	conn, err := net.Dial(_NET, _ADDRESS)
	if err != nil {
		return nil
	}
	pro := transporter.NewWireProtocoler()
	trs := transporter.NewWireTransporter(conn)
	return transporter.NewPackager(trs, pro)
}

func waitForDial() transporter.Packager {
	for {
		pkger := dial()
		if pkger != nil {
			return pkger
		}
		fmt.Printf("\rWaiting for server at %s...", _ADDRESS)
		time.Sleep(time.Second)
	}
}

func main() {
	pkger := waitForDial()
	fmt.Printf("\r\033[KConnected to %s\n", _ADDRESS)

	clt := client.NewClient(pkger)

	reconnect := func() (client.Client, <-chan struct{}) {
		clt.Close()
		pkger := waitForDial()
		fmt.Printf("\r\033[KReconnected to %s\r\n", _ADDRESS)
		clt.Reconnect(pkger)
		return clt, clt.Disconnected()
	}

	shell := client.NewShellWithReconnect(clt, reconnect)
	shell.SetDisconnCh(clt.Disconnected())
	shell.Run()
}
