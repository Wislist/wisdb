package sm

import (
	"bytes"
	"path/filepath"
	"testing"

	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)

func mustRead(t *testing.T, sm *serializabilityManager, xid tm.XID, uid utils.UUID) ([]byte, bool) {
	t.Helper()
	data, ok, err := sm.Read(xid, uid)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	return data, ok
}

func TestSerializabilityManagerLifecycle(t *testing.T) {
	base := filepath.Join(t.TempDir(), "sm_lifecycle")
	mem := int64(pcacher.PAGE_SIZE * 50)

	tm0 := tm.Create(base)
	dm0 := dm.Create(base, mem, tm0)
	sm0 := NewSerializabilityManager(tm0, dm0)

	xid1 := sm0.Begin(0)
	uidAlive, err := sm0.Insert(xid1, []byte("alive"))
	if err != nil {
		t.Fatalf("Insert alive error: %v", err)
	}
	if err := sm0.Commit(xid1); err != nil {
		t.Fatalf("Commit xid1 error: %v", err)
	}

	xid2 := sm0.Begin(0)
	uidDelete, err := sm0.Insert(xid2, []byte("to-delete"))
	if err != nil {
		t.Fatalf("Insert to-delete error: %v", err)
	}
	if err := sm0.Commit(xid2); err != nil {
		t.Fatalf("Commit xid2 error: %v", err)
	}

	xid3 := sm0.Begin(0)
	deleted, err := sm0.Delete(xid3, uidDelete)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if !deleted {
		t.Fatalf("Delete returned false")
	}
	if err := sm0.Commit(xid3); err != nil {
		t.Fatalf("Commit xid3 error: %v", err)
	}

	xid4 := sm0.Begin(0)
	aliveData, aliveOK := mustRead(t, sm0, xid4, uidAlive)
	if !aliveOK || !bytes.Equal(aliveData, []byte("alive")) {
		t.Fatalf("alive record mismatch, ok=%v data=%v", aliveOK, aliveData)
	}
	_, deletedOK := mustRead(t, sm0, xid4, uidDelete)
	if deletedOK {
		t.Fatalf("deleted record still visible")
	}
	sm0.Abort(xid4)

	xid5 := sm0.Begin(0)
	uidAbort, err := sm0.Insert(xid5, []byte("abort-me"))
	if err != nil {
		t.Fatalf("Insert abort-me error: %v", err)
	}
	sm0.Abort(xid5)

	xid6 := sm0.Begin(0)
	_, abortVisible := mustRead(t, sm0, xid6, uidAbort)
	if abortVisible {
		t.Fatalf("aborted record should not be visible")
	}
	sm0.Abort(xid6)

	dm0.Close()
	tm0.Close()

	tm1 := tm.Open(base)
	dm1 := dm.Open(base, mem, tm1)
	sm1 := NewSerializabilityManager(tm1, dm1)
	defer func() {
		dm1.Close()
		tm1.Close()
	}()

	xid7 := sm1.Begin(0)
	aliveData2, aliveOK2 := mustRead(t, sm1, xid7, uidAlive)
	if !aliveOK2 || !bytes.Equal(aliveData2, []byte("alive")) {
		t.Fatalf("reopen alive mismatch, ok=%v data=%v", aliveOK2, aliveData2)
	}
	_, deletedOK2 := mustRead(t, sm1, xid7, uidDelete)
	if deletedOK2 {
		t.Fatalf("reopen deleted record still visible")
	}
	if err := sm1.Commit(xid7); err != nil {
		t.Fatalf("Commit xid7 error: %v", err)
	}
}

