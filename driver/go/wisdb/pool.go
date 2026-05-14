// pool.go — 连接池
package wisdb

import (
	"fmt"
	"sync"
	"time"
)

// PoolOptions 连接池配置
type PoolOptions struct {
	MaxConns    int           // 最大连接数，默认 10
	MinConns    int           // 最小空闲连接数，默认 2
	DialTimeout time.Duration // 建连超时，默认 10s
	IdleTimeout time.Duration // 空闲连接超时，默认 5min（超时后关闭重建）
}

// DefaultPoolOptions 返回推荐的默认配置
func DefaultPoolOptions() PoolOptions {
	return PoolOptions{
		MaxConns:    10,
		MinConns:    2,
		DialTimeout: 10 * time.Second,
		IdleTimeout: 5 * time.Minute,
	}
}

type idleConn struct {
	conn     *Conn
	idleSince time.Time
}

// Pool 管理到 WisDB 的连接池。
// 并发安全，多个 goroutine 可同时调用 Get/Put。
type Pool struct {
	addr    string
	opts    PoolOptions
	mu      sync.Mutex
	idle    []idleConn
	active  int  // 当前已建立的连接总数（idle + in-use）
	closed  bool
}

// NewPool 创建连接池并预建 MinConns 条连接
func NewPool(addr string, opts PoolOptions) (*Pool, error) {
	if opts.MaxConns <= 0 {
		opts.MaxConns = 10
	}
	if opts.MinConns < 0 {
		opts.MinConns = 0
	}
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 10 * time.Second
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 5 * time.Minute
	}

	p := &Pool{addr: addr, opts: opts}

	// 预建最小连接数
	for i := 0; i < opts.MinConns; i++ {
		conn, err := DialTimeout(addr, opts.DialTimeout)
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("wisdb: pool init: %w", err)
		}
		p.idle = append(p.idle, idleConn{conn: conn, idleSince: time.Now()})
		p.active++
	}

	return p, nil
}

// Get 从池中获取一条连接。
// 如果池中有空闲连接则直接返回，否则新建（不超过 MaxConns）。
// 如果连接数已达上限，返回 ErrPoolExhausted。
func (p *Pool) Get() (*Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, fmt.Errorf("wisdb: pool is closed")
	}

	// 从空闲队列尾部取（LIFO，保持连接热度）
	for len(p.idle) > 0 {
		ic := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]

		// 检查空闲超时
		if p.opts.IdleTimeout > 0 && time.Since(ic.idleSince) > p.opts.IdleTimeout {
			ic.conn.Close()
			p.active--
			continue
		}
		return ic.conn, nil
	}

	// 没有空闲连接，尝试新建
	if p.active >= p.opts.MaxConns {
		return nil, fmt.Errorf("wisdb: pool exhausted (max=%d)", p.opts.MaxConns)
	}

	conn, err := DialTimeout(p.addr, p.opts.DialTimeout)
	if err != nil {
		return nil, err
	}
	p.active++
	return conn, nil
}

// Put 将连接归还连接池。
// 如果连接已损坏（inTx 状态异常），直接关闭而不归还。
func (p *Pool) Put(conn *Conn) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || conn.inTx {
		// 事务未结束的连接不能归还，直接关闭
		conn.Close()
		p.active--
		return
	}

	p.idle = append(p.idle, idleConn{conn: conn, idleSince: time.Now()})
}

// Close 关闭连接池，释放所有空闲连接
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true
	for _, ic := range p.idle {
		ic.conn.Close()
	}
	p.idle = nil
}

// Stats 返回连接池当前状态（用于监控）
type PoolStats struct {
	Active int // 已建立的连接总数
	Idle   int // 当前空闲连接数
	InUse  int // 正在使用的连接数
}

func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	idle := len(p.idle)
	return PoolStats{
		Active: p.active,
		Idle:   idle,
		InUse:  p.active - idle,
	}
}
