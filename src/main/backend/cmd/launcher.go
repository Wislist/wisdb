package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

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
	defaultMem  = (1 << 20) * 64
	defaultNet  = "tcp"
	defaultAddr = ":3307"
	version     = "0.2.0"
)

const (
	_KB = 1 << 10
	_MB = 1 << 20
	_GB = 1 << 30
)

func main() {
	cobra.EnableCommandSorting = false
	setHelpTemplates(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "wisdb-server",
	Short:   utils.CyanText("WisDB") + " \u2014 a lightweight KV-based relational database",
	Version: version,
	Long:    "WisDB is a lightweight KV-based relational database prototype written in Go.\nIt features MVCC transactions, WAL recovery, B+Tree indexing, and a TCP\nclient interface with SQL support.\n\nAll database files are stored inside a single directory.",
	CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	SilenceUsage:      true,
}

var serveCmd = &cobra.Command{
	Use:     "serve --db-path <path> [flags]",
	Short:   "Start the database server",
	Long:    "Start the WisDB server, listening for client connections.\n\nThe database directory must already exist and contain a valid WisDB\ndatabase (use 'create' to initialize one).",
	Example: "  wisdb-server serve --db-path ./mydb\n  wisdb-server serve --db-path ./mydb --mem 128MB --addr :4000",
	Args:    cobra.NoArgs,
	RunE:    runServe,
}

var createCmd = &cobra.Command{
	Use:     "create --db-path <path>",
	Short:   "Initialize a new database",
	Long:    "Create a new WisDB database directory.\n\nThis creates the data, log, transaction, and metadata files inside\nthe specified directory. The directory will be created if it does\nnot already exist.",
	Example: "  wisdb-server create --db-path ./mydb",
	Args:    cobra.NoArgs,
	RunE:    runCreate,
}

var (
	serveDBPath  string
	serveMem     string
	serveNet     string
	serveAddr    string
	createDBPath string
)

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(createCmd)
	setHelpTemplates(rootCmd)
	setHelpTemplates(serveCmd)
	setHelpTemplates(createCmd)

	serveCmd.Flags().StringVarP(&serveDBPath, "db-path", "d", "", "Path to database directory (required)")
	serveCmd.Flags().StringVarP(&serveMem, "mem", "m", "64MB", "Memory budget for page cache")
	serveCmd.Flags().StringVar(&serveNet, "net", defaultNet, "Network type (tcp, unix)")
	serveCmd.Flags().StringVarP(&serveAddr, "addr", "a", defaultAddr, "Listen address")
	serveCmd.MarkFlagRequired("db-path")

	createCmd.Flags().StringVarP(&createDBPath, "db-path", "d", "", "Path for new database directory (required)")
	createCmd.MarkFlagRequired("db-path")
}

func runServe(cmd *cobra.Command, args []string) error {
	mem, err := parseMem(serveMem)
	if err != nil {
		return fmt.Errorf("invalid --mem value %q: %w", serveMem, err)
	}
	fmt.Println(utils.BoldText("\n  WisDB") + " " + utils.DimText("v"+version))
	fmt.Println(utils.DimText("  listening on " + serveNet + "://" + serveAddr + "\n"))
	openDB(serveDBPath, mem, serveNet, serveAddr)
	return nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	if dbExists(createDBPath) {
		return fmt.Errorf("database already exists at %q \u2014 use 'serve' to open it", createDBPath)
	}
	createDB(createDBPath)
	fmt.Printf("\n  "+utils.GreenText("ok")+"  database created at %s\n\n", createDBPath)
	return nil
}

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
	for _, p := range []string{
		prefix + pcacher.SUFFIX_DB,
		prefix + logger.SUFFIX_LOG,
		prefix + tm.XID_SUFFIX,
		prefix + ".bt",
	} {
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
	if len(memStr) < 2 {
		return 0, fmt.Errorf("memory size must include a unit suffix (KB, MB, GB)")
	}
	unit := memStr[len(memStr)-2:]
	num, err := utils.StrToUint64(memStr[:len(memStr)-2])
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
		return 0, fmt.Errorf("unknown unit %q \u2014 use KB, MB, or GB", unit)
	}
}
