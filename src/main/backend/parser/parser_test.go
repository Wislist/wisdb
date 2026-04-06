package parser

import (
	"errors"
	"testing"

	"mydb/src/main/backend/parser/statement"
)

func TestTokenerSequence(t *testing.T) {
	tk := newTokener([]byte(`read name,age from user where city = "shanghai"`))
	var got []string
	for {
		token, err := tk.Peek()
		if err != nil {
			t.Fatalf("Peek error: %v", err)
		}
		if token == "" {
			break
		}
		got = append(got, token)
		tk.Pop()
	}

	want := []string{"read", "name", ",", "age", "from", "user", "where", "city", "=", "shanghai"}
	if len(got) != len(want) {
		t.Fatalf("token count mismatch: got=%d want=%d tokens=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token[%d] mismatch: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestParseCreateReadInsertUpdate(t *testing.T) {
	createStat, err := Parse([]byte(`create table user id uint64, name string, age uint32 (index id)`))
	if err != nil {
		t.Fatalf("Parse create error: %v", err)
	}
	create, ok := createStat.(*statement.Create)
	if !ok {
		t.Fatalf("create type mismatch: %T", createStat)
	}
	if create.TableName != "user" || len(create.FieldName) != 3 || len(create.Index) != 1 || create.Index[0] != "id" {
		t.Fatalf("unexpected create stat: %+v", create)
	}

	readStat, err := Parse([]byte(`read id,name from user where id = 10 and age > 18`))
	if err != nil {
		t.Fatalf("Parse read error: %v", err)
	}
	read, ok := readStat.(*statement.Read)
	if !ok {
		t.Fatalf("read type mismatch: %T", readStat)
	}
	if read.TableName != "user" || len(read.Fields) != 2 || read.Where == nil || read.Where.LogicOp != "and" {
		t.Fatalf("unexpected read stat: %+v", read)
	}

	insertStat, err := Parse([]byte(`insert into user values 1 "tom" 20`))
	if err != nil {
		t.Fatalf("Parse insert error: %v", err)
	}
	insert, ok := insertStat.(*statement.Insert)
	if !ok {
		t.Fatalf("insert type mismatch: %T", insertStat)
	}
	if insert.TableName != "user" || len(insert.Values) != 3 || insert.Values[1] != "tom" {
		t.Fatalf("unexpected insert stat: %+v", insert)
	}

	updateStat, err := Parse([]byte(`update user set name = "jerry" where id = 1`))
	if err != nil {
		t.Fatalf("Parse update error: %v", err)
	}
	update, ok := updateStat.(*statement.Update)
	if !ok {
		t.Fatalf("update type mismatch: %T", updateStat)
	}
	if update.TableName != "user" || update.FieldName != "name" || update.Value != "jerry" || update.Where == nil {
		t.Fatalf("unexpected update stat: %+v", update)
	}
}

func TestParseTxnAndSimpleCommands(t *testing.T) {
	beginStat, err := Parse([]byte(`begin isolation level repeatable read`))
	if err != nil {
		t.Fatalf("Parse begin error: %v", err)
	}
	begin, ok := beginStat.(*statement.Begin)
	if !ok {
		t.Fatalf("begin type mismatch: %T", beginStat)
	}
	if !begin.IsRepeatableRead {
		t.Fatalf("begin repeatable read not parsed")
	}

	if _, err := Parse([]byte(`commit`)); err != nil {
		t.Fatalf("Parse commit error: %v", err)
	}
	if _, err := Parse([]byte(`abort`)); err != nil {
		t.Fatalf("Parse abort error: %v", err)
	}
	if _, err := Parse([]byte(`show`)); err != nil {
		t.Fatalf("Parse show error: %v", err)
	}
}

func TestParseInvalidStatements(t *testing.T) {
	cases := [][]byte{
		[]byte(`unknown cmd`),
		[]byte(`create table t id uint64`),
		[]byte(`insert user values 1`),
		[]byte(`read from user`),
		[]byte(`update user name = 1`),
		[]byte(`show extra`),
		[]byte(`read id from user where x ! 1`),
	}

	for _, stat := range cases {
		_, err := Parse(stat)
		if err == nil {
			t.Fatalf("expected parse error for: %s", stat)
		}
	}

	_, err := Parse([]byte(`create table t id uint64`))
	if !errors.Is(err, ErrHasNoIndex) {
		t.Fatalf("expect ErrHasNoIndex, got: %v", err)
	}
}

