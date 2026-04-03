package dm

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"mydb/src/main/backend/dm/logger"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/tm"
)

// newTestDM 创建隔离的 DataManager 测试环境，并返回清理函数。
func newTestDM(t *testing.T) (*dataManager, func()) {
	t.Helper()
	base := filepath.Join(t.TempDir(), "dm_test")
	pc := pcacher.Create(base, pcacher.PAGE_SIZE*20)
	lg := logger.CreateMock(base)
	tmger := tm.CreateMock(base)
	dm := NewDataManager(pc, lg, tmger)
	cleanup := func() {
		pc.Close()
		lg.Close()
	}
	return dm, cleanup
}

// TestDataManagerInsertRead 验证插入后可读且数据一致。
func TestDataManagerInsertRead(t *testing.T) {
	dm, cleanup := newTestDM(t)
	defer cleanup()

	payload := []byte("hello-dm")
	uid, err := dm.Insert(1, payload)
	if err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	di, ok, err := dm.Read(uid)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if !ok {
		t.Fatalf("Read ok=false")
	}
	if !bytes.Equal(di.Data(), payload) {
		t.Fatalf("read data mismatch: got=%v want=%v", di.Data(), payload)
	}
	di.Release()
}

// TestDataManagerInsertTooLarge 验证超大数据会返回 ErrDataTooLarge。
func TestDataManagerInsertTooLarge(t *testing.T) {
	dm, cleanup := newTestDM(t)
	defer cleanup()

	tooLarge := make([]byte, PXMaxFreeSpace())
	_, err := dm.Insert(1, tooLarge)
	if err == nil {
		t.Fatalf("expected ErrDataTooLarge")
	}
	if err != ErrDataTooLarge {
		t.Fatalf("got err=%v want=%v", err, ErrDataTooLarge)
	}
}

// TestDataManagerReadInvalid 验证被标记无效的数据读取时返回 ok=false。
func TestDataManagerReadInvalid(t *testing.T) {
	dm, cleanup := newTestDM(t)
	defer cleanup()

	uid, err := dm.Insert(1, []byte("to-be-invalid"))
	if err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	pgno, offset := UUID2Address(uid)
	pg, err := dm.pc.GetPage(pgno)
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	pg.Dirty()
	InValidRawDataitem(pg.Data()[offset:])
	pg.Release()

	di, ok, err := dm.Read(uid)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if ok {
		if di != nil {
			di.Release()
		}
		t.Fatalf("expected ok=false for invalid item")
	}
}

// TestDataManagerConcurrentInsertRead 验证并发插入读取的基本稳定性与一致性。
func TestDataManagerConcurrentInsertRead(t *testing.T) {
	dm, cleanup := newTestDM(t)
	defer cleanup()

	const workers = 20
	const perWorker = 30
	var wg sync.WaitGroup
	errCh := make(chan error, workers*perWorker)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				payload := []byte(fmt.Sprintf("w%d-%d", worker, i))
				uid, err := dm.Insert(1, payload)
				if err != nil {
					errCh <- err
					return
				}
				di, ok, err := dm.Read(uid)
				if err != nil {
					errCh <- err
					return
				}
				if !ok {
					errCh <- fmt.Errorf("read ok=false, uid=%d", uid)
					return
				}
				if !bytes.Equal(di.Data(), payload) {
					di.Release()
					errCh <- fmt.Errorf("mismatch uid=%d", uid)
					return
				}
				di.Release()
			}
		}(w)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent test error: %v", err)
	}
}
