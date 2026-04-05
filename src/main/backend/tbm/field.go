package tbm

import (
	"errors"
	"mydb/src/main/backend/im"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)

/*
	field.go 管理具体字段

	一个field的二进制格式为
	[Field Name]    string
	[Type Name]     string
	[Index UUID]    UUID

	如果该field没有索引，那么[Index UUID]为NilUUID
*/

var (
	ErrInvalidFieldType  = errors.New("Invalid field type.")
	ErrInvalidFieldValue = errors.New("Invalid field value")
)

type field struct {
	SelfUUID utils.UUID
	tb       *table

	FName    string
	FType    string
	index    utils.UUID
	bt       im.BPlusTree
}

/*
	LoadField 从DB中加载field
	panic的原因和LoadTable类似
*/
func LoadField(tb *table, uuid utils.UUID) *field {
	raw, ok, err := tb.TBM.SM.Read(tm.SUPER_XID, uuid)
	utils.Assert(ok)
	if err != nil {
		panic(err)
	}

	f := &field{
		SelfUUID: uuid,
		tb:       tb,
	}

	f.parseSelf(raw)
	return f
}

func (f *field) parseSelf(raw []byte) {
	var pos, shift int
	f.FName, shift = utils.ParseVarStr(raw[pos:])
	pos += shift
	f.FType, shift = utils.ParseVarStr(raw[pos:])
	pos += shift
	f.index = utils.ParseUUID(raw[pos:])
	if f.index != utils.NilUUID {
		var err error
		f.bt, err = im.Load(f.index, &f.tb.TBM.DM)
		if err != nil {
			panic(err)
		}
	}
}

func CreateField(tb *table, xid tm.XID, fname, ftype string, indexed bool) (*field, error) {
	err := typeCheck(ftype)
	if err != nil {
		return nil, err
	}

	f := &field{
		tb: tb,
		FName: fname,
		FType: ftype,
		index: utils.NilUUID,
	}

	if indexed {
		index, err := im.Create(tb.TBM.DM)
		if err != nil {
			return nil, err
		}
		bt, err := im.Load(index, &tb.TBM.DM)
		if err != nil {
			return nil, err
		}
		f.index = index
		f.bt = bt
	}

	err = f.persistSelf(xid)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// persist 将该field持久化
func (f *field) persistSelf(xid tm.XID) error {
	raw := utils.VarStrToRaw(f.FName)
	raw = append(raw, utils.VarStrToRaw(f.FType)...)
	raw = append(raw, utils.UUIDToRaw(f.index)...)
	self, err := f.tb.TBM.SM.Insert(xid, raw)
	if err != nil {
		return err
	}
	f.SelfUUID = self
	return nil
}

func typeCheck(ftype string) error  {
	if ftype != "uint32" && ftype != "uint64" && ftype != "string" {
		return ErrInvalidFieldType
	}
	return nil
}

func (f *field) Print() string{
	str := "("
	str += f.FName
	str += "," + f.FType
	if f.index != utils.NilUUID {
		str += ", Index"
	} else {
		str += ", NoIndex"
	}
	str += ")"
	return str
}

func (f *field) IsIndexed() bool {
	return f.index != utils.NilUUID
}

func (f *field) Insert(key interface{}, uuid utils.UUID) error {
	ukey := f.ValueToUUID(key)
	return f.bt.Insert(ukey, uuid)
}





