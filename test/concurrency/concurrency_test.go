package concurrency
/*
	测试并发客户端下的联合读写稳定性
	从50个并发量 到 100个并发量 进行稳定性测试

*/
import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/server"
	"mydb/src/main/backend/sm"
	"mydb/src/main/backend/tbm"
	"mydb/src/main/backend/tm"
	clt "mydb/src/main/client/client"
	"mydb/src/main/transporter"
)

func createBooterFile(t *testing.T, base string) {
	t.Helper()
	f, err := os.OpenFile(base+".bt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatalf("create booter file error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close booter file error: %v", err)
	}
}

func runBackendSession(conn net.Conn, tableManager tbm.TableManager) {
	tr := transporter.NewWireTransporter(conn)
	pr := transporter.NewWireServerProtocoler()
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
	pro := transporter.NewWireProtocoler()
	trs := transporter.NewWireTransporter(clientConn)
	pkger := transporter.NewPackager(trs, pro)
	return clt.NewClient(pkger)
}

// TestBackendClientConcurrent50To100 验证50到100并发客户端下的联合读写稳定性。
func TestBackendClientConcurrent50To100(t *testing.T) {
	base := filepath.Join(t.TempDir(), "backend_client_concurrency")
	mem := int64(pcacher.PAGE_SIZE * 200)
	createBooterFile(t, base)

	tm0 := tm.Create(base)
	dm0 := dm.Create(base, mem, tm0)
	sm0 := sm.NewSerializabilityManager(tm0, dm0)
	tbm0 := tbm.Create(base, sm0, dm0)
	defer func() {
		dm0.Close()
		tm0.Close()
	}()

	setupClient := newPipeClient(tbm0)
	defer setupClient.Close()
	_, err := setupClient.Execute([]byte("create table user id uint64, name string, age uint32 (index id)"))
	if err != nil {
		t.Fatalf("create table error: %v", err)
	}

	for _, workers := range []int{50, 100} {
		t.Run(fmt.Sprintf("%d_workers", workers), func(t *testing.T) {
			errCh := make(chan error, workers)
			var wg sync.WaitGroup

			baseID := workers * 1000
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					id := baseID + idx + 1
					name := fmt.Sprintf("u%d", id)
					client := newPipeClient(tbm0)
					defer client.Close()

					insertSQL := fmt.Sprintf("insert into user values %d '%s' %d", id, name, id%100)
					resp, err := client.Execute([]byte(insertSQL))
					if err != nil || string(resp) != "Insert" {
						errCh <- fmt.Errorf("insert id=%d failed, resp=%s err=%v", id, resp, err)
						return
					}

					readSQL := fmt.Sprintf("read * from user where id = %d", id)
					resp, err = client.Execute([]byte(readSQL))
					if err != nil {
						errCh <- fmt.Errorf("read id=%d failed, err=%v", id, err)
						return
					}
					if !bytes.Contains(resp, []byte(name)) {
						errCh <- fmt.Errorf("read id=%d mismatch, resp=%s", id, resp)
					}
				}(i)
			}

			wg.Wait()
			close(errCh)
			for err := range errCh {
				if err != nil {
					t.Fatal(err)
				}
			}

			verifyClient := newPipeClient(tbm0)
			defer verifyClient.Close()
			for i := 0; i < workers; i++ {
				id := baseID + i + 1
				name := fmt.Sprintf("u%d", id)
				readSQL := fmt.Sprintf("read * from user where id = %d", id)
				resp, err := verifyClient.Execute([]byte(readSQL))
				if err != nil {
					t.Fatalf("verify read id=%d failed, err=%v", id, err)
				}
				if !bytes.Contains(resp, []byte(name)) {
					t.Fatalf("verify read id=%d mismatch, resp=%s", id, resp)
				}
			}
		})
	}
}
