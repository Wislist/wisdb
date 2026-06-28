package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/dm/logger"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/server"
	"mydb/src/main/backend/sm"
	"mydb/src/main/backend/tbm"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)

const (
	defaultMem  = (1 << 20) * 64 // 64MB
	defaultNet  = "tcp"
	defaultAddr = ":3307"
	version     = "0.2.0"
)

const (
	_KB = 1 << 10
	_MB = 1 << 20
	_GB = 1 << 30
)

var (
	ErrInvalidMem = errors.New("invalid memory size — use format like 64MB, 128MB, 1GB")
)

func main() {
	if len(os.Args) < 2 {
		printServerUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd()
	case "create":
		createCmd()
	case "--version", "-v":
		fmt.Println("wisdb-server", version)
	case "--help", "-h", "help":
		printServerUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printServerUsage()
		os.Exit(1)
	}
}

func printServerUsage() {
	fmt.Print(`WisDB — a lightweight KV-based relational database.

Usage:
  wisdb-server serve  --db-path <path> [flags]
  wisdb-server create --db-path <path>
  wisdb-server --version

Commands:
  serve     Start the database server.
  create    Initialize a new database at the given path.

Serve flags:
  --db-path  PATH   Path to the database directory (required).
  --mem      SIZE   Memory budget for page cache (default: 64MB).
  --addr     ADDR   Listen address (default: ":3307").
  --net      NET    Network type: tcp, unix (default: "tcp").

All database files are stored inside <db-path>/ — the path is a directory.

Examples:
  wisdb-server create --db-path ./mydb
  wisdb-server serve  --db-path ./mydb
  wisdb-server serve  --db-path ./mydb --mem 128MB --addr :4000
`)
}

func serveCmd() {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := flags.String("db-path", "", "Path to database directory")
	memStr := flags.String("mem", "64MB", "Memory budget (64MB, 128MB, 1GB)")
	network := flags.String("net", defaultNet, "Network type (tcp, unix)")
	addr := flags.String("addr", defaultAddr, "Listen address")

	flags.Usage = func() {
		fmt.Print("Usage: wisdb-server serve --db-path <path> [flags]\n\nFlags:\n")
		flags.PrintDefaults()
	}
	flags.Parse(os.Args[2:])

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "error: --db-path is required")
		flags.Usage()
		os.Exit(1)
	}

	mem, err := parseMem(*memStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid --mem value %q: %v\n", *memStr, err)
		os.Exit(1)
	}

	openDB(*dbPath, mem, *network, *addr)
}

func createCmd() {
	flags := flag.NewFlagSet("create", flag.ExitOnError)
	dbPath := flags.String("db-path", "", "Path for new database directory")

	flags.Usage = func() {
		fmt.Print("Usage: wisdb-server create --db-path <path>\n\nFlags:\n")
		flags.PrintDefaults()
	}
	flags.Parse(os.Args[2:])

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "error: --db-path is required")
		flags.Usage()
		os.Exit(1)
	}

	if dbExists(*dbPath) {
		fmt.Fprintf(os.Stderr, "error: database already exists at %q — use 'serve' to open it\n", *dbPath)
		os.Exit(1)
	}

	createDB(*dbPath)
}

// dbPrefix returns the path prefix used by all storage modules inside the
// database directory. Files live under <path>/wisdb.* so only files owned
// by WisDB appear inside the user-specified directory.
func dbPrefix(path string) string {
	return filepath.Join(path, "wisdb")
}

func openDB(path string, mem int64, network, addr string) {
	prefix := dbPrefix(path)

	tm0, _ := tm.Open(prefix)
	dm0, err := dm.Open(prefix, mem, tm0)
	if err != nil {
		utils.Fatal("Failed to open DM:", err)
	}
	sm0 := sm.NewSerializabilityManager(tm0, dm0)
	tbm0 := tbm.Open(prefix, sm0, dm0)
	sv := server.NewServer(network, addr, tbm0)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sig
		utils.Info("Shutting down...")
		sv.Close()
	}()

	sv.Start()

	if err := dm0.Close(); err != nil {
		utils.Info("DM close error:", err)
	}
	if err := tm0.Close(); err != nil {
		utils.Info("TM close error:", err)
	}
	utils.Info("Database closed.")
}

func createDB(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		utils.Fatal("Failed to create database directory:", err)
	}

	prefix := dbPrefix(path)

	tm, _ := tm.Create(prefix)
	dm, err := dm.Create(prefix, defaultMem, tm)
	if err != nil {
		utils.Fatal("Failed to create DM:", err)
	}
	sm := sm.NewSerializabilityManager(tm, dm)
	tbm.Create(prefix, sm, dm)
	if err := tm.Close(); err != nil {
		utils.Info("TM close error:", err)
	}
	if err := dm.Close(); err != nil {
		utils.Info("DM close error:", err)
	}
}

func dbExists(path string) bool {
	prefix := dbPrefix(path)
	paths := []string{
		prefix + pcacher.SUFFIX_DB,
		prefix + logger.SUFFIX_LOG,
		prefix + tm.XID_SUFFIX,
		prefix + ".bt",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func parseMem(memStr string) (int64, error) {
	if memStr == "" {
		return defaultMem, nil
	}
	length := len(memStr)
	if length < 2 {
		return 0, fmt.Errorf("memory size must include a unit suffix (KB, MB, GB)")
	}

	unit := memStr[length-2:]
	num, err := utils.StrToUint64(memStr[:length-2])
	if err != nil {
		return 0, fmt.Errorf("invalid number: %w", err)
	}
	switch unit {
	case "KB":
		return int64(num) * _KB, nil
	case "MB":
		return int64(num) * _MB, nil
	case "GB":
		return int64(num) * _GB, nil
	default:
		return 0, fmt.Errorf("unknown unit %q — use KB, MB, or GB", unit)
	}
}
