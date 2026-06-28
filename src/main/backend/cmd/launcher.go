package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
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
	_DEFAULT_MEM = (1 << 20) * 64 // 64MB
)

const (
	_KB = 1 << 10
	_MB = 1 << 20
	_GB = 1 << 30
)

var (
	ErrInvalidMem = errors.New("invalid memory size — use format like 64MB, 128MB, 1GB")
	ErrDBExists   = errors.New("database already exists at this path — use -open instead of -create")
)

func openDB(path string, mem int64, net, addr string) {
	tm0, _ := tm.Open(path)
	dm0, err := dm.Open(path, mem, tm0)
	if err != nil {
		utils.Fatal("Failed to open DM:", err)
	}
	sm0 := sm.NewSerializabilityManager(tm0, dm0)
	tbm0 := tbm.Open(path, sm0, dm0)
	sv := server.NewServer(net, addr, tbm0)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sig
		utils.Info("Shutting down...")
		sv.Close()
	}()

	sv.Start()

	// Ensure proper shutdown order:
	// 1. Server.Close() was already called by the signal handler (or Start returned due to error)
	// 2. sv.Close() drains all active connections via WaitGroup, ensuring no transactions remain
	// 3. Only then is it safe to close DM and TM
	if err := dm0.Close(); err != nil {
		utils.Info("DM close error:", err)
	}
	if err := tm0.Close(); err != nil {
		utils.Info("TM close error:", err)
	}
	utils.Info("Database closed.")
}

func createDB(path string) {
	if dbExists(path) {
		panic(ErrDBExists)
	}
	tm, _ := tm.Create(path)
	dm, err := dm.Create(path, _DEFAULT_MEM, tm)
	if err != nil {
		panic(err)
	}
	sm := sm.NewSerializabilityManager(tm, dm)
	tbm.Create(path, sm, dm)
	if err := tm.Close(); err != nil {
		utils.Info("TM close error:", err)
	}
	if err := dm.Close(); err != nil {
		utils.Info("DM close error:", err)
	}
}

func dbExists(path string) bool {
	paths := []string{
		path + pcacher.SUFFIX_DB,
		path + logger.SUFFIX_LOG,
		path + tm.XID_SUFFIX,
		path + ".bt",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func main() {
	open := flag.String("open", "", "-open DBPath")
	create := flag.String("create", "", "-create DBPath")
	memStr := flag.String("mem", "64MB", "-mem 64MB")
	net := flag.String("net", "tcp", "-net tcp")
	addr := flag.String("addr", ":3307", "-addr :3307")
	flag.Parse()

	if *open != "" {
		openDB(*open, parseMem(*memStr), *net, *addr)
		return
	}
	if *create != "" {
		createDB(*create)
		return
	}
	fmt.Println("Usage: launcher -open DBPath [-mem 64MB] [-net tcp] [-addr :3307]")
	fmt.Println("       launcher -create DBPath")
}

func parseMem(memStr string) int64 {
	if memStr == "" {
		return _DEFAULT_MEM
	}
	length := len(memStr)
	if length < 2 {
		panic(ErrInvalidMem)
	}

	memUint := memStr[length-2:]
	memNum, err := utils.StrToUint64(memStr[:length-2])
	if err != nil {
		panic(err)
	}
	switch memUint {
	case "KB":
		return int64(memNum) * _KB
	case "MB":
		return int64(memNum) * _MB
	case "GB":
		return int64(memNum) * _GB
	default:
		panic(ErrInvalidMem)
	}
}
