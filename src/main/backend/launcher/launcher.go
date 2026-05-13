package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
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
	_NET         = "tcp"
	_ADDRESS     = ":8080"
	_DEFAULT_MEM = (1 << 20) * 64 // 64MB
)

const (
	_KB = 1 << 10
	_MB = 1 << 10
	_GB = 1 << 10
)

var (
	ErrInvalidMem = errors.New("Invalid Memory Size.")
	ErrDBExists   = errors.New("Database already exists, use -open.")
)

func openDB(path string, mem int64) {
	tm := tm.Open(path)
	dm := dm.Open(path, mem, tm)
	sm := sm.NewSerializabilityManager(tm, dm)
	tbm := tbm.Open(path, sm, dm)
	sv := server.NewServer(_NET, _ADDRESS, tbm)
	sv.Start()
}

func createDB(path string) {
	if dbExists(path) {
		panic(ErrDBExists)
	}
	tm := tm.Create(path)
	dm := dm.Create(path, _DEFAULT_MEM, tm)
	sm := sm.NewSerializabilityManager(tm, dm)
	tbm.Create(path, sm, dm)
	tm.Close()
	dm.Close()
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
	memStr := flag.String("mem", "", "-mem 64MB")
	flag.Parse()

	if *open != "" {
		openDB(*open, parseMem(*memStr))
		return
	}
	if *create != "" {
		createDB(*create)
		return
	}
	fmt.Println("Usage: launcher (open|create) DBPath")
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
