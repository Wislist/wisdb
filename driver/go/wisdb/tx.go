// tx.go — 事务对象
package wisdb

import "fmt"

// Tx 表示一个活跃的数据库事务。
// 通过 conn.Begin() 获取，使用完毕后必须调用 Commit 或 Rollback。
type Tx struct {
	conn   *Conn
	done   bool
}

// Execute 在事务内执行一条 SQL
func (tx *Tx) Execute(sql string) ([]byte, error) {
	if tx.done {
		return nil, fmt.Errorf("wisdb: transaction already closed")
	}
	return tx.conn.Execute(sql)
}

// Commit 提交事务
func (tx *Tx) Commit() error {
	if tx.done {
		return fmt.Errorf("wisdb: transaction already closed")
	}
	tx.done = true
	tx.conn.inTx = false
	_, err := tx.conn.Execute("commit")
	return err
}

// Rollback 回滚事务
func (tx *Tx) Rollback() error {
	if tx.done {
		return fmt.Errorf("wisdb: transaction already closed")
	}
	tx.done = true
	tx.conn.inTx = false
	_, err := tx.conn.Execute("abort")
	return err
}
