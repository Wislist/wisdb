// conn.go — 单条数据库连接
package wisdb

import (
	"fmt"
	"net"
	"time"
)

// IsolationLevel 事务隔离级别
type IsolationLevel int

const (
	ReadCommitted  IsolationLevel = 0
	RepeatableRead IsolationLevel = 1
)

// Conn 表示一条到 WisDB 的连接。
// 非并发安全——同一时刻只能有一个 goroutine 使用。
// 从连接池获取后使用，用完归还池（pool.Put）。
type Conn struct {
	wc      *wireConn
	inTx    bool
	addr    string
	timeout time.Duration
}

// Dial 建立一条新连接（不使用连接池时直接调用）
func Dial(addr string) (*Conn, error) {
	return DialTimeout(addr, 10*time.Second)
}

// DialTimeout 建立连接，带超时
func DialTimeout(addr string, timeout time.Duration) (*Conn, error) {
	nc, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("wisdb: dial %s: %w", addr, err)
	}
	return &Conn{
		wc:      newWireConn(nc),
		addr:    addr,
		timeout: timeout,
	}, nil
}

// Execute 执行一条 SQL，返回原始结果字节。
// 如果当前在事务中，SQL 在事务上下文内执行。
func (c *Conn) Execute(sql string) ([]byte, error) {
	if err := c.wc.send([]byte(sql)); err != nil {
		return nil, fmt.Errorf("wisdb: send: %w", err)
	}
	data, err := c.wc.recv()
	if err != nil {
		return nil, fmt.Errorf("wisdb: %w", err)
	}
	return data, nil
}

// Begin 开启一个事务，返回 Tx 对象。
// 同一连接同一时刻只能有一个活跃事务。
func (c *Conn) Begin(level IsolationLevel) (*Tx, error) {
	if c.inTx {
		return nil, fmt.Errorf("wisdb: already in a transaction")
	}

	var sql string
	switch level {
	case RepeatableRead:
		sql = "begin isolation level repeatable read"
	default:
		sql = "begin isolation level read committed"
	}

	if _, err := c.Execute(sql); err != nil {
		return nil, err
	}
	c.inTx = true
	return &Tx{conn: c}, nil
}

// Close 关闭连接（不归还连接池）
func (c *Conn) Close() error {
	return c.wc.close()
}

// IsAlive 检查连接是否仍然可用（发送一个空 ping 帧）
// 目前 WisDB 没有 ping 命令，通过检查底层连接状态判断
func (c *Conn) IsAlive() bool {
	if c.wc == nil || c.wc.conn == nil {
		return false
	}
	// 设置极短的读超时，尝试读 0 字节来探测连接状态
	c.wc.conn.SetReadDeadline(time.Now().Add(time.Millisecond))
	buf := make([]byte, 1)
	_, err := c.wc.conn.Read(buf)
	c.wc.conn.SetReadDeadline(time.Time{}) // 清除 deadline
	// 超时错误说明连接存活（没有数据可读，但连接正常）
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return false
}
