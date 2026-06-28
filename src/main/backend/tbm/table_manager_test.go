package tbm

import (
	"errors"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/parser/statement"
	"mydb/src/main/backend/sm"
	"mydb/src/main/backend/tm"
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

func buildCreateUser() *statement.Create {
	return &statement.Create{
		TableName: "user",
		FieldName: []string{"id", "name", "age"},
		FieldType: []string{"uint64", "string", "uint32"},
		Index:     []string{"id"},
	}
}

func TestTableManagerLifecycle(t *testing.T) {
	base := filepath.Join(t.TempDir(), "tbm_lifecycle")
	mem := int64(pcacher.PAGE_SIZE * 80)
	createBooterFile(t, base)

	tm0 := tm.Create(base)
	dm0, _ := dm.Create(base, mem, tm0)
	sm0 := sm.NewSerializabilityManager(tm0, dm0)
	tbm0 := Create(base, sm0, dm0)

	xid, beginMsg := tbm0.Begin(&statement.Begin{})
	if string(beginMsg) != "begin" {
		t.Fatalf("begin message=%s", beginMsg)
	}

	createMsg, err := tbm0.Create(xid, buildCreateUser())
	if err != nil {
		t.Fatalf("Create table error: %v", err)
	}
	if string(createMsg) != "create user" {
		t.Fatalf("create message=%s", createMsg)
	}

	showMsg := tbm0.Show(xid)
	if !bytes.Contains(showMsg, []byte("user")) {
		t.Fatalf("show does not contain table name, got=%s", showMsg)
	}

	_, err = tbm0.Create(xid, buildCreateUser())
	if !errors.Is(err, ErrDuplicatedTable) {
		t.Fatalf("duplicate create err=%v want=%v", err, ErrDuplicatedTable)
	}

	_, err = tbm0.Insert(xid, &statement.Insert{
		TableName: "user",
		Values:    []string{"1", "Alice", "20"},
	})
	if err != nil {
		t.Fatalf("insert 1 error: %v", err)
	}
	_, err = tbm0.Insert(xid, &statement.Insert{
		TableName: "user",
		Values:    []string{"2", "Tom", "18"},
	})
	if err != nil {
		t.Fatalf("insert 2 error: %v", err)
	}

	readOne, err := tbm0.Read(xid, &statement.Read{
		TableName: "user",
		Fields:    []string{"*"},
		Where: &statement.Where{
			SingleExp1: &statement.SingleExp{Field: "id", CmpOp: "=", Value: "1"},
		},
	})
	if err != nil {
		t.Fatalf("read id=1 error: %v", err)
	}
	if !bytes.Contains(readOne, []byte("Alice")) {
		t.Fatalf("read id=1 result mismatch: %s", readOne)
	}

	updateMsg, err := tbm0.Update(xid, &statement.Update{
		TableName: "user",
		FieldName: "name",
		Value:     "Bob",
		Where: &statement.Where{
			SingleExp1: &statement.SingleExp{Field: "id", CmpOp: "=", Value: "1"},
		},
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if string(updateMsg) != "Update 1" {
		t.Fatalf("update message=%s", updateMsg)
	}

	readAfterUpdate, err := tbm0.Read(xid, &statement.Read{
		TableName: "user",
		Fields:    []string{"*"},
		Where: &statement.Where{
			SingleExp1: &statement.SingleExp{Field: "id", CmpOp: "=", Value: "1"},
		},
	})
	if err != nil {
		t.Fatalf("read after update error: %v", err)
	}
	if !bytes.Contains(readAfterUpdate, []byte("Bob")) {
		t.Fatalf("read after update mismatch: %s", readAfterUpdate)
	}

	deleteMsg, err := tbm0.Delete(xid, &statement.Delete{
		TableName: "user",
		Where: &statement.Where{
			SingleExp1: &statement.SingleExp{Field: "id", CmpOp: "=", Value: "2"},
		},
	})
	if err != nil {
		t.Fatalf("delete error: %v", err)
	}
	if string(deleteMsg) != "Delete 1" {
		t.Fatalf("delete message=%s", deleteMsg)
	}

	commitMsg, err := tbm0.Commit(xid)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if string(commitMsg) != "commit" {
		t.Fatalf("commit message=%s", commitMsg)
	}

	xid2, _ := tbm0.Begin(&statement.Begin{})
	readDeleted, err := tbm0.Read(xid2, &statement.Read{
		TableName: "user",
		Fields:    []string{"*"},
		Where: &statement.Where{
			SingleExp1: &statement.SingleExp{Field: "id", CmpOp: "=", Value: "2"},
		},
	})
	if err != nil {
		t.Fatalf("read deleted row error: %v", err)
	}
	if len(readDeleted) != 0 {
		t.Fatalf("deleted row still exists: %s", readDeleted)
	}
	abortMsg := tbm0.Abort(xid2)
	if string(abortMsg) != "abort" {
		t.Fatalf("abort message=%s", abortMsg)
	}

	_, err = tbm0.Read(xid2, &statement.Read{TableName: "not_exists"})
	if err != ErrNoThatTable {
		t.Fatalf("read missing table err=%v want=%v", err, ErrNoThatTable)
	}

	xid3, _ := tbm0.Begin(&statement.Begin{})
	dropMsg, err := tbm0.Drop(xid3, &statement.Drop{TableName: "user"})
	if err != nil {
		t.Fatalf("drop error: %v", err)
	}
	if string(dropMsg) != "drop user" {
		t.Fatalf("drop message=%s", dropMsg)
	}
	showAfterDrop := tbm0.Show(xid3)
	if bytes.Contains(showAfterDrop, []byte("user")) {
		t.Fatalf("show still contains dropped table: %s", showAfterDrop)
	}
	_, err = tbm0.Read(xid3, &statement.Read{TableName: "user", Fields: []string{"*"}})
	if err != ErrNoThatTable {
		t.Fatalf("read dropped table err=%v want=%v", err, ErrNoThatTable)
	}
	if _, err := tbm0.Commit(xid3); err != nil {
		t.Fatalf("commit drop transaction error: %v", err)
	}

	dm0.Close()
	tm0.Close()

	tm1 := tm.Open(base)
	dm1, _ := dm.Open(base, mem, tm1)
	sm1 := sm.NewSerializabilityManager(tm1, dm1)
	tbm1 := Open(base, sm1, dm1)

	xid4, _ := tbm1.Begin(&statement.Begin{})
	showAfterReopen := tbm1.Show(xid4)
	if bytes.Contains(showAfterReopen, []byte("user")) {
		t.Fatalf("show after reopen still contains dropped table: %s", showAfterReopen)
	}
	_, err = tbm1.Read(xid4, &statement.Read{TableName: "user", Fields: []string{"*"}})
	if err != ErrNoThatTable {
		t.Fatalf("read dropped table after reopen err=%v want=%v", err, ErrNoThatTable)
	}
	tbm1.Abort(xid4)

	defer func() {
		dm1.Close()
		tm1.Close()
	}()
}
