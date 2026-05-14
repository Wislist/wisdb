/**
 * locktable 维护了一个有向图，每次添加边的时候，就会进行死锁检测
 */
package locktable

import (
	"container/list"
	"mydb/src/main/backend/utils"
	"sync"
)



 type LockTable interface {
	//Add 向有锁表中加入一条xid到uid的变，如果返回flase，说明有死锁
	Add(xid, uid utils.UUID) (bool, chan struct{})

	// Remove 从有锁表中移除所有的xid
	Remove(xid utils.UUID)
 
 }

 type lockTable struct {
	x2u    map[utils.UUID]*list.List		// xid已经获得的资源uid
	u2x    map[utils.UUID]utils.UUID		// uid被哪个xid获得
	wait   map[utils.UUID]*list.List		// 表示哪些xid在等待这个uid,uwait和x2u应该是对偶关系
	waitCh map[utils.UUID]chan struct{}		// 等待的xid
	xwaitu map[utils.UUID]utils.UUID		// xid等待哪个uid
	lock sync.Mutex
 }

func NewLockTable() *lockTable {
	return &lockTable{
		x2u:    make(map[utils.UUID]*list.List),
		u2x:    make(map[utils.UUID]utils.UUID),
		wait:   make(map[utils.UUID]*list.List),
		waitCh: make(map[utils.UUID]chan struct{}),
		xwaitu: make(map[utils.UUID]utils.UUID),
	}
}

func (lt *lockTable) Add(xid, uid utils.UUID) (bool, chan struct{}) {
	lt.lock.Lock()
	defer lt.lock.Unlock()

	if isInList(lt.x2u, xid, uid) == true {
		ch := make(chan struct{})
		go func() {
			ch <- struct{}{}
		}()
		return true, ch
	}

	if _, ok := lt.u2x[uid]; ok == false {
		lt.u2x[uid] = xid
		putIntoList(lt.x2u, xid, uid)
		ch := make(chan struct{})
		go func ()  {
			ch <- struct{}{}
		}()
		return true, ch
	}

	// 尝试将xid->uid的等待边加入到图中，然后判断是否会造成死锁
	lt.xwaitu[xid] = uid
	putIntoList(lt.wait, uid, xid)
	if lt.hasDeadLock() == true {
		delete(lt.xwaitu, xid)
		removeFromList(lt.wait, uid, xid)
		return false, nil
	}
	// 如果不会造成死锁，则等待回应
	ch := make(chan struct{})
	lt.waitCh[xid] = ch
	return true, ch
}

func (lt *lockTable) dfs(xid utils.UUID, xidStamp map[utils.UUID]int, stamp int) bool {
	stp, ok := xidStamp[xid]
	if ok && stp == stamp {
		return true //有环
	}
	if ok && stp < stamp {
		return false //该节点之前已经被遍历过了且无环
	}
	xidStamp[xid] = stamp

	uid, ok := lt.xwaitu[xid]
	if ok == false {
		return false
	}
	xid, ok = lt.u2x[uid]
	utils.Assert(ok)
	return lt.dfs(xid, xidStamp, stamp)
}

func (lt *lockTable) hasDeadLock() bool {
	xidStamp := make(map[utils.UUID]int)
	stamp := 1
	for xid := range lt.x2u {
		if xidStamp[xid] > 0 {
			continue
		}
		stamp++
		if lt.dfs(xid, xidStamp, stamp) == true {
			return true
		}
	}
	return false
}

// selectNewXID 为uid从等待队列中，选择下一个uid来占用它
func (lt *lockTable) selectNewXID(uid utils.UUID) {
	delete(lt.u2x, uid) // 先将原来的事务删除掉
	l := lt.wait[uid]
	if l == nil {
		return // 没有等待的xid
	}
	utils.Assert(l.Len() > 0)

	for l.Len() > 0 {
		e := l.Front()
		v := l.Remove(e)
		xid := v.(utils.UUID)
		if _, ok := lt.waitCh[xid]; ok == false {
			continue
		} else {
			lt.u2x[uid] = xid        //将该uid指向xid
			ch := lt.waitCh[xid]     //将xid进行回应
			delete(lt.waitCh, xid)   //删除该xid的等待通道
			delete(lt.xwaitu, xid)   // 删除xid对uid的等待关系
			ch <- struct{}{}         // 回应
			break
		}
	}

	if l.Len() == 0 {
		delete(lt.wait, uid)
	}
}


func (lt *lockTable) Remove(xid utils.UUID) {
	lt.lock.Lock()
	defer lt.lock.Unlock()

	l := lt.x2u[xid]
	if l != nil {
		for l.Len() > 0 {
			e := l.Front()
			v := l.Remove(e)
			uid := v.(utils.UUID)
			lt.selectNewXID(uid)
		}
	}

	delete(lt.xwaitu, xid)
	delete(lt.x2u, xid)
	delete(lt.waitCh, xid)
}

func isInList(listMap map[utils.UUID]*list.List, uid0, uid1 utils.UUID) bool {
	if _, ok := listMap[uid0] ; ok == false {
		return false
	}
	l := listMap[uid0]
	e := l.Front()
	for e != nil {
		uid := e.Value.(utils.UUID)
		if uid == uid1 {
			return true
		}
		e = e.Next()
	}
	return false
}

func putIntoList(listMap map[utils.UUID]*list.List, uid0, uid1 utils.UUID)  {
	if _, ok := listMap[uid0] ; ok == false {
		listMap[uid0] = new(list.List)
	}
	listMap[uid0].PushBack(uid1)
}

func removeFromList(listMap map[utils.UUID]*list.List, uid0, uid1 utils.UUID) {
	l := listMap[uid0]
	e := l.Front()
	for e != nil {
		uid := e.Value.(utils.UUID)
		if uid == uid1 {
			l.Remove(e)
			break
		}
	}
	if l.Len() == 0 {
		delete(listMap, uid0)
	}
}



