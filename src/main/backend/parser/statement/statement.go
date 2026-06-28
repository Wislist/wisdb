package statement

type Begin struct {
	IsRepeatableRead bool
}

type Commit struct{}
type Abort struct{}

type Drop struct {
	TableName string
}

type Show struct {
}

type Create struct {
	TableName string
	FieldName []string
	FieldType []string
	Index     []string
}

type Update struct {
	TableName string
	FieldName string
	Value     string
	Where     *Where
}

type Delete struct {
	TableName string
	Where     *Where
}

type Insert struct {
	TableName string
	Values    []string
}

// Aggregate represents an aggregate function call.
type Aggregate struct {
	Func  string // "count", "sum", "avg"
	Field string // field name or "*"
}

type Read struct {
	TableName  string
	Fields     []string
	Aggregates []Aggregate
	Where      *Where
	OrderBy    string
	OrderDesc  bool
	Limit      int
	Offset     int
}

type Where struct {
	SingleExp1 *SingleExp
	LogicOp    string
	SingleExp2 *SingleExp
}

type SingleExp struct {
	Field string
	CmpOp string
	Value string
}
