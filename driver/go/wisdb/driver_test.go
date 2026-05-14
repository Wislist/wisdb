package wisdb

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

// TestConnExecute 验证基本的 Execute 收发
func TestConnExecute(t *testing.T) {
	conn := startMockServer(t, func(sql string) ([]byte, error) {
		if sql == "read * from user where id = 1" {
			return []byte("id=1 name=alice age=20"), nil
		}
		return nil, fmt.Errorf("unknown sql: %s", sql)
	})
	defer conn.Close()

	result, err := conn.Execute("read * from user where id = 1")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if string(result) != "id=1 name=alice age=20" {
		t.Fatalf("result mismatch: %q", result)
	}
}

// TestConnExecuteError 验证服务端返回错误时 driver 正确传递
func TestConnExecuteError(t *testing.T) {
	conn := startMockServer(t, func(sql string) ([]byte, error) {
		return nil, errors.New("syntax error near 'bad'")
	})
	defer conn.Close()

	_, err := conn.Execute("bad sql")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestTransaction 验证事务 Begin/Execute/Commit 流程
func TestTransaction(t *testing.T) {
	var received []string
	conn := startMockServer(t, func(sql string) ([]byte, error) {
		received = append(received, sql)
		switch {
		case strings.HasPrefix(sql, "begin"):
			return []byte("begin"), nil
		case sql == "insert into user values 3 'carol' 30":
			return []byte("Insert"), nil
		case sql == "commit":
			return []byte("commit"), nil
		default:
			return nil, fmt.Errorf("unexpected sql: %s", sql)
		}
	})
	defer conn.Close()

	tx, err := conn.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	if _, err := tx.Execute("insert into user values 3 'carol' 30"); err != nil {
		t.Fatalf("tx.Execute: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// 验证 SQL 顺序
	want := []string{"begin isolation level read committed", "insert into user values 3 'carol' 30", "commit"}
	if len(received) != len(want) {
		t.Fatalf("sql count mismatch: got %v want %v", received, want)
	}
	for i, w := range want {
		if received[i] != w {
			t.Fatalf("sql[%d] mismatch: got %q want %q", i, received[i], w)
		}
	}
}

// TestTransactionRollback 验证 Rollback 发送 abort
func TestTransactionRollback(t *testing.T) {
	var received []string
	conn := startMockServer(t, func(sql string) ([]byte, error) {
		received = append(received, sql)
		if strings.HasPrefix(sql, "begin") {
			return []byte("begin"), nil
		}
		return []byte("ok"), nil
	})
	defer conn.Close()

	tx, err := conn.Begin(RepeatableRead)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if received[0] != "begin isolation level repeatable read" {
		t.Fatalf("wrong begin sql: %q", received[0])
	}
	if received[1] != "abort" {
		t.Fatalf("expected abort, got %q", received[1])
	}
}

// TestTransactionDoubleClose 验证事务关闭后不能再操作
func TestTransactionDoubleClose(t *testing.T) {
	conn := startMockServer(t, func(sql string) ([]byte, error) {
		if strings.HasPrefix(sql, "begin") {
			return []byte("begin"), nil
		}
		return []byte("ok"), nil
	})
	defer conn.Close()

	tx, _ := conn.Begin(ReadCommitted)
	tx.Commit()

	if err := tx.Commit(); err == nil {
		t.Fatal("expected error on double commit")
	}
	if err := tx.Rollback(); err == nil {
		t.Fatal("expected error on rollback after commit")
	}
}

// TestPoolGetPut 验证连接池基本的 Get/Put 流程
func TestPoolGetPut(t *testing.T) {
	// 用 net.Pipe 模拟服务端，接受连接但不处理（pool 测试不发 SQL）
	listener, err := newMockListener(t)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.close()

	pool, err := NewPool(listener.addr(), PoolOptions{
		MaxConns:    3,
		MinConns:    1,
		DialTimeout: 5e9,
	})
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	stats := pool.Stats()
	if stats.Active != 1 || stats.Idle != 1 {
		t.Fatalf("initial stats wrong: %+v", stats)
	}

	conn, err := pool.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	stats = pool.Stats()
	if stats.InUse != 1 || stats.Idle != 0 {
		t.Fatalf("after Get stats wrong: %+v", stats)
	}

	pool.Put(conn)
	stats = pool.Stats()
	if stats.Idle != 1 || stats.InUse != 0 {
		t.Fatalf("after Put stats wrong: %+v", stats)
	}
}

// TestPoolExhausted 验证超过 MaxConns 时返回错误
func TestPoolExhausted(t *testing.T) {
	listener, err := newMockListener(t)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.close()

	pool, err := NewPool(listener.addr(), PoolOptions{
		MaxConns: 2,
		MinConns: 0,
	})
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	c1, _ := pool.Get()
	c2, _ := pool.Get()
	_, err = pool.Get()
	if err == nil {
		t.Fatal("expected pool exhausted error")
	}
	pool.Put(c1)
	pool.Put(c2)
}

// TestPoolConcurrent 验证并发 Get/Put 不 panic
func TestPoolConcurrent(t *testing.T) {
	listener, err := newMockListener(t)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.close()

	pool, err := NewPool(listener.addr(), PoolOptions{MaxConns: 5, MinConns: 0})
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := pool.Get()
			if err != nil {
				return // pool exhausted, ok
			}
			pool.Put(conn)
		}()
	}
	wg.Wait()
}

// mockListener 是一个简单的 TCP listener，接受连接但不处理
type mockListener struct {
	ln net.Listener
}

func newMockListener(t *testing.T) (*mockListener, error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ml := &mockListener{ln: ln}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 1)
				c.Read(buf) // 阻塞直到连接关闭
				c.Close()
			}(conn)
		}
	}()
	return ml, nil
}

func (ml *mockListener) addr() string {
	return ml.ln.Addr().String()
}

func (ml *mockListener) close() {
	ml.ln.Close()
}
