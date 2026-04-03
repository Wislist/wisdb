package pindex

import (
	"container/list"
	"mydb/src/main/backend/dm/pcacher"
	"sync"
)



const (
	_NO_INTERVAL = 40
	_THRESHOLD = pcacher.PAGE_SIZE / _NO_INTERVAL
)

type Pindex interface {
	/*
		Add将该键值对添加到Pindex中
	*/
	Add(pgno pcacher.Pgno, freeSpace int)

	/*
		Select为spaceSize选择适当的Pgno,并暂时将Pgno从Pindex中移除
	*/
	Select(spaces int) (pcacher.Pgno , int , bool)
}

type pindex struct {
	lock sync.Mutex
	lists [_NO_INTERVAL + 1]list.List
}

type pair struct {
	pgno 		pcacher.Pgno
	freeSpace 	int
}

func NewPindex() *pindex {
	return &pindex{
		lists: [_NO_INTERVAL + 1]list.List{},
	}
}

func (pi *pindex) Add(pgno pcacher.Pgno, freeSpace int) {
	pi.lock.Lock()
	defer pi.lock.Unlock()

	no := freeSpace / _THRESHOLD
	pi.lists[no].PushBack(&pair{pgno,freeSpace})
}

func (pi *pindex) Select(spaceSize int) (pcacher.Pgno,int,bool){
	pi.lock.Lock()
	defer pi.lock.Unlock()

	no := spaceSize / _THRESHOLD
	if no < _NO_INTERVAL {
		no ++
	}
	for no <= _NO_INTERVAL {
		if pi.lists[no].Len() == 0 {
			no++
			continue
		}
		e := pi.lists[no].Front()
		v := pi.lists[no].Remove(e)
		pr := v.(*pair)
		return pr.pgno,pr.freeSpace,true
	}
	return 0,0,false
}