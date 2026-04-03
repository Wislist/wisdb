package pcacher

import "sync"

/*
	page.go 实现了关于Page的逻辑和接口

	1、Page更新协议：
		在对Page做任何更新之前，一定要调用Dirty()

	2、Page释放协议：
		在对Page做完所有更新后，一定要调用Release()


*/

type Page interface {
	Pgno() Pgno		//返回该Page的页号
	Data() []byte	//返回该page的内容，以共享的方式

	Dirty()	        //将该页设置为脏页
	Release()		//释放该页

	Lock()
	Unlock()
}

type page struct {
	pgno Pgno
	data []byte
	dirty bool
	lock sync.Mutex

	pc *pcacher
}

func NewPage(pgno Pgno , data []byte, pc *pcacher) *page {
	return &page{
		pgno: pgno,
		data: data,
		pc: pc,
	}
}

func (p *page) Pgno() Pgno {
	return p.pgno
}

func (p *page) Data() []byte {
	return p.data
}

func (p *page) Dirty() {
	p.dirty = true
}
func (p *page) Release() {
	p.pc.release(p)
}

func (p *page) Lock() {
	p.lock.Lock()
}

func (p *page) Unlock() {
	p.lock.Unlock()
}
