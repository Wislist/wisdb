package im

import (
	"path/filepath"
	"sync"
	"testing"

	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)

func containsUUID(uuids []utils.UUID, target utils.UUID) bool {
	for _, u := range uuids {
		if u == target {
			return true
		}
	}
	return false
}

// 并发性测试：并发插入 + 并发查询，验证B+树在多协程下的数据正确性。
func TestBPlusTreeConcurrentInsertAndSearch(t *testing.T) {
	base := filepath.Join(t.TempDir(), "im_concurrent")
	mem := int64(pcacher.PAGE_SIZE * 80)

	tm0, _ := tm.Create(base)
	dm0, _ := dm.Create(base, mem, tm0)
	defer func() {
		dm0.Close()
		tm0.Close()
	}()

	bootUUID, err := Create(dm0)
	if err != nil {
		t.Fatalf("Create tree error: %v", err)
	}
	btI, err := Load(bootUUID, dm0)
	if err != nil {
		t.Fatalf("Load tree error: %v", err)
	}
	bt := btI.(*bPlusTree)
	defer bt.Close()

	const workers = 8
	const perWorker = 6

	insertErr := make(chan error, workers*perWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				key := utils.UUID(worker*100 + i + 1)
				uuid := utils.UUID(100000 + worker*100 + i + 1)
				if err := bt.Insert(key, uuid); err != nil {
					insertErr <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(insertErr)
	for err := range insertErr {
		t.Fatalf("concurrent insert error: %v", err)
	}

	searchErr := make(chan error, workers*perWorker)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				key := utils.UUID(worker*100 + i + 1)
				expected := utils.UUID(100000 + worker*100 + i + 1)
				got, err := bt.Search(key)
				if err != nil {
					searchErr <- err
					return
				}
				if !containsUUID(got, expected) {
					searchErr <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(searchErr)
	for err := range searchErr {
		t.Fatalf("concurrent search validation error: %v", err)
	}
}

// 稳定性测试：覆盖写入、查询、范围查询、关闭重开后的持久化一致性。
func TestBPlusTreeLifecycleStability(t *testing.T) {
	base := filepath.Join(t.TempDir(), "im_lifecycle")
	mem := int64(pcacher.PAGE_SIZE * 120)

	tm0, _ := tm.Create(base)
	dm0, _ := dm.Create(base, mem, tm0)

	bootUUID, err := Create(dm0)
	if err != nil {
		t.Fatalf("Create tree error: %v", err)
	}
	btI, err := Load(bootUUID, dm0)
	if err != nil {
		t.Fatalf("Load tree error: %v", err)
	}
	bt := btI.(*bPlusTree)

	const total = 300
	for i := 1; i <= total; i++ {
		key := utils.UUID(i)
		uuid := utils.UUID(i + 1000)
		if err := bt.Insert(key, uuid); err != nil {
			t.Fatalf("Insert key=%d error: %v", key, err)
		}
	}

	for i := 1; i <= total; i += 17 {
		key := utils.UUID(i)
		uuid := utils.UUID(i + 1000)
		got, err := bt.Search(key)
		if err != nil {
			t.Fatalf("Search key=%d error: %v", key, err)
		}
		if !containsUUID(got, uuid) {
			t.Fatalf("Search key=%d missing uuid=%d", key, uuid)
		}
	}

	rangeResult, err := bt.SearchRange(50, 100)
	if err != nil {
		t.Fatalf("SearchRange error: %v", err)
	}
	if len(rangeResult) < 51 {
		t.Fatalf("SearchRange result too small: got=%d want_at_least=51", len(rangeResult))
	}

	bt.Close()
	dm0.Close()
	tm0.Close()

	tm1, _ := tm.Open(base)
	dm1, _ := dm.Open(base, mem, tm1)
	defer func() {
		dm1.Close()
		tm1.Close()
	}()

	btI2, err := Load(bootUUID, dm1)
	if err != nil {
		t.Fatalf("Reload tree error: %v", err)
	}
	bt2 := btI2.(*bPlusTree)
	defer bt2.Close()

	for i := 1; i <= total; i += 29 {
		key := utils.UUID(i)
		uuid := utils.UUID(i + 1000)
		got, err := bt2.Search(key)
		if err != nil {
			t.Fatalf("Reload search key=%d error: %v", key, err)
		}
		if !containsUUID(got, uuid) {
			t.Fatalf("Reload search key=%d missing uuid=%d", key, uuid)
		}
	}
}
