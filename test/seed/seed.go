//脚本 循环插入300条数据
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"mydb/src/main/transporter"
	"mydb/src/main/client/client"
	"mydb/src/main/backend/netconfig"
)


func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run seed.go <sql-file>")
		os.Exit(1)
	}
	sqlFile := os.Args[1]

	f, err := os.Open(sqlFile)
	if err != nil {
		fmt.Printf("打开文件失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	conn, err := net.DialTimeout(netconfig.Net, netconfig.Address, 5*time.Second)
	if err != nil {
		fmt.Printf("连接数据库失败: %v\n确认服务端已启动（go run ./src/main/backend/cmd -open <dbpath>）\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	pro := transporter.NewWireProtocoler()
	trs := transporter.NewWireTransporter(conn)
	pkger := transporter.NewPackager(trs, pro)
	clt := client.NewClient(pkger)

	scanner := bufio.NewScanner(f)
	total := 0
	success := 0
	skipped := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "--") {
			skipped++
			continue
		}

		total++
		result, err := clt.Execute([]byte(line))
		if err != nil {
			fmt.Printf("[%d] ERR  %-60s => %v\n", total, line, err)
		} else {
			success++
			resp := strings.TrimSpace(string(result))
			if resp != "" {
				fmt.Printf("[%d] OK   %-60s => %s\n", total, line, resp)
			} else {
				fmt.Printf("[%d] OK   %s\n", total, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("读取文件出错: %v\n", err)
	}

	fmt.Printf("\n完成：共 %d 条语句，成功 %d，失败 %d，跳过空行/注释 %d\n",
		total, success, total-success, skipped)
}
