package tbm

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"mydb/src/main/backend/parser/statement"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)


var (
	ErrInvalidValues   = errors.New("value count does not match field count — check CREATE TABLE for expected fields")
	ErrInvalidLogOP    = errors.New("unsupported logic operator — use AND or OR (max 2 conditions)")
	ErrNoThatField     = errors.New("field not found in table — check field name in CREATE TABLE")
	ErrFieldHasNoField = errors.New("field is not indexed — add it to the index list in CREATE TABLE, e.g. (index id age)")
)


// map[Field]Value
type entry map[string]interface{}

type table struct {
	TBM      *tableManager
	SelfUUID utils.UUID

	Name   string
	status byte
	Next   utils.UUID
	fields []*field
}

/*
	LoadTable 从数据库中将uuid指定的table读入内存.
	该函数只会在TM启动时被调用.
	因为该函数被调用时, 为单线程, 所以不会有ErrCacheFull之类的错误, 因此一旦遇到错误, 那一定
	是不可恢复的错误, 应该直接panic.
*/
func LoadTable(tbm *tableManager, uuid utils.UUID) *table {
	raw, ok, err := tbm.SM.Read(tm.SUPER_XID, uuid)
	utils.Assert(ok)
	if err != nil {
		panic(err)
	}

	tb := &table{
		TBM:      tbm,
		SelfUUID: uuid,
	}

	tb.parseSelf(raw)
	return tb
}

// parseSelf 通过raw解析出table自己的信息.
func (t *table) parseSelf(raw []byte) {
	var pos, shift int
	t.Name, shift = utils.ParseVarStr(raw[pos:])
	pos += shift
	t.Next = utils.ParseUUID(raw[pos:])
	pos += utils.LEN_UUID

	for pos < len(raw) {
		uuid := utils.ParseUUID(raw[pos:])
		pos += utils.LEN_UUID
		f := LoadField(t, uuid)
		t.fields = append(t.fields, f)
	}
}

// CreateTable 创建一张表， 并返回其指针
func CreateTable(tbm *tableManager, next utils.UUID, xid tm.XID, create *statement.Create) (*table, error) {
	tb := &table{
		TBM: tbm,
		Name: create.TableName,
		Next: next,
	}

	for i := 0; i < len(create.FieldName); i++ {
		fname := create.FieldName[i]
		ftype := create.FieldType[i]
		indexed := false
		for j := 0; j < len(create.Index); j ++ {
			if create.Index[j] == fname {
				indexed = true
				break
			}
		}
		// 表结构元数据用 SUPER_XID 写入，避免 Undo 时破坏表结构
		field, err := CreateField(tb, tm.SUPER_XID, fname, ftype, indexed)
		if err != nil {
			return nil, err
		}
		tb.fields = append(tb.fields, field)
	}

	// 表元数据同样用 SUPER_XID 写入
	err := tb.persistSelf(tm.SUPER_XID)
	if err != nil {
		return nil, err
	}
	return tb, nil
} 

// persist 将t自身持久化到磁盘上, 该函数只会在CreateTable的时候被调用
func (t *table) persistSelf(xid tm.XID) error {
	raw := utils.VarStrToRaw(t.Name)
	raw = append(raw, utils.UUIDToRaw(t.Next)...)
	for _, f := range t.fields {
		raw = append(raw, utils.UUIDToRaw(f.SelfUUID)...)
	}

	self, err := t.TBM.SM.Insert(xid, raw)
	if err != nil {
		return err
	}

	t.SelfUUID = self
	return nil
}


func (t *table) Print() string {
	str := "{"
	str += t.Name + ": "
	for i := 0; i < len(t.fields); i++ {
		str += t.fields[i].Print()
		if i == len(t.fields)-1 {
			str += "}"
		} else {
			str += ", "
		}
	}
	return str
}

func (t *table) Delete(xid tm.XID, delete *statement.Delete) (int, error) {
	uuids, err := t.parseWhere(xid, delete.Where)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, uuid := range uuids {
		ok, err := t.TBM.SM.Delete(xid, uuid)
		if err != nil {
			return 0, err
		}
		if ok {
			count++
		}
	}

	return count, nil
}

func (t *table) Update(xid tm.XID, update *statement.Update) (int, error) {
	uuids, err := t.parseWhere(xid, update.Where)
	if err != nil {
		return 0, err
	}

	var fd *field
	for _, f := range t.fields {
		if f.FName == update.FieldName {
			fd = f
			break
		}
	}
	if fd == nil {
		return 0, fmt.Errorf("%s: %w", update.FieldName, ErrNoThatField)
	}
	v, err := fd.StrToValue(update.Value)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, uuid := range uuids {
		raw, ok, err := t.TBM.SM.Read(xid, uuid)
		if err != nil {
			return 0, err
		}
		if ok == false {
			continue
		}

		_, err = t.TBM.SM.Delete(xid, uuid) // 删除原来的entry
		if err != nil {
			return 0, err
		}

		e := t.parseEntry(raw) // 读取并解析entry
		e[fd.FName] = v        // 更新entry
		raw = t.entryToRaw(e)  // 将新entry存储进DB
		uuid, err = t.TBM.SM.Insert(xid, raw)
		if err != nil {
			return 0, err
		}

		count++

		for _, f := range t.fields { // 更新对应的索引
			if f.IsIndexed() {
				err := f.Insert(e[f.FName], uuid)
				if err != nil {
					return 0, err
				}
			}
		}
	}

	return count, nil
}

func (t *table) Read(xid tm.XID, read *statement.Read) (string, error) {
	uuids, err := t.parseWhere(xid, read.Where)
	if err != nil {
		return "", err
	}

	// Parse all entries
	var entries []entry
	for _, uuid := range uuids {
		raw, ok, err := t.TBM.SM.Read(xid, uuid)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		entries = append(entries, t.parseEntry(raw))
	}

	// ORDER BY
	if read.OrderBy != "" {
		t.sortEntries(entries, read.OrderBy, read.OrderDesc)
	}

	// LIMIT / OFFSET
	if read.Limit > 0 {
		entries = t.limitEntries(entries, read.Limit, read.Offset)
	}

	// Aggregates
	if len(read.Aggregates) > 0 {
		return t.computeAggregates(entries, read.Aggregates), nil
	}

	// Regular output
	result := ""
	for _, e := range entries {
		result += t.entryPrint(e) + "\n"
	}
	return result, nil
}

// parseWhere 对where语句进行解析, 返回field, 该where对应区间内的uuid
func (t *table) parseWhere(xid tm.XID, where *statement.Where) ([]utils.UUID, error) {
	var l0, r0, l1, r1 utils.UUID
	single := false
	var err error
	var fd *field

	if where == nil {
		for _, f := range t.fields {
			if f.IsIndexed() {
				fd = f
				break
			}
		}
		l0, r0 = 0, utils.INF
		single = true
	} else if where != nil {
		for _, f := range t.fields {
			if f.FName == where.SingleExp1.Field {
				fd = f
				break
			}
		}
		if fd == nil {
			return nil, ErrNoThatField
		}

		// If the field is not indexed, fall back to full table scan with in-memory filter
		if fd.IsIndexed() == false {
			return t.fullScanFilter(xid, where)
		}

		l0, r0, l1, r1, single, err = t.calWhere(fd, where)
		if err != nil {
			return nil, err
		}
	}

	uuids, err := fd.Search(l0, r0)
	if err != nil {
		return nil, err
	}
	if single == false {
		tmp, err := fd.Search(l1, r1)
		if err != nil {
			return nil, err
		}
		uuids = append(uuids, tmp...)
	}

	return uuids, nil
}

// calWhere 计算该where语句所表示的key的区间.
// 由于where或许有or, 所以区间可能为2个.
func (t *table) calWhere(fd *field, where *statement.Where) (l0, r0, l1, r1 utils.UUID, single bool, err error) {
	if where.LogicOp == "" { // single
		single = true
		l0, r0, err = fd.CalExp(where.SingleExp1)
	} else if where.LogicOp == "or" {
		single = false
		l0, r0, err = fd.CalExp(where.SingleExp1)
		if err != nil {
			return
		}
		l1, r1, err = fd.CalExp(where.SingleExp2)
	} else if where.LogicOp == "and" {
		single = true
		l0, r0, err = fd.CalExp(where.SingleExp1)
		if err != nil {
			return
		}
		l1, r1, err = fd.CalExp(where.SingleExp2)
		// 合并[l0, r0], [l1, r1]两个区间
		if l1 > l0 {
			l0 = l1
		}
		if r1 < r0 {
			r0 = r1
		}
		return
	} else {
		err = ErrInvalidLogOP
	}
	return
}
// fullScanFilter performs a full table scan (via any indexed field) and filters rows in memory
// against the WHERE condition. Used when the queried field has no index.
func (t *table) fullScanFilter(xid tm.XID, where *statement.Where) ([]utils.UUID, error) {
	// Find any indexed field to scan all rows
	var scanField *field
	for _, f := range t.fields {
		if f.IsIndexed() {
			scanField = f
			break
		}
	}
	if scanField == nil {
		return nil, errors.New("cannot perform full scan: table has no indexed fields — add at least one index in CREATE TABLE")
	}

	allUUIDs, err := scanField.Search(0, utils.INF)
	if err != nil {
		return nil, err
	}

	var result []utils.UUID
	for _, uuid := range allUUIDs {
		raw, ok, err := t.TBM.SM.Read(xid, uuid)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		entry := t.parseEntry(raw)
		if t.evaluateWhere(entry, where) {
			result = append(result, uuid)
		}
	}
	return result, nil
}

// evaluateWhere checks if an entry satisfies a WHERE clause.
func (t *table) evaluateWhere(entry entry, where *statement.Where) bool {
	if where == nil {
		return true
	}
	ok1 := t.matchSingleExp(entry, where.SingleExp1)
	if where.LogicOp == "" {
		return ok1
	}
	ok2 := t.matchSingleExp(entry, where.SingleExp2)
	if where.LogicOp == "and" {
		return ok1 && ok2
	}
	return ok1 || ok2
}

// matchSingleExp checks if an entry satisfies a single WHERE condition.
func (t *table) matchSingleExp(entry entry, exp *statement.SingleExp) bool {
	v, ok := entry[exp.Field]
	if !ok {
		return false
	}
	var fd *field
	for _, f := range t.fields {
		if f.FName == exp.Field {
			fd = f
			break
		}
	}
	if fd == nil {
		return false
	}
	target, err := fd.StrToValue(exp.Value)
	if err != nil {
		return false
	}
	cmp := fd.valueCompare(v, target)
	switch exp.CmpOp {
	case "=":
		return cmp == 0
	case "<":
		return cmp < 0
	case ">":
		return cmp > 0
	}
	return false
}

// sortEntries sorts entries by a field.
func (t *table) sortEntries(entries []entry, fieldName string, desc bool) {
	var fd *field
	for _, f := range t.fields {
		if f.FName == fieldName {
			fd = f
			break
		}
	}
	if fd == nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		cmp := fd.valueCompare(entries[i][fieldName], entries[j][fieldName])
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

// limitEntries applies LIMIT and OFFSET to a slice of entries.
func (t *table) limitEntries(entries []entry, limit, offset int) []entry {
	if offset >= len(entries) {
		return nil
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	return entries[offset:end]
}

// computeAggregates computes aggregate functions over entries.
func (t *table) computeAggregates(entries []entry, aggs []statement.Aggregate) string {
	var parts []string
	for _, agg := range aggs {
		switch agg.Func {
		case "count":
			if agg.Field == "*" {
				parts = append(parts, fmt.Sprintf("count(*)=%d", len(entries)))
			}
		case "sum":
			var total uint64
			for _, e := range entries {
				v := e[agg.Field]
				switch v := v.(type) {
				case uint32:
					total += uint64(v)
				case uint64:
					total += v
				}
			}
			parts = append(parts, fmt.Sprintf("sum(%s)=%d", agg.Field, total))
		case "avg":
			var total uint64
			for _, e := range entries {
				v := e[agg.Field]
				switch v := v.(type) {
				case uint32:
					total += uint64(v)
				case uint64:
					total += v
				}
			}
			if len(entries) > 0 {
				parts = append(parts, fmt.Sprintf("avg(%s)=%d", agg.Field, total/uint64(len(entries))))
			} else {
				parts = append(parts, fmt.Sprintf("avg(%s)=0", agg.Field))
			}
		}
	}
	return strings.Join(parts, " ")
}

// Insert 对该表执行insert语句.
func (t *table) Insert(xid tm.XID, insert *statement.Insert) error {
	e, err := t.strToEntry(insert.Values) // 将insert的values转换为entry
	if err != nil {
		return err
	}

	raw := t.entryToRaw(e) // 将该entry插入到DB
	uuid, err := t.TBM.SM.Insert(xid, raw)
	if err != nil {
		return err
	}

	for _, f := range t.fields { // 更新对应的索引
		if f.IsIndexed() {
			err := f.Insert(e[f.FName], uuid)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *table) strToEntry(values []string) (entry, error) {
	if len(values) != len(t.fields) {
		return nil, fmt.Errorf("got %d values, expected %d fields: %w", len(values), len(t.fields), ErrInvalidValues)
	}

	e := entry{}
	for i, f := range t.fields {
		v, err := f.StrToValue(values[i])
		if err != nil {
			return nil, err
		}
		e[f.FName] = v
	}

	return e, nil
}

func (t *table) entryToRaw(e entry) []byte {
	var raw []byte
	for _, f := range t.fields {
		raw = append(raw, f.ValueToRaw(e[f.FName])...)
	}
	return raw
}

func (t *table) parseEntry(raw []byte) entry {
	var pos, shift int
	e := entry{}
	for _, f := range t.fields {
		e[f.FName], shift = f.ParseValue(raw[pos:])
		pos += shift
	}
	return e
}

func (t *table) entryPrint(e entry) string {
	str := "["
	for i, f := range t.fields {
		str += f.ValuePrint(e[f.FName])
		if i == len(t.fields)-1 {
			str += "]"
		} else {
			str += ", "
		}
	}
	return str
}
