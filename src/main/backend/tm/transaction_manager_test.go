package tm_test

import (
	"math/rand"
	"mydb/src/main/backend/tm"
	"os"
	"sync"
	"testing"
	"time"
)

const (
	TEST_FILE      = "concurrent_test"
	NUM_WORKERS    = 50  // 并发工作协程数
	OPS_PER_WORKER = 100 // 每个协程的操作次数
)

func setupConcurrentTest() tm.TransactionManager {
	os.Remove(TEST_FILE)
	return tm.Create(TEST_FILE)
}

func cleanupConcurrentTest(tm tm.TransactionManager) {
	tm.Close()
	os.Remove(TEST_FILE)
}

func TestConcurrentTransactions(t *testing.T) {
	a := setupConcurrentTest()
	defer cleanupConcurrentTest(a)

	var wg sync.WaitGroup
	statusMap := sync.Map{} // 线程安全的 map 记录事务状态

	worker := func() {
		defer wg.Done()
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for i := 0; i < OPS_PER_WORKER; i++ {
			op := r.Intn(3)
			switch op {
			case 0: // Begin + Commit
				xid := a.Begin()
				statusMap.Store(xid, tm.FIELD_TRAN_ACTIVE)
				time.Sleep(time.Duration(r.Intn(10)) * time.Millisecond) // 模拟处理延迟
				a.Commit(xid)
				statusMap.Store(xid, tm.FIELD_TRAN_COMMITTED)

			case 1: // Begin + Abort
				xid := a.Begin()
				statusMap.Store(xid, tm.FIELD_TRAN_ACTIVE)
				time.Sleep(time.Duration(r.Intn(10)) * time.Millisecond)
				a.Abort(xid)
				statusMap.Store(xid, tm.FIELD_TRAN_ABORTED)

			case 2: // 随机验证已有事务状态
				if xid, ok := randomExistingXid(&statusMap, r); ok {
					checkTransactionState(t, a, xid, &statusMap)
				}
			}
		}
	}

	wg.Add(NUM_WORKERS)
	for i := 0; i < NUM_WORKERS; i++ {
		go worker()
	}
	wg.Wait()

	// 最终一致性验证
	statusMap.Range(func(key, value interface{}) bool {
		xid := key.(int64)
		_ = value.(byte)
		checkTransactionState(t, a, xid, &statusMap)
		return true
	})
}

// 辅助函数：随机获取一个已存在的事务ID
func randomExistingXid(m *sync.Map, r *rand.Rand) (int64, bool) {
	var xid int64
	found := false
	m.Range(func(key, _ interface{}) bool {
		if r.Intn(2) == 0 { // 50% 概率选择当前key
			xid = key.(int64)
			found = true
			return false
		}
		return true
	})
	return xid, found
}

// 辅助函数：验证事务状态是否匹配
func checkTransactionState(t *testing.T, tmd tm.TransactionManager, xid int64, m *sync.Map) {
	expectedStatus, _ := m.Load(xid)
	var actualStatus byte
	switch {
	case tmd.IsActive(xid):
		actualStatus = tm.FIELD_TRAN_ACTIVE
	case tmd.IsCommitted(xid):
		actualStatus = tm.FIELD_TRAN_COMMITTED
	case tmd.IsAborted(xid):
		actualStatus = tm.FIELD_TRAN_ABORTED
	default:
		t.Fatalf("xid %d has no valid state", xid)
	}

	if actualStatus != expectedStatus {
		t.Errorf("xid %d state mismatch: expected %d, got %d",
			xid, expectedStatus, actualStatus)
	}
}
