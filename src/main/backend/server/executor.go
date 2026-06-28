package server

import (
	"errors"
	"mydb/src/main/backend/parser"
	"mydb/src/main/backend/parser/statement"
	"mydb/src/main/backend/tbm"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)


var (
	ErrNoNestedTransaction = errors.New("cannot begin a transaction inside another transaction — commit or abort the current one first")
	ErrNotInAnyTransaction = errors.New("no active transaction — use begin first before commit/abort")
)

type Executor interface {
	Execute(sql []byte) ([]byte, error)
	Close()
}

type executor struct {
	xid tm.XID
	tbm tbm.TableManager
}

func NewExecutor(tbm tbm.TableManager) *executor {
	return &executor{
		tbm: tbm,
	}
}

func (e *executor) Close() {
	if e.xid != 0 {
		utils.Info("Abnormal Abort: ", e.xid)
		e.tbm.Abort(e.xid)
	}
}

func (e *executor) Execute(sql []byte) ([]byte, error) {
	utils.Info("Execute: ", string(sql))

	stat, err := parser.Parse(sql)
	if err != nil {
		return nil, err
	}

	var result []byte
	switch st := stat.(type) {
	case *statement.Begin:
		if e.xid != 0 {
			return nil, ErrNoNestedTransaction
		}
		e.xid, result = e.tbm.Begin(st)
		return result, nil
	case *statement.Commit:
		if e.xid == 0 {
			return nil, ErrNotInAnyTransaction
		}
		result, err = e.tbm.Commit(e.xid)
		if err != nil {
			return nil, err
		}
		e.xid = 0
		return result, nil
	case *statement.Abort:
		if e.xid == 0 {
			return nil, ErrNotInAnyTransaction
		}
		result = e.tbm.Abort(e.xid)
		e.xid = 0
		return result, nil
	default:
		return e.execute2(st)
	}
}

func (e *executor) execute2(stat interface{}) ([]byte, error) {
	var err error
	tmpTransaction := false
	if e.xid  == 0 {
		tmpTransaction = true
		xid, _ := e.tbm.Begin(new(statement.Begin))
		if xid == 0 {
			return nil, errors.New("failed to begin implicit transaction: transaction manager returned invalid XID — the database may be in an inconsistent state")
		}
		e.xid = xid
	}
	defer func() {
		if tmpTransaction {
			if err != nil {
				e.tbm.Abort(e.xid)
			} else {
				_, commitErr := e.tbm.Commit(e.xid)
				if commitErr != nil {
					utils.Info("Implicit transaction commit error:", commitErr)
					err = commitErr
				}
			}
			e.xid = 0
		}
	} ()

	var result []byte
	switch st := stat.(type) {
	case *statement.Show:
		result = e.tbm.Show(e.xid)
	case *statement.Create:
		result, err = e.tbm.Create(e.xid, st)
	case *statement.Drop:
		result, err = e.tbm.Drop(e.xid, st)
	case *statement.Read:
		result, err = e.tbm.Read(e.xid, st)
	case *statement.Insert:
		result, err = e.tbm.Insert(e.xid, st)
	case *statement.Delete:
		result, err = e.tbm.Delete(e.xid, st)
	case *statement.Update:
		result, err = e.tbm.Update(e.xid, st)
	}

	return result, err
}
