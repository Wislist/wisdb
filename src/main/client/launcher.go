package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"

	"mydb/src/main/client/client"
	"mydb/src/main/transporter"
)

const (
	defaultNet  = "tcp"
	defaultAddr = ":3307"
	version     = "0.2.0"
)

func main() {
	cobra.EnableCommandSorting = false
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "wisdb-client [--addr <address>]",
	Short:   "WisDB Client — interactive SQL shell",
	Version: version,
	Long: `WisDB Client connects to a WisDB server and provides an interactive
SQL shell with readline-style input, command history, and auto-reconnect.

Type SQL statements directly at the prompt. Use 'begin' / 'commit' /
'abort' for transactions, and 'exit' or Ctrl+D to quit.`,
	Example: `  wisdb-client
  wisdb-client --addr :4000
  wisdb-client --addr 192.168.1.10:3307`,
	CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	SilenceUsage:      true,
	RunE:              runConnect,
}

var (
	clientNet  string
	clientAddr string
)

func init() {
	rootCmd.Flags().StringVarP(&clientAddr, "addr", "a", defaultAddr, "Server address")
	rootCmd.Flags().StringVar(&clientNet, "net", defaultNet, "Network type (tcp, unix)")
}

func runConnect(cmd *cobra.Command, args []string) error {
	pkger := dial(clientNet, clientAddr)

	clt := client.NewClient(pkger)

	reconnect := func() (client.Client, <-chan struct{}) {
		clt.Close()
		pkger := dial(clientNet, clientAddr)
		clt.Reconnect(pkger)
		return clt, clt.Disconnected()
	}

	shell := client.NewShellWithReconnect(clt, reconnect)
	shell.SetDisconnCh(clt.Disconnected())
	shell.Run()
	return nil
}

func dial(network, addr string) transporter.Packager {
	fmt.Printf("Connecting to %s://%s ...\r\n", network, addr)
	for {
		conn, err := net.Dial(network, addr)
		if err == nil {
			fmt.Printf("Connected to %s\r\n", addr)
			pro := transporter.NewWireProtocoler()
			trs := transporter.NewWireTransporter(conn)
			return transporter.NewPackager(trs, pro)
		}
		fmt.Printf("\rWaiting for server at %s...", addr)
		time.Sleep(time.Second)
	}
}
