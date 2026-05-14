// Package benchmark 测量 mydb-go 在不同并发量下的 TPS 和平均延迟。
// 运行方式：go test -v -timeout 300s ./test/benchmark/
// 结果输出到 test/benchmark/results.csv
package benchmark

import (
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/server"
	"mydb/src/main/backend/sm"
	"mydb/src/main/backend/tbm"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
	clt "mydb/src/main/client/client"
	"mydb/src/main/transporter"
)

// benchLevel 定义单个并发梯度的测试参数
type benchLevel struct {
	workers  int // 并发客户端数
	opsEach  int // 每个客户端执行的操作数
}

// benchLevels 定义并发梯度：1, 5, 10, 20, 50, 100, 150, 200
var benchLevels = []benchLevel{
	{1, 100},
	{5, 60},
	{10, 40},
	{20, 30},
	{50, 20},
	{100, 15},
	{150, 10},
	{200, 8},
}

// result 保存单个梯度的测量结果
type result struct {
	workers     int
	totalOps    int
	elapsedMs   int64
	tps         float64
	avgLatencyMs float64
	errorCount  int64
}

func createBooterFile(t *testing.T, base string) {
	t.Helper()
	f, err := os.OpenFile(base+".bt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatalf("create booter file: %v", err)
	}
	f.Close()
}

func runBackendSession(conn net.Conn, tableManager tbm.TableManager) {
	tr := transporter.NewHexTransporter(conn)
	pr := transporter.NewProtocoler()
	pkger := transporter.NewPackager(tr, pr)
	defer pkger.Close()

	exe := server.NewExecutor(tableManager)
	defer exe.Close()

	for {
		pkg, err := pkger.Receive()
		if err != nil {
			return
		}
		result, sqlErr := exe.Execute(pkg.Data())
		if err := pkger.Send(transporter.NewPackage(result, sqlErr)); err != nil {
			return
		}
	}
}

func newPipeClient(tableManager tbm.TableManager) clt.Client {
	serverConn, clientConn := net.Pipe()
	go runBackendSession(serverConn, tableManager)
	pro := transporter.NewProtocoler()
	trs := transporter.NewHexTransporter(clientConn)
	pkger := transporter.NewPackager(trs, pro)
	return clt.NewClient(pkger)
}

// runLevel 在指定并发量下执行压测，返回测量结果
func runLevel(t *testing.T, tbm0 tbm.TableManager, level benchLevel, idOffset int) result {
	t.Helper()

	var (
		wg         sync.WaitGroup
		errCount   int64
		totalLatNs int64 // 所有操作延迟之和（纳秒）
		opsDone    int64
	)

	start := time.Now()

	for i := 0; i < level.workers; i++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()
			client := newPipeClient(tbm0)
			defer client.Close()

			baseID := idOffset + workerIdx*level.opsEach

			for j := 0; j < level.opsEach; j++ {
				id := baseID + j + 1
				name := fmt.Sprintf("u%d", id)

				// INSERT
				opStart := time.Now()
				sql := fmt.Sprintf("insert into bench values %d '%s' %d", id, name, id%100)
				_, err := client.Execute([]byte(sql))
				latNs := time.Since(opStart).Nanoseconds()
				if err != nil {
					atomic.AddInt64(&errCount, 1)
				} else {
					atomic.AddInt64(&totalLatNs, latNs)
					atomic.AddInt64(&opsDone, 1)
				}

				// READ
				opStart = time.Now()
				sql = fmt.Sprintf("read * from bench where id = %d", id)
				_, err = client.Execute([]byte(sql))
				latNs = time.Since(opStart).Nanoseconds()
				if err != nil {
					atomic.AddInt64(&errCount, 1)
				} else {
					atomic.AddInt64(&totalLatNs, latNs)
					atomic.AddInt64(&opsDone, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	done := atomic.LoadInt64(&opsDone)
	errs := atomic.LoadInt64(&errCount)
	elapsedMs := elapsed.Milliseconds()

	tps := 0.0
	if elapsed.Seconds() > 0 {
		tps = float64(done) / elapsed.Seconds()
	}
	avgLatMs := 0.0
	if done > 0 {
		avgLatMs = float64(atomic.LoadInt64(&totalLatNs)) / float64(done) / 1e6
	}

	return result{
		workers:      level.workers,
		totalOps:     int(done),
		elapsedMs:    elapsedMs,
		tps:          tps,
		avgLatencyMs: avgLatMs,
		errorCount:   errs,
	}
}

// TestBenchmarkConcurrency 梯度并发压测主入口
func TestBenchmarkConcurrency(t *testing.T) {
	// 关闭 Info 日志，避免大量输出拖慢测试
	utils.LOG_LEVEL = utils.LOG_LEVEL_WARN
	defer func() { utils.LOG_LEVEL = utils.LOG_LEVEL_INFO }()
	base := filepath.Join(t.TempDir(), "bench")
	mem := int64(pcacher.PAGE_SIZE * 1000) // 4MB 页缓存
	createBooterFile(t, base)

	tm0 := tm.Create(base)
	dm0 := dm.Create(base, mem, tm0)
	sm0 := sm.NewSerializabilityManager(tm0, dm0)
	tbm0 := tbm.Create(base, sm0, dm0)
	defer func() {
		dm0.Close()
		tm0.Close()
	}()

	// 建表
	setup := newPipeClient(tbm0)
	_, err := setup.Execute([]byte("create table bench id uint64, name string, age uint32 (index id)"))
	setup.Close()
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// 输出 CSV 路径（写到源码目录，方便找到）
	csvPath := filepath.Join("results.csv")
	f, err := os.Create(csvPath)
	if err != nil {
		t.Fatalf("create csv: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{"workers", "total_ops", "elapsed_ms", "tps", "avg_latency_ms", "errors"})

	var results []result
	idOffset := 0

	for _, level := range benchLevels {
		t.Logf("running: workers=%d opsEach=%d ...", level.workers, level.opsEach)
		r := runLevel(t, tbm0, level, idOffset)
		idOffset += level.workers * level.opsEach * 2 // 避免 ID 冲突

		results = append(results, r)
		w.Write([]string{
			fmt.Sprintf("%d", r.workers),
			fmt.Sprintf("%d", r.totalOps),
			fmt.Sprintf("%d", r.elapsedMs),
			fmt.Sprintf("%.2f", r.tps),
			fmt.Sprintf("%.3f", r.avgLatencyMs),
			fmt.Sprintf("%d", r.errorCount),
		})
		w.Flush()

		t.Logf("  workers=%-3d  ops=%d  elapsed=%dms  TPS=%.1f  avgLat=%.3fms  errors=%d",
			r.workers, r.totalOps, r.elapsedMs, r.tps, r.avgLatencyMs, r.errorCount)
	}

	// 打印汇总表
	t.Log("\n=== Benchmark Summary ===")
	t.Log("Workers | TotalOps | Elapsed(ms) | TPS      | AvgLat(ms) | Errors")
	t.Log("--------|----------|-------------|----------|------------|-------")
	for _, r := range results {
		t.Logf("%-7d | %-8d | %-11d | %-8.1f | %-10.3f | %d",
			r.workers, r.totalOps, r.elapsedMs, r.tps, r.avgLatencyMs, r.errorCount)
	}
	t.Logf("\nCSV written to: %s", csvPath)
}
