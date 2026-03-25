package cacher

import (
	"errors"
	"mydb/src/main/backend/utils"
	"sync"
	"time"
)

/**
 * cacher.go主要实现一个带reference的cache
 *
 *
 */

var (
	ErrCacheFull = errors.New("Cache is full.")

	_TIME_WAIT = time.Millisecond
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
	Release func(uid utils.UUID, obj interface{})

	//允许的最大资源数
	MaxHandles uint32
}

func NewCacher(options *Options) *cacher {
	return &cacher{
		options: options,
		cache:   make(map[utils.UUID]interface{}),
		getting: make(map[utils.UUID]bool),
		refs:    make(map[utils.UUID]uint32),
	}
}

type cacher struct {
	options *Options

	cache   map[utils.UUID]interface{}
	refs    map[utils.UUID]uint32
	getting map[utils.UUID]bool
	count   uint32
	lock    sync.Mutex
	
}

func (c *cacher) Get(uid utils.UUID) (interface{}, error) {
	for {
		c.lock.Lock()

		if _, ok := c.getting[uid]; ok {
			c.lock.Unlock()
			time.Sleep(_TIME_WAIT)
			continue
		}

		if _, ok := c.cache[uid]; ok {
			//如果资源在缓存中，则直接返回
			h := c.cache[uid]
			c.refs[uid]++
			c.lock.Unlock()
			return h, nil
		}
		//否则 则尝试获取该资源
		if c.options.MaxHandles > 0 && c.count == c.options.MaxHandles {
			//资源已满
			c.lock.Unlock()
			return nil, ErrCacheFull
		} else {
			c.count++
			c.getting[uid] = true
		}
		c.lock.Unlock()
		break
	}

	//调用options.Get时是无锁的，因此可以和其他Get并行进行
	//这要求options.Get必须是并发安全的
	underlying, err := c.options.Get(uid)
	if err != nil {
		c.lock.Lock()
		c.count--
		delete(c.getting, uid)
		c.lock.Unlock()
		return nil, err
	}

	c.lock.Lock()
	delete(c.getting,uid)
	c.cache[uid] = underlying
	c.refs[uid] = 1
	c.lock.Unlock()

	return underlying, nil
}

func (c *cacher) Release(uid utils.UUID) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.refs[uid]--
	if c.refs[uid] == 0 {
		underlying := c.cache[uid]
		/*
			这里Rlease是不能被异步处理的
			如果Release被异步处理，那么有可能在Release完成之前，就有新的线程Get这个资源
			那么新线程得到的将会是未被更新的新资源
		*/
		c.options.Release(uid, underlying)
		delete(c.refs,uid)
		delete(c.cache, uid)
		c.count--
	}

	
}