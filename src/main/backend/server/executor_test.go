package server

import (
	"bytes"
	"errors"
	"testing"

	"mydb/src/main/backend/parser/statement"
	"mydb/src/main/backend/tm"
)

type fakeTableManager struct {
	nextXID tm.XID

	beginCalls  int
	beginLevels []bool

	commitXIDs []tm.XID
	commitErr  error

	abortXIDs []tm.XID

	showXIDs []tm.XID
	showResp []byte

	createXIDs []tm.XID
	createErr  error
	createResp []byte
}

func newFakeTableManager() *fakeTableManager {
	return &fakeTableManager{
		nextXID:    100,
		showResp:   []byte("show"),
		createResp: []byte("create ok"),
	}
}

func (f *fakeTableManager) Begin(begin *statement.Begin) (tm.XID, []byte) {
	f.beginCalls++
	f.beginLevels = append(f.beginLevels, begin.IsRepeatableRead)
	xid := f.nextXID
	f.nextXID++
	return xid, []byte("begin")
}

func (f *fakeTableManager) Commit(xid tm.XID) ([]byte, error) {
	f.commitXIDs = append(f.commitXIDs, xid)
	if f.commitErr != nil {
		return nil, f.commitErr
	}
	return []byte("commit"), nil
}

func (f *fakeTableManager) Abort(xid tm.XID) []byte {
	f.abortXIDs = append(f.abortXIDs, xid)
	return []byte("abort")
}

func (f *fakeTableManager) Show(xid tm.XID) []byte {
	f.showXIDs = append(f.showXIDs, xid)
	return f.showResp
}

func (f *fakeTableManager) Create(xid tm.XID, create *statement.Create) ([]byte, error) {
	f.createXIDs = append(f.createXIDs, xid)
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.createResp, nil
}

func (f *fakeTableManager) Insert(xid tm.XID, insert *statement.Insert) ([]byte, error) {
	return []byte("insert"), nil
}

func (f *fakeTableManager) Read(xid tm.XID, read *statement.Read) ([]byte, error) {
	return []byte("read"), nil
}

func (f *fakeTableManager) Update(xid tm.XID, update *statement.Update) ([]byte, error) {
	return []byte("update"), nil
}

func (f *fakeTableManager) Delete(xid tm.XID, delete *statement.Delete) ([]byte, error) {
	return []byte("delete"), nil
}

// TestExecutorExplicitTransactionFlow 验证显式事务的 begin/show/commit 流程与事务状态约束。
func TestExecutorExplicitTransactionFlow(t *testing.T) {
	ft := newFakeTableManager()
	exe := NewExecutor(ft)

	resp, err := exe.Execute([]byte("begin"))
	if err != nil || string(resp) != "begin" {
		t.Fatalf("begin failed, resp=%s err=%v", resp, err)
	}
	if ft.beginCalls != 1 {
		t.Fatalf("begin calls=%d want=1", ft.beginCalls)
	}

	_, err = exe.Execute([]byte("begin"))
	if !errors.Is(err, ErrNoNestedTransaction) {
		t.Fatalf("nested begin err=%v want=%v", err, ErrNoNestedTransaction)
	}

	resp, err = exe.Execute([]byte("show"))
	if err != nil || !bytes.Equal(resp, ft.showResp) {
		t.Fatalf("show in transaction failed, resp=%s err=%v", resp, err)
	}
	if ft.beginCalls != 1 {
		t.Fatalf("show should not create tmp transaction")
	}

	resp, err = exe.Execute([]byte("commit"))
	if err != nil || string(resp) != "commit" {
		t.Fatalf("commit failed, resp=%s err=%v", resp, err)
	}
	if len(ft.commitXIDs) != 1 || ft.commitXIDs[0] != 100 {
		t.Fatalf("commit xid mismatch, got=%v", ft.commitXIDs)
	}

	_, err = exe.Execute([]byte("commit"))
	if !errors.Is(err, ErrNotInAnyTransaction) {
		t.Fatalf("commit without transaction err=%v want=%v", err, ErrNotInAnyTransaction)
	}
}

// TestExecutorAutoTransactionCommitOnSuccess 验证非事务语句会自动开启临时事务并在成功后提交。
func TestExecutorAutoTransactionCommitOnSuccess(t *testing.T) {
	ft := newFakeTableManager()
	exe := NewExecutor(ft)

	resp, err := exe.Execute([]byte("show"))
	if err != nil {
		t.Fatalf("show execute error: %v", err)
	}
	if !bytes.Equal(resp, ft.showResp) {
		t.Fatalf("show resp mismatch: %s", resp)
	}
	if ft.beginCalls != 1 {
		t.Fatalf("begin calls=%d want=1", ft.beginCalls)
	}
	if len(ft.commitXIDs) != 1 || ft.commitXIDs[0] != 100 {
		t.Fatalf("commit xid mismatch, got=%v", ft.commitXIDs)
	}
	if len(ft.abortXIDs) != 0 {
		t.Fatalf("unexpected abort calls=%v", ft.abortXIDs)
	}
	if len(ft.showXIDs) != 1 || ft.showXIDs[0] != 100 {
		t.Fatalf("show xid mismatch, got=%v", ft.showXIDs)
	}
}

// TestExecutorAutoTransactionAbortOnFailure 验证非事务语句失败时会自动回滚临时事务。
func TestExecutorAutoTransactionAbortOnFailure(t *testing.T) {
	ft := newFakeTableManager()
	ft.createErr = errors.New("create failed")
	exe := NewExecutor(ft)

	_, err := exe.Execute([]byte("create table user id uint64 (index id)"))
	if err == nil {
		t.Fatalf("expected create error")
	}
	if ft.beginCalls != 1 {
		t.Fatalf("begin calls=%d want=1", ft.beginCalls)
	}
	if len(ft.abortXIDs) != 1 || ft.abortXIDs[0] != 100 {
		t.Fatalf("abort xid mismatch, got=%v", ft.abortXIDs)
	}
	if len(ft.commitXIDs) != 0 {
		t.Fatalf("unexpected commit calls=%v", ft.commitXIDs)
	}
}

// TestExecutorCloseAbortActiveTransaction 验证执行器关闭时会回滚未结束的事务。
func TestExecutorCloseAbortActiveTransaction(t *testing.T) {
	ft := newFakeTableManager()
	exe := NewExecutor(ft)

	_, err := exe.Execute([]byte("begin"))
	if err != nil {
		t.Fatalf("begin error: %v", err)
	}
	exe.Close()
	if len(ft.abortXIDs) != 1 || ft.abortXIDs[0] != 100 {
		t.Fatalf("close abort xid mismatch, got=%v", ft.abortXIDs)
	}
}

// TestExecutorParseError 验证SQL解析失败时不会触发表管理器调用。
func TestExecutorParseError(t *testing.T) {
	ft := newFakeTableManager()
	exe := NewExecutor(ft)

	_, err := exe.Execute([]byte("not-a-valid-sql"))
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if ft.beginCalls != 0 {
		t.Fatalf("parse error should not call table manager")
	}
}
