package test

/*
	测试客户端与后端的集成测试
*/
import (
	"bytes"
	"net"
	"os"
	"path/filepath"
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

// TestBackendClientIntegration 验证客户端与后端通过协议层联合工作，覆盖建表、增删改查和事务控制主流程。
func TestBackendClientIntegration(t *testing.T) {
	base := filepath.Join(t.TempDir(), "backend_client_it")
	mem := int64(pcacher.PAGE_SIZE * 120)
	createBooterFile(t, base)

	tm0 := tm.Create(base)
	dm0 := dm.Create(base, mem, tm0)
	sm0 := sm.NewSerializabilityManager(tm0, dm0)
	tbm0 := tbm.Create(base, sm0, dm0)
	defer func() {
		dm0.Close()
		tm0.Close()
	}()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go runBackendSession(serverConn, tbm0)

	pro := transporter.NewWireProtocoler()
	trs := transporter.NewWireTransporter(clientConn)
	pkger := transporter.NewPackager(trs, pro)
	client := clt.NewClient(pkger)
	defer client.Close()

	resp, err := client.Execute([]byte("begin"))
	if err != nil || string(resp) != "begin" {
		t.Fatalf("begin failed, resp=%s err=%v", resp, err)
	}

	resp, err = client.Execute([]byte("create table user id uint64, name string, age uint32 (index id)"))
	if err != nil || string(resp) != "create user" {
		t.Fatalf("create table failed, resp=%s err=%v", resp, err)
	}

	resp, err = client.Execute([]byte("insert into user values 1 'alice' 20"))
	if err != nil || string(resp) != "Insert" {
		t.Fatalf("insert failed, resp=%s err=%v", resp, err)
	}

	resp, err = client.Execute([]byte("read * from user where id = 1"))
	if err != nil {
		t.Fatalf("read failed, err=%v", err)
	}
	if !bytes.Contains(resp, []byte("alice")) {
		t.Fatalf("read result mismatch: %s", resp)
	}

	resp, err = client.Execute([]byte("update user set name = 'bob' where id = 1"))
	if err != nil || string(resp) != "Update 1" {
		t.Fatalf("update failed, resp=%s err=%v", resp, err)
	}

	resp, err = client.Execute([]byte("read * from user where id = 1"))
	if err != nil {
		t.Fatalf("read after update failed, err=%v", err)
	}
	if !bytes.Contains(resp, []byte("bob")) {
		t.Fatalf("read after update mismatch: %s", resp)
	}

	resp, err = client.Execute([]byte("delete from user where id = 1"))
	if err != nil || string(resp) != "Delete 1" {
		t.Fatalf("delete failed, resp=%s err=%v", resp, err)
	}

	resp, err = client.Execute([]byte("commit"))
	if err != nil || string(resp) != "commit" {
		t.Fatalf("commit failed, resp=%s err=%v", resp, err)
	}

	resp, err = client.Execute([]byte("show"))
	if err != nil {
		t.Fatalf("show failed, err=%v", err)
	}
	if !bytes.Contains(resp, []byte("user")) {
		t.Fatalf("show result mismatch: %s", resp)
	}

	_, err = client.Execute([]byte("commit"))
	if err == nil {
		t.Fatalf("expected commit error when not in transaction")
	}
}
