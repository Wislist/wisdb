/*
	mvcc_test.go 验证 MVCC 隔离级别的核心语义：
	  - Read Committed (level=0)：防脏读，允许不可重复读
	  - Repeatable Read / Serializable (level=1)：防不可重复读，防版本跳跃
*/
package sm

import (
	"bytes"
	"path/filepath"
	"sync"
	"testing"

	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)

func newSM(t *testing.T) (*serializabilityManager, func()) {
	t.Helper()
	base := filepath.Join(t.TempDir(), "mvcc")
	mem := int64(pcacher.PAGE_SIZE * 50)
	tm0, _ := tm.Create(base)
	dm0, _ := dm.Create(base, mem, tm0)
	sm0 := NewSerializabilityManager(tm0, dm0)
	return sm0, func() {
		dm0.Close()
		tm0.Close()
	}
}

// --- Read Committed ---

// T2 在 T1 未提交时不可见（防脏读）
func TestRC_NoDirtyRead(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	t1, _ := sm0.Begin(0)
	uid, err := sm0.Insert(t1, []byte("dirty"))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	t2, _ := sm0.Begin(0)
	_, ok, err := sm0.Read(t2, uid)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if ok {
		t.Fatal("RC: T2 should not see T1's uncommitted data (dirty read)")
	}
	sm0.Abort(t2)
	sm0.Abort(t1)
}

// T2 在 T1 提交后可见（读已提交）
func TestRC_ReadAfterCommit(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	t1, _ := sm0.Begin(0)
	uid, err := sm0.Insert(t1, []byte("committed"))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := sm0.Commit(t1); err != nil {
		t.Fatalf("commit t1: %v", err)
	}

	t2, _ := sm0.Begin(0)
	data, ok, err := sm0.Read(t2, uid)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !ok || !bytes.Equal(data, []byte("committed")) {
		t.Fatalf("RC: T2 should see T1's committed data, ok=%v data=%s", ok, data)
	}
	sm0.Abort(t2)
}

// RC 允许不可重复读：T2 两次读之间 T1 提交了新版本，第二次读到新值
func TestRC_AllowsNonRepeatableRead(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	// 先提交一条初始数据
	setup, _ := sm0.Begin(0)
	uid, err := sm0.Insert(setup, []byte("v1"))
	if err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	if err := sm0.Commit(setup); err != nil {
		t.Fatalf("commit setup: %v", err)
	}

	t2, _ := sm0.Begin(0)
	data1, ok1, _ := sm0.Read(t2, uid)
	if !ok1 {
		t.Fatal("first read should succeed")
	}

	// T1 删除旧版本并插入新版本后提交
	t1, _ := sm0.Begin(0)
	if _, err := sm0.Delete(t1, uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	uid2, err := sm0.Insert(t1, []byte("v2"))
	if err != nil {
		t.Fatalf("insert v2: %v", err)
	}
	if err := sm0.Commit(t1); err != nil {
		t.Fatalf("commit t1: %v", err)
	}

	// T2 第二次读旧 uid 应不可见（已被删除），新 uid 可见
	_, ok2, _ := sm0.Read(t2, uid)
	data3, ok3, _ := sm0.Read(t2, uid2)

	if ok2 {
		t.Fatal("RC: old version should be invisible after T1 committed delete")
	}
	if !ok3 || !bytes.Equal(data3, []byte("v2")) {
		t.Fatalf("RC: new version should be visible, ok=%v data=%s", ok3, data3)
	}
	_ = data1
	sm0.Abort(t2)
}

// --- Repeatable Read / Serializable ---

// RR 防不可重复读：T2 两次读同一 uid，中间 T1 提交删除，第二次仍可见
func TestRR_RepeatableRead(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	setup, _ := sm0.Begin(0)
	uid, err := sm0.Insert(setup, []byte("stable"))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := sm0.Commit(setup); err != nil {
		t.Fatalf("commit setup: %v", err)
	}

	t2, _ := sm0.Begin(1) // RR
	data1, ok1, _ := sm0.Read(t2, uid)
	if !ok1 {
		t.Fatal("first read should succeed")
	}

	// T1 删除后提交
	t1, _ := sm0.Begin(0)
	if _, err := sm0.Delete(t1, uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := sm0.Commit(t1); err != nil {
		t.Fatalf("commit t1: %v", err)
	}

	// T2 第二次读，RR 下仍应可见（快照隔离）
	data2, ok2, _ := sm0.Read(t2, uid)
	if !ok2 || !bytes.Equal(data2, data1) {
		t.Fatalf("RR: second read should return same data, ok=%v data=%s", ok2, data2)
	}
	sm0.Abort(t2)
}

// RR 下，T2 看不到 T1 在 T2 开始后提交的新数据（快照隔离）
func TestRR_SnapshotIsolation(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	t2, _ := sm0.Begin(1) // T2 先开始，建立快照

	t1, _ := sm0.Begin(0)
	uid, err := sm0.Insert(t1, []byte("new"))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := sm0.Commit(t1); err != nil {
		t.Fatalf("commit t1: %v", err)
	}

	// T2 不应看到 T1 提交的数据（T1 在 T2 快照之后）
	_, ok, _ := sm0.Read(t2, uid)
	if ok {
		t.Fatal("RR: T2 should not see data committed after its snapshot")
	}
	sm0.Abort(t2)
}

// 版本跳跃检测：直接验证 IsVersionSkip 在 RR 下对已被后续事务提交删除的版本返回 true
func TestRR_VersionSkipDetected(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	setup, _ := sm0.Begin(0)
	uid, err := sm0.Insert(setup, []byte("original"))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := sm0.Commit(setup); err != nil {
		t.Fatalf("commit setup: %v", err)
	}

	t2, _ := sm0.Begin(1) // T2 先开始，建立快照
	t2tx := sm0.tc[t2]

	// T1 删除并提交（在 T2 快照之后）
	t1, _ := sm0.Begin(0)
	if _, err := sm0.Delete(t1, uid); err != nil {
		t.Fatalf("T1 delete: %v", err)
	}
	if err := sm0.Commit(t1); err != nil {
		t.Fatalf("commit t1: %v", err)
	}

	// 直接加载 entry 验证 IsVersionSkip
	handle, err := sm0.ec.Get(uid)
	if err != nil {
		t.Fatalf("get entry: %v", err)
	}
	e := handle.(*entry)
	defer e.Release()

	if !IsVersionSkip(sm0.TM, t2tx, e) {
		t.Fatal("RR: IsVersionSkip should return true when a newer committed xmax exists outside snapshot")
	}
	sm0.Abort(t2)
}

// 自己插入的数据对自己可见
func TestSelfVisibility(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	for _, level := range []int{0, 1} {
		xid, _ := sm0.Begin(level)
		uid, err := sm0.Insert(xid, []byte("self"))
		if err != nil {
			t.Fatalf("level=%d insert: %v", level, err)
		}
		data, ok, err := sm0.Read(xid, uid)
		if err != nil {
			t.Fatalf("level=%d read: %v", level, err)
		}
		if !ok || !bytes.Equal(data, []byte("self")) {
			t.Fatalf("level=%d: own insert should be visible, ok=%v data=%s", level, ok, data)
		}
		sm0.Abort(xid)
	}
}

// 回滚后数据对其他事务不可见
func TestAbortedDataInvisible(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	t1, _ := sm0.Begin(0)
	uid, err := sm0.Insert(t1, []byte("ghost"))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	sm0.Abort(t1)

	t2, _ := sm0.Begin(0)
	_, ok, _ := sm0.Read(t2, uid)
	if ok {
		t.Fatal("aborted data should not be visible to other transactions")
	}
	sm0.Abort(t2)
}

// 并发插入：多个 RC 事务并发写入，提交后全部可读
func TestConcurrentInsertRead(t *testing.T) {
	sm0, cleanup := newSM(t)
	defer cleanup()

	const n = 20
	uids := make([]utils.UUID, n)
	payloads := make([][]byte, n)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, n)

	for i := 0; i < n; i++ {
		payloads[i] = []byte{byte(i)}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			xid, _ := sm0.Begin(0)
			uid, err := sm0.Insert(xid, payloads[idx])
			if err != nil {
				errCh <- err
				sm0.Abort(xid)
				return
			}
			if err := sm0.Commit(xid); err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			uids[idx] = uid
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	reader, _ := sm0.Begin(0)
	for i := 0; i < n; i++ {
		data, ok, err := sm0.Read(reader, uids[i])
		if err != nil || !ok || !bytes.Equal(data, payloads[i]) {
			t.Fatalf("idx=%d read mismatch ok=%v data=%v err=%v", i, ok, data, err)
		}
	}
	sm0.Abort(reader)
}
