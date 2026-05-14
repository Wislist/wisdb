package cacher

import (
	"errors"
	"mydb/src/main/backend/utils"
	"sync"
)

var (
	ErrCacheFull = errors.New("Cache is full.")
)

type Cacher interface {
	Get(uid utils.UUID) (interface{}, error)
	Release(uid utils.UUID)
	Close()
}

type Options struct {
	//当uid不再缓存中时 则调用该函数取得对应的资源
	//该函数必须要是并发安全的
	Get func(uid utils.UUID) (interface{}, error)

	//释放资源
	Release func(underlying interface{})

	//允许的最大资源数
	MaxHandles uint32
}

func NewCacher(options *Options) *cacher {
	return &cacher{
		options: options,
		cache:   make(map[utils.UUID]interface{}),
		getting: make(map[utils.UUID]chan struct{}),
		refs:    make(map[utils.UUID]uint32),
	}
}

type cacher struct {
	options *Options

	cache   map[utils.UUID]interface{}
	refs    map[utils.UUID]uint32
	getting map[utils.UUID]chan struct{} // 正在加载中的资源，等待者订阅此 channel
	count   uint32
	lock    sync.Mutex
}

func (c *cacher) Get(uid utils.UUID) (interface{}, error) {
	for {
		c.lock.Lock()

		if ch, ok := c.getting[uid]; ok {
			// 有其他 goroutine 正在加载，订阅其完成通知后等待
			c.lock.Unlock()
			<-ch
			continue
		}

		if _, ok := c.cache[uid]; ok {
			h := c.cache[uid]
			c.refs[uid]++
			c.lock.Unlock()
			return h, nil
		}

		if c.options.MaxHandles > 0 && c.count == c.options.MaxHandles {
			c.lock.Unlock()
			return nil, ErrCacheFull
		}

		// 占位：创建 channel，其他等待者会阻塞在 <-ch 上
		ch := make(chan struct{})
		c.getting[uid] = ch
		c.count++
		c.lock.Unlock()
		break
	}

	// 在锁外加载资源，允许其他 Get 并行执行
	underlying, err := c.options.Get(uid)

	c.lock.Lock()
	ch := c.getting[uid]
	delete(c.getting, uid)
	if err != nil {
		c.count--
		c.lock.Unlock()
		close(ch) // 通知所有等待者重新竞争
		return nil, err
	}
	c.cache[uid] = underlying
	c.refs[uid] = 1
	c.lock.Unlock()
	close(ch) // 通知所有等待者资源已就绪
	return underlying, nil
}

func (c *cacher) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for uid, h := range c.cache {
		c.options.Release(h)
		delete(c.refs, uid)
		delete(c.cache, uid)
	}
}

func (c *cacher) Release(uid utils.UUID) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.refs[uid]--
	if c.refs[uid] == 0 {
		underlying := c.cache[uid]
		/*
			Release 不能异步处理：若异步，新线程可能在 Release 完成前 Get 到旧资源
		*/
		c.options.Release(underlying)
		delete(c.refs, uid)
		delete(c.cache, uid)
		c.count--
	}
}