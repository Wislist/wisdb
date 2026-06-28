package cacher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mydb/src/main/backend/utils"
)

type resource struct {
	mu    sync.Mutex
	value int
	id    int64
}

func newTestCacher(getDelay time.Duration) (*cacher, *int64, *int64) {
	var createCount int64
	var releaseCount int64
	opts := &Options{
		Get: func(uid utils.UUID) (interface{}, error) {
			time.Sleep(getDelay)
			id := atomic.AddInt64(&createCount, 1)
			return &resource{id: id}, nil
		},
		Release: func(underlying interface{}) {
			atomic.AddInt64(&releaseCount, 1)
		},
		MaxHandles: 0,
	}
	return NewCacher(opts), &createCount, &releaseCount
}

func TestCacher_ConcurrentSingleUID(t *testing.T) {
	c, createCount, _ := newTestCacher(2 * time.Millisecond)

	uid := utils.UUID(1)
	const workers = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	hold, err := c.Get(uid)
	if err != nil {
		t.Fatalf("initial Get error: %v", err)
	}
	first := hold

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			obj, err := c.Get(uid)
			if err != nil {
				t.Errorf("Get error: %v", err)
				return
			}
			if obj != first {
				t.Errorf("expected same instance for concurrent Get")
				return
			}
			c.Release(uid)
		}()
	}

	wg.Wait()
	c.Release(uid)

	if atomic.LoadInt64(createCount) != 1 {
		t.Fatalf("underlying Get called %d times, expected 1", atomic.LoadInt64(createCount))
	}
}

//并发写入缓存
func TestCacher_ReadWrite(t *testing.T) {
	c, _, _ := newTestCacher(0)

	uid := utils.UUID(2)

	obj1, err := c.Get(uid)
	if err != nil {
		t.Errorf("Get error: %v", err)
				return
	}
	r1 := obj1.(*resource)

	r1.mu.Lock()
	r1.value = 42
	r1.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		obj2, err := c.Get(uid)
		if err != nil {
			t.Errorf("Get error: %v", err)
				return
		}
		r2 := obj2.(*resource)
		r2.mu.Lock()
		defer r2.mu.Unlock()
		if r2.value != 42 {
			t.Errorf("expected value 42, got %d", r2.value)
		}
		c.Release(uid)
	}()

	wg.Wait()
	c.Release(uid)
}

//并发删除缓存，然后重新创建
func TestCacher_DeleteAndRecreate(t *testing.T) {
	c, createCount, releaseCount := newTestCacher(0)

	uid := utils.UUID(3)

	obj1, err := c.Get(uid)
	if err != nil {
		t.Errorf("Get error: %v", err)
				return
	}

	c.Release(uid)

	if atomic.LoadInt64(releaseCount) != 1 {
		t.Fatalf("expected 1 release, got %d", atomic.LoadInt64(releaseCount))
	}

	obj2, err := c.Get(uid)
	if err != nil {
		t.Errorf("Get error: %v", err)
				return
	}

	if obj1 == obj2 {
		t.Fatalf("expected new instance after deletion")
	}
	if atomic.LoadInt64(createCount) < 2 {
		t.Fatalf("expected recreate, createCount=%d", atomic.LoadInt64(createCount))
	}
	c.Release(uid)
}
