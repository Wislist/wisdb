/*
table_manager.go 实现了TBM.
TBM用于管理表结构, 已经为上层模块提供更加高级和抽象的接口.
TBM会依赖IM进行索引, 依赖SM进行表单数据查找.

TBM本身的模型如下:
[TBM] -> [Booter] -> [Table1] -> [Table2] -> [Table3] ...
TBM将它管理的所有的表, 以链表的结构组织起来.
并利用Booter, 存储了第一张表的UUID.

	TBM目前没有实现表的可见性管理.

这样的目的是为了简洁代码.
*/
package tbm

import (
	"errors"
	"fmt"
	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/parser/statement"
	"mydb/src/main/backend/sm"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
	"mydb/src/main/backend/utils/booter"
	"sort"
	"sync"
)

var (
	ErrDuplicatedTable = errors.New("table already exists — use a different name or drop the existing table first")
	ErrNoThatTable     = errors.New("table not found — check table name or use SHOW to list available tables")
)

type TableManager interface {
	Begin(begin *statement.Begin) (tm.XID, []byte)
	Commit(xid tm.XID) ([]byte, error)
	Abort(xid tm.XID) []byte

	Show(xid tm.XID) []byte
	Create(xid tm.XID, create *statement.Create) ([]byte, error)
	Drop(xid tm.XID, drop *statement.Drop) ([]byte, error)

	Insert(xid tm.XID, insert *statement.Insert) ([]byte, error)
	Read(xid tm.XID, read *statement.Read) ([]byte, error)
	Update(xid tm.XID, update *statement.Update) ([]byte, error)
	Delete(xid tm.XID, delete *statement.Delete) ([]byte, error)
}

type tableManager struct {
	DM dm.DataManager
	SM sm.SerializabilityManager

	booter booter.Booter

	tc   map[string]*table
	xtc  map[tm.XID][]*table
	lock sync.Mutex
}

func newTableManager(sm sm.SerializabilityManager, dm dm.DataManager, booter booter.Booter) *tableManager {
	tbm := &tableManager{
		DM:     dm,
		SM:     sm,
		booter: booter,
		tc:     make(map[string]*table),
		xtc:    make(map[tm.XID][]*table),
	}

	tbm.loadTables()
	return tbm
}

func Create(path string, sm sm.SerializabilityManager, dm dm.DataManager) *tableManager {
	booter := booter.Create(path)
	booter.Update(utils.UUIDToRaw(utils.NilUUID))
	return newTableManager(sm, dm, booter)
}

func Open(path string, sm sm.SerializabilityManager, dm dm.DataManager) *tableManager {
	booter := booter.Open(path)
	return newTableManager(sm, dm, booter)
}

// loadTables 将所有的table读入内存.
func (tbm *tableManager) loadTables() {
	uuid := tbm.firstTableUUID()
	for uuid != utils.NilUUID {
		tb := LoadTable(tbm, uuid)
		uuid = tb.Next
		tbm.tc[tb.Name] = tb
	}
}

func (tbm *tableManager) firstTableUUID() utils.UUID {
	raw := tbm.booter.Load()
	return utils.ParseUUID(raw)
}

func (tbm *tableManager) updateFirstTableUUID(uuid utils.UUID) {
	raw := utils.UUIDToRaw(uuid)
	tbm.booter.Update(raw)
}

func (tbm *tableManager) Read(xid tm.XID, read *statement.Read) ([]byte, error) {
	tbm.lock.Lock()
	tb, ok := tbm.tc[read.TableName]
	tbm.lock.Unlock()
	if ok == false {
		return nil, ErrNoThatTable
	}

	result, err := tb.Read(xid, read)
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

func (tbm *tableManager) Update(xid tm.XID, update *statement.Update) ([]byte, error) {
	tbm.lock.Lock()
	tb, ok := tbm.tc[update.TableName]
	tbm.lock.Unlock()
	if ok == false {
		return nil, ErrNoThatTable
	}

	count, err := tb.Update(xid, update)
	if err != nil {
		return nil, err
	}
	return []byte("Update " + utils.Uint32ToStr(uint32(count))), nil
}

func (tbm *tableManager) Delete(xid tm.XID, delete *statement.Delete) ([]byte, error) {
	tbm.lock.Lock()
	tb, ok := tbm.tc[delete.TableName]
	tbm.lock.Unlock()
	if ok == false {
		return nil, ErrNoThatTable
	}

	count, err := tb.Delete(xid, delete)
	if err != nil {
		return nil, err
	}
	return []byte("Delete " + utils.Uint32ToStr(uint32(count))), nil
}

func (tbm *tableManager) Insert(xid tm.XID, insert *statement.Insert) ([]byte, error) {
	tbm.lock.Lock()
	tb, ok := tbm.tc[insert.TableName]
	tbm.lock.Unlock()
	if ok == false {
		return nil, ErrNoThatTable
	}

	err := tb.Insert(xid, insert)
	if err != nil {
		return nil, err
	}
	return []byte("Insert"), nil
}

func (tbm *tableManager) Create(xid tm.XID, create *statement.Create) ([]byte, error) {
	tbm.lock.Lock()
	defer tbm.lock.Unlock()

	_, ok := tbm.tc[create.TableName]
	if ok == true { // 已经存在
		return nil, fmt.Errorf("table %s: %w", create.TableName, ErrDuplicatedTable)
	}

	// 直接创建新表
	tb, err := CreateTable(tbm, tbm.firstTableUUID(), xid, create)
	if err != nil {
		return nil, err
	} else { // 创建成功
		tbm.updateFirstTableUUID(tb.SelfUUID)
		tbm.tc[create.TableName] = tb
		tbm.xtc[xid] = append(tbm.xtc[xid], tb)
		return []byte("create " + create.TableName), nil
	}
}

func (tbm *tableManager) Drop(xid tm.XID, drop *statement.Drop) ([]byte, error) {
	tbm.lock.Lock()
	defer tbm.lock.Unlock()

	if _, ok := tbm.tc[drop.TableName]; ok == false {
		return nil, ErrNoThatTable
	}

	delete(tbm.tc, drop.TableName)
	for txid, tables := range tbm.xtc {
		filtered := tables[:0]
		for _, tb := range tables {
			if tb.Name != drop.TableName {
				filtered = append(filtered, tb)
			}
		}
		tbm.xtc[txid] = filtered
	}

	if err := tbm.rebuildTableChain(xid); err != nil {
		return nil, err
	}
	return []byte("drop " + drop.TableName), nil
}

func (tbm *tableManager) rebuildTableChain(xid tm.XID) error {
	tables := make([]*table, 0, len(tbm.tc))
	for _, tb := range tbm.tc {
		tables = append(tables, tb)
	}
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})

	next := utils.NilUUID
	for i := len(tables) - 1; i >= 0; i-- {
		tables[i].Next = next
		if err := tables[i].persistSelf(xid); err != nil {
			return err
		}
		next = tables[i].SelfUUID
	}
	tbm.updateFirstTableUUID(next)
	return nil
}

/*
Show 返回所有的表名.
*/
func (tbm *tableManager) Show(xid tm.XID) []byte {
	tbm.lock.Lock()
	defer tbm.lock.Unlock()
	var results []byte

	// 当前事务创建的表（未提交，不在 tc 中）
	xtcSet := make(map[string]bool)
	for _, t := range tbm.xtc[xid] {
		xtcSet[t.Name] = true
		tPrint := t.Print()
		results = append(results, tPrint...)
		results = append(results, '\n')
	}

	// 已提交的表（排除当前事务创建的，避免重复）
	for _, t := range tbm.tc {
		if xtcSet[t.Name] {
			continue
		}
		tPrint := t.Print()
		results = append(results, tPrint...)
		results = append(results, '\n')
	}

	return results
}

func (tbm *tableManager) Begin(begin *statement.Begin) (tm.XID, []byte) {
	var level int
	if begin.IsRepeatableRead {
		level = 1
	}
	xid, err := tbm.SM.Begin(level)
	if err != nil {
		return 0, nil
	}
	return xid, []byte("begin")
}

func (tbm *tableManager) Commit(xid tm.XID) ([]byte, error) {
	err := tbm.SM.Commit(xid)
	if err != nil {
		return nil, err
	}
	return []byte("commit"), nil
}

func (tbm *tableManager) Abort(xid tm.XID) []byte {
	tbm.SM.Abort(xid)
	return []byte("abort")
}
