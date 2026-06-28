package parser

import (
	"errors"
	"fmt"
	"mydb/src/main/backend/parser/statement"
)

var (
	ErrInvalidStat = errors.New("Invalid command. Supported: begin, commit, abort, create, drop, show, insert, read/select, update, delete")
)

func Parse(statement []byte) (interface{}, error) {
	tokener := newTokener(statement)
	token, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	tokener.Pop()

	var stat interface{}
	var staterr error

	switch token {
	case "begin":
		stat, staterr = parseBegin(tokener)
	case "commit":
		stat, staterr = parseCommit(tokener)
	case "abort":
		stat, staterr = parseAbort(tokener)
	case "create":
		stat, staterr = parseCreate(tokener)
	case "drop":
		stat, staterr = parseDrop(tokener)
	case "read", "select":
		stat, staterr = parseRead(tokener)
	case "insert":
		stat, staterr = parseInsert(tokener)
	case "delete":
		stat, staterr = parseDelete(tokener)
	case "update":
		stat, staterr = parseUpdate(tokener)
	case "show":
		stat, staterr = parseShow(tokener)
	default:
		return nil, ErrInvalidStat
	}

	next, err := tokener.Peek()
	if err == nil && next != "" {
		errStat := tokener.ErrStat()
		staterr = fmt.Errorf("unexpected token after statement at: %s. Hint: check syntax — ORDER BY comes after WHERE, LIMIT after ORDER BY", string(errStat))
	}

	return stat, staterr
}

func parseShow(tokener *tokener) (*statement.Show, error) {
	tmp, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if tmp == "" {
		return new(statement.Show), nil
	} else {
		return nil, ErrInvalidStat
	}
}

func parseUpdate(tokener *tokener) (*statement.Update, error) {
	var err error
	update := new(statement.Update)
	update.TableName, err = tokener.Peek()
	if err != nil {
		return nil, err
	}
	tokener.Pop()

	set, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if set != "set" {
		return nil, ErrInvalidStat
	}
	tokener.Pop()

	update.FieldName, err = tokener.Peek()
	if err != nil {
		return nil, err
	}
	tokener.Pop()

	tmp, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if tmp != "=" {
		return nil, ErrInvalidStat
	}
	tokener.Pop()

	update.Value, err = tokener.Peek()
	if err != nil {
		return nil, err
	}
	tokener.Pop()

	tmp, err = tokener.Peek()
	if err != nil {
		return nil, err
	}
	if tmp == "" {
		update.Where = nil
		return update, nil
	}

	where, err := parseWhere(tokener)
	if err != nil {
		return nil, err
	}
	update.Where = where
	return update, nil
}

func parseDelete(tokener *tokener) (*statement.Delete, error) {
	deleteStat := new(statement.Delete)

	from, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if from != "from" {
		return nil, ErrInvalidStat
	}

	tokener.Pop()
	tableName, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if isName(tableName) == false {
		return nil, ErrInvalidStat
	}
	deleteStat.TableName = tableName

	tokener.Pop()
	where, err := parseWhere(tokener)
	if err != nil {
		return nil, err
	}
	deleteStat.Where = where
	return deleteStat, nil
}

func parseInsert(tokener *tokener) (*statement.Insert, error) {
	insert := new(statement.Insert)

	into, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if into != "into" {
		return nil, ErrInvalidStat
	}

	tokener.Pop()
	tableName, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if isName(tableName) == false {
		return nil, ErrInvalidStat
	}
	insert.TableName = tableName

	tokener.Pop()
	values, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if values != "values" {
		return nil, ErrInvalidStat
	}

	// Peek at the next token to decide single vs bulk insert
	tokener.Pop()
	first, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if first == "" {
		return nil, ErrInvalidStat
	}

	if first == "{" {
		// Bulk insert: {v1, v2, v3}, {v4, v5, v6}, ...
		return parseInsertBulk(tokener, insert)
	}

	// Single-row insert (legacy): val1 val2 val3 ...
	insert.Values = append(insert.Values, first)
	for {
		tokener.Pop()
		value, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		if value == "" {
			break
		}
		insert.Values = append(insert.Values, value)
	}

	return insert, nil
}

func parseInsertBulk(tokener *tokener, insert *statement.Insert) (*statement.Insert, error) {
	for {
		var row []string
		for {
			tokener.Pop()
			value, err := tokener.Peek()
			if err != nil {
				return nil, err
			}
			if value == "" {
				return nil, ErrInvalidStat
			}
			if value == "}" {
				break // end of current row
			}
			if value == "," {
				continue // next value in current row
			}
			row = append(row, value)
		}
		insert.ValuesList = append(insert.ValuesList, row)

		// Check for more rows
		tokener.Pop()
		next, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		if next == "" {
			break // end of statement
		}
		if next == "," {
			// Expect { for next row
			tokener.Pop()
			br, err := tokener.Peek()
			if err != nil {
				return nil, err
			}
			if br != "{" {
				return nil, ErrInvalidStat
			}
			continue
		}
		return nil, ErrInvalidStat
	}

	return insert, nil
}

func parseRead(tokener *tokener) (*statement.Read, error) {
	read := new(statement.Read)

	asterisk, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if asterisk == "*" {
		read.Fields = append(read.Fields, "*")
		tokener.Pop()
	} else {
		for {
			field, err := tokener.Peek()
			if err != nil {
				return nil, err
			}

			if isAggFunc(field) {
				agg := statement.Aggregate{Func: field}
				tokener.Pop()

				lparen, err := tokener.Peek()
				if err != nil || lparen != "(" {
					return nil, ErrInvalidStat
				}
				tokener.Pop()

				arg, err := tokener.Peek()
				if err != nil {
					return nil, err
				}
				if arg == "*" && field == "count" {
					agg.Field = "*"
					tokener.Pop()
				} else if isName(arg) {
					agg.Field = arg
					tokener.Pop()
				} else {
					return nil, ErrInvalidStat
				}

				rparen, err := tokener.Peek()
				if err != nil || rparen != ")" {
					return nil, ErrInvalidStat
				}
				tokener.Pop()

				read.Aggregates = append(read.Aggregates, agg)
			} else if isName(field) {
				read.Fields = append(read.Fields, field)
				tokener.Pop()
			} else {
				return nil, ErrInvalidStat
			}

			comma, err := tokener.Peek()
			if err != nil {
				return nil, err
			}
			if comma == "," {
				tokener.Pop()
			} else {
				break
			}
		}
	}

	from, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if from != "from" {
		return nil, ErrInvalidStat
	}
	tokener.Pop()

	tableName, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if isName(tableName) == false {
		return nil, ErrInvalidStat
	}
	read.TableName = tableName
	tokener.Pop()

	tmp, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if tmp == "" {
		return read, nil
	}

	if tmp == "where" {
		where, err := parseWhere(tokener)
		if err != nil {
			return nil, err
		}
		read.Where = where
	}

	// ORDER BY
	tmp, err = tokener.Peek()
	if err != nil {
		return nil, err
	}
	if tmp == "order" {
		tokener.Pop()
		by, err := tokener.Peek()
		if err != nil || by != "by" {
			return nil, ErrInvalidStat
		}
		tokener.Pop()

		field, err := tokener.Peek()
		if err != nil || isName(field) == false {
			return nil, ErrInvalidStat
		}
		read.OrderBy = field
		tokener.Pop()

		dir, err := tokener.Peek()
		if err == nil && dir != "" {
			if dir == "desc" {
				read.OrderDesc = true
				tokener.Pop()
			} else if dir == "asc" {
				tokener.Pop()
			}
		}
	}

	// LIMIT
	tmp, err = tokener.Peek()
	if err != nil {
		return nil, err
	}
	if tmp == "limit" {
		tokener.Pop()
		limitStr, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		limitVal, err := utilsStrToInt(limitStr)
		if err != nil {
			return nil, ErrInvalidStat
		}
		read.Limit = limitVal
		tokener.Pop()

		off, err := tokener.Peek()
		if err == nil && off == "offset" {
			tokener.Pop()
			offsetStr, err := tokener.Peek()
			if err != nil {
				return nil, err
			}
			offsetVal, err := utilsStrToInt(offsetStr)
			if err != nil {
				return nil, ErrInvalidStat
			}
			read.Offset = offsetVal
			tokener.Pop()
		}
	}

	return read, nil
}

func parseWhere(tokener *tokener) (*statement.Where, error) {
	where := new(statement.Where)

	where0, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if where0 != "where" {
		return nil, ErrInvalidStat
	}
	tokener.Pop()

	sexp1, err := parseSingleExpr(tokener)
	if err != nil {
		return nil, err
	}
	where.SingleExp1 = sexp1

	logicOp, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if logicOp == "" || isLogicOp(logicOp) == false {
		where.LogicOp = ""
		return where, nil
	}
	where.LogicOp = logicOp
	tokener.Pop()

	sexp2, err := parseSingleExpr(tokener)
	if err != nil {
		return nil, err
	}
	where.SingleExp2 = sexp2

	eof, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if eof != "" {
		return nil, ErrInvalidStat
	}

	return where, nil
}

func parseSingleExpr(tokener *tokener) (*statement.SingleExp, error) {
	singleExp := new(statement.SingleExp)
	field, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if isName(field) == false {
		return nil, ErrInvalidStat
	}
	singleExp.Field = field
	tokener.Pop()

	op, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	// Handle multi-char operators: <= and >=
	tokener.Pop()
	// Handle multi-char operators: <= >= !=
	if op == "<" || op == ">" || op == "!" {
		next, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		if next == "=" {
			op = op + "="
			tokener.Pop()
		}
	}
	if isCmpOp(op) == false {
		return nil, ErrInvalidStat
	}
	singleExp.CmpOp = op

	value, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	singleExp.Value = value
	tokener.Pop()

	return singleExp, nil
}

func parseDrop(tokener *tokener) (*statement.Drop, error) {
	table, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if table != "table" {
		return nil, ErrInvalidStat
	}

	tokener.Pop()
	tableName, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if isName(tableName) == false {
		return nil, ErrInvalidStat
	}

	tokener.Pop()
	eof, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if eof != "" {
		return nil, ErrInvalidStat
	}

	drop := new(statement.Drop)
	drop.TableName = tableName
	return drop, nil
}

func parseCreate(tokener *tokener) (*statement.Create, error) {
	table, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if table != "table" {
		return nil, ErrInvalidStat
	}

	create := new(statement.Create)
	tokener.Pop()
	name, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if isName(name) == false {
		return nil, ErrInvalidStat
	}
	create.TableName = name

	for {
		tokener.Pop()
		field, err := tokener.Peek()

		if err != nil {
			return nil, err
		}

		if field == "(" {
			break
		}
		if isName(field) == false {
			return nil, ErrInvalidStat
		}

		tokener.Pop()
		ftype, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		if isType(ftype) == false {
			return nil, ErrInvalidStat
		}

		create.FieldName = append(create.FieldName, field)
		create.FieldType = append(create.FieldType, ftype)

		tokener.Pop()
		next, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		if next == "," {
		} else if next == "" {
			// No index clause — the first field will be auto-indexed.
			eof, err := tokener.Peek()
			if err != nil {
				return nil, err
			}
			if eof != "" {
				return nil, ErrInvalidStat
			}
			return create, nil
		} else if next == "(" {
			break
		} else {
			return nil, ErrInvalidStat
		}
	}

	tokener.Pop()
	index, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if index != "index" {
		return nil, ErrInvalidStat
	}
	for {
		tokener.Pop()
		field, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		if field == ")" {
			break
		} else if isName(field) == false {
			return nil, ErrInvalidStat
		} else {
			create.Index = append(create.Index, field)
		}
	}
	tokener.Pop()
	eof, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if eof != "" {
		return nil, ErrInvalidStat
	}
	return create, nil
}

func isLogicOp(op string) bool {
	return op == "and" || op == "or"
}

func isAggFunc(name string) bool {
	return name == "count" || name == "sum" || name == "avg" || name == "min" || name == "max"
}

func isType(tp string) bool {
	return tp == "uint32" || tp == "uint64" || tp == "string"
}

func isName(name string) bool {
	return !(len(name) == 1 && isAlphaBeta(name[0]) == false)
}

func isCmpOp(op string) bool {
	return op == "=" || op == ">" || op == "<" || op == "!=" || op == "<=" || op == ">="
}

func parseBegin(tokener *tokener) (*statement.Begin, error) {
	isolation, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	begin := new(statement.Begin)
	if isolation == "" {
		return begin, nil
	}
	if isolation != "isolation" {
		return nil, ErrInvalidStat
	}

	tokener.Pop()
	level, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if level != "level" {
		return nil, ErrInvalidStat
	}

	tokener.Pop()
	tmp1, err := tokener.Peek()
	if err != nil {
		return nil, err
	}

	if tmp1 == "read" {
		tokener.Pop()
		tmp2, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		if tmp2 == "committed" {
			tokener.Pop()
			eof, err := tokener.Peek()
			if err != nil {
				return nil, err
			}
			if eof != "" {
				return nil, ErrInvalidStat
			}

			return begin, nil
		} else {
			return nil, ErrInvalidStat
		}
	} else if tmp1 == "repeatable" {
		tokener.Pop()
		tmp2, err := tokener.Peek()
		if err != nil {
			return nil, err
		}
		if tmp2 == "read" {
			begin.IsRepeatableRead = true
			tokener.Pop()
			eof, err := tokener.Peek()
			if err != nil {
				return nil, err
			}
			if eof != "" {
				return nil, ErrInvalidStat
			}
			return begin, nil
		} else {
			return nil, ErrInvalidStat
		}
	} else {
		return nil, ErrInvalidStat
	}
}

func parseCommit(tokener *tokener) (*statement.Commit, error) {
	tmp, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if tmp == "" {
		return new(statement.Commit), nil
	} else {
		return nil, ErrInvalidStat
	}
}

func parseAbort(tokener *tokener) (*statement.Abort, error) {
	tmp, err := tokener.Peek()
	if err != nil {
		return nil, err
	}
	if tmp == "" {
		return new(statement.Abort), nil
	} else {
		return nil, ErrInvalidStat
	}
}


func utilsStrToInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, ErrInvalidStat
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
