package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"mydb/src/main/client/client"
	"mydb/src/main/transporter"
)

const (
	defaultNet  = "tcp"
	defaultAddr = ":3307"
	version     = "0.2.0"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println("wisdb-client", version)
			return
		case "--help", "-h", "help":
			printClientUsage()
			return
		}
	}

	connectCmd()
}

func printClientUsage() {
	fmt.Print(`WisDB Client — interactive SQL shell.

Usage:
  wisdb-client [flags]
  wisdb-client --version

Flags:
  --addr  ADDR   Server address (default: ":3307").
  --net   NET    Network type: tcp, unix (default: "tcp").

Examples:
  wisdb-client
  wisdb-client --addr :4000
  wisdb-client --addr 192.168.1.10:3307
`)
}

func connectCmd() {
	flags := flag.NewFlagSet("connect", flag.ExitOnError)
	network := flags.String("net", defaultNet, "Network type (tcp, unix)")
	addr := flags.String("addr", defaultAddr, "Server address")

	flags.Usage = func() {
		fmt.Print("Usage: wisdb-client [flags]\n\nFlags:\n")
		flags.PrintDefaults()
	}

	// Parse remaining args (skip program name + subcommand if present)
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "connect") {
		args = args[1:]
	}
	flags.Parse(args)

	fmt.Printf("Connecting to %s://%s ...\n", *network, *addr)
	pkger := dial(*network, *addr)
	fmt.Printf("Connected to %s\n", *addr)

	clt := client.NewClient(pkger)

	reconnect := func() (client.Client, <-chan struct{}) {
		clt.Close()
		pkger := dial(*network, *addr)
		fmt.Printf("Reconnected to %s\n", *addr)
		clt.Reconnect(pkger)
		return clt, clt.Disconnected()
	}

	shell := client.NewShellWithReconnect(clt, reconnect)
	shell.SetDisconnCh(clt.Disconnected())
	shell.Run()
}

func dial(network, addr string) transporter.Packager {
	for {
		conn, err := net.Dial(network, addr)
		if err == nil {
			pro := transporter.NewWireProtocoler()
			trs := transporter.NewWireTransporter(conn)
			return transporter.NewPackager(trs, pro)
		}
		fmt.Printf("\rWaiting for server at %s...", addr)
		time.Sleep(time.Second)
	}
}
