/*
	B+ 树 由node.go和tree.go实现
	该B+树基于DM，且支持并发读写，无死锁

	该代码详细参考了boltDB的B+树实现
*/

package im

import (
	"mydb/src/main/backend/dm"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
	"sync"
)

type BPlusTree interface {
	Insert(key, uuid utils.UUID) error
	Search(key utils.UUID) ([]utils.UUID, error)
	SearchRange(leftKey, rightKey utils.UUID) ([]utils.UUID, error)
}

/*
	每棵B+树都有一个bootUUID, 可通过它向DM读取该树的boot.
	B+树boot里面存储了B+树根节点的地址.

	PS: 因为B+树在算法执行过程中, 根节点可能会发生改变, 所以不能直接用根节点的地址当boot,
	而需要一个固定的boot, 用来指向它的根节点.

	PS: 目前B+树支持的最大键值为INF-1


*/

type bPlusTree struct {
	bootUUID     utils.UUID
	bootDataitem dm.Dataitem
	bootLock     sync.Mutex

	DM dm.DataManager
}

// CreateBPlusTree 创建一颗B+树，返回其bootUUID
func Create(dm dm.DataManager) (utils.UUID, error) {
	rawBoot := newNilRootRaw()
	rootUUID, err := dm.Insert(tm.SUPER_XID, rawBoot)
	if err != nil {
		return utils.NilUUID, err
	}
	bootUUID, err := dm.Insert(tm.SUPER_XID, utils.UUIDToRaw(rootUUID))
	if err != nil {
		return utils.NilUUID, err
	}
	return bootUUID, nil
}

// LoadBPlusTree 从DM加载一颗B+树，返回其bootUUID
func Load(bootUUID utils.UUID, dm dm.DataManager) (BPlusTree, error) {
	bootDataitem, ok, err := dm.Read(bootUUID)
	if err != nil {
		return nil, err
	}
	utils.Assert(ok == true)

	return &bPlusTree{
		bootUUID:     bootUUID,
		DM:           dm,
		bootDataitem: bootDataitem,
	}, nil
}

// rootUUID 通过bootUUID读取该树的根节点地址（调用方需持有 bootLock）
func (bt *bPlusTree) rootUUID_() utils.UUID {
	return utils.ParseUUID(bt.bootDataitem.Data())
}

// rootUUID 并发安全版本，供外部只读场景使用
func (bt *bPlusTree) rootUUID() utils.UUID {
	bt.bootLock.Lock()
	defer bt.bootLock.Unlock()
	return bt.rootUUID_()
}

// updaterootUUID_ 更新该树的根节点（调用方需持有 bootLock）
func (bt *bPlusTree) updaterootUUID_(left, right, rightKey utils.UUID) error {
	rootRaw := newRootRaw(left, right, rightKey)
	newRootUUID, err := bt.DM.Insert(tm.SUPER_XID, rootRaw)
	if err != nil {
		return err
	}

	bt.bootDataitem.Before()
	copy(bt.bootDataitem.Data(), utils.UUIDToRaw(newRootUUID))
	bt.bootDataitem.After(tm.SUPER_XID)
	return nil
}

// updaterootUUID 更新该树的根节点（并发安全版本）
func (bt *bPlusTree) updaterootUUID(left, right, rightKey utils.UUID) error {
	bt.bootLock.Lock()
	defer bt.bootLock.Unlock()
	return bt.updaterootUUID_(left, right, rightKey)
}

// searchLeaf 根据key，在nodeUUID代表节点的子树中搜索， 直到找到其对应的叶节点地址
func (bt *bPlusTree) searchLeaf(nodeUUID, key utils.UUID) (utils.UUID, error) {
	node, err := loadNode(bt, nodeUUID)
	if err != nil {
		return utils.NilUUID, err
	}

	isLeaf := node.IsLeaf()
	node.Release()

	if isLeaf {
		return nodeUUID, nil
	} else {
		next, err := bt.searchNext(nodeUUID, key)
		if err != nil {
			return utils.NilUUID, err
		}
		return bt.searchLeaf(next, key)
	}
}

// searchNext 从nodeUUID对应节点开始， 不断向右试探兄弟节点，找到对应key的next uid
func (bt *bPlusTree) searchNext(nodeUUID, key utils.UUID) (utils.UUID, error) {
	for {
		node, err := loadNode(bt, nodeUUID)
		if err != nil {
			return utils.NilUUID, err
		}
		next, siblingUUID := node.SearchNext(key)
		node.Release()
		if next != utils.NilUUID {
			return next, nil
		}
		nodeUUID = siblingUUID
	}	
}

func (bt *bPlusTree) Search(key utils.UUID) ([]utils.UUID, error) {
	return bt.SearchRange(key, key)
}

func (bt *bPlusTree) SearchRange(leftKey, rightKey utils.UUID) ([]utils.UUID, error) {
	rootUUID := bt.rootUUID()

	leafUUID, err := bt.searchLeaf(rootUUID, leftKey) // 找到左边界叶子节点
	if err != nil {
		return nil, err
	}

	var uuids []utils.UUID
	//不断从leaf向sibling迭代，将所有满足的uuid都加入
	for {
		leaf, err := loadNode(bt, leafUUID) // 收集当前叶子节点里满足的UUID
		if err != nil {
			return nil, err
		}
		tmp, siblingUUID := leaf.LeafSearchRange(leftKey, rightKey)
		leaf.Release()
		uuids = append(uuids, tmp...)

		if siblingUUID == utils.NilUUID {
			break
		} else {
			leafUUID = siblingUUID
		}
	}

	return uuids, nil
}

// Insert 插入一个uuid, key 键值对
func (bt *bPlusTree) Insert(key, uuid utils.UUID) error {
	bt.bootLock.Lock()
	rootUUID := bt.rootUUID_()
	bt.bootLock.Unlock()

	newNode, newKey, err := bt.insert(rootUUID, uuid, key)
	if err != nil {
		return err
	}

	if newNode != utils.NilUUID {
		// 持锁保护"读根节点 → 更新根节点"的原子性，防止并发分裂时重复更新
		bt.bootLock.Lock()
		// 重新读取根节点：在 insert 执行期间，其他 goroutine 可能已经更新了根节点
		// 只有当根节点仍是我们插入时的那个，才需要更新
		currentRoot := bt.rootUUID_()
		if currentRoot == rootUUID {
			err = bt.updaterootUUID_(rootUUID, newNode, newKey)
		}
		bt.bootLock.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// insert 将(uuid, key)插入到B+树中，如果有分裂，则将分裂产生的新节点也返回
func (bt *bPlusTree) insert(nodeUUID, uuid, key utils.UUID) (newNodeUUID, newNodeKey utils.UUID, err error) {
	var node *node
	node, err = loadNode(bt,nodeUUID)
	if err != nil {
		return
	}
	
	isLeaf := node.IsLeaf()
	node.Release()

	if isLeaf {
		newNodeUUID, newNodeKey, err = bt.insertAndSplit(nodeUUID, uuid, key)
		
	} else {
		var next utils.UUID
		next, err = bt.searchNext(nodeUUID, key)
		if err != nil {
			return
		}
		var newSonUUID utils.UUID
		var newSonKey utils.UUID
		newSonUUID, newSonKey, err = bt.insert(next, uuid, key)
		if err != nil {
			return
		}
	
		if newSonUUID != utils.NilUUID {
			newSonUUID, newSonKey, err = bt.insertAndSplit(nodeUUID, newSonUUID, newSonKey)
		}
	}
	return
}

// insertAndSplit 从node开始，不断的向右试探右兄弟节点，直到找到一个节点，能够插入进对应的值
func (bt *bPlusTree) insertAndSplit(nodeUUID, uuid, key utils.UUID) (utils.UUID, utils.UUID, error) {
	for {
		node, err := loadNode(bt, nodeUUID)
		if err != nil {
			return utils.NilUUID, utils.NilUUID, err
		}
		siblingSon, newNodeSon, newNodeKey, err := node.insertAndSplit(uuid, key)
		node.Release()

		if siblingSon != utils.NilUUID { //继续向sibling尝试
			nodeUUID = siblingSon
		} else {
			return newNodeSon, newNodeKey, err
		}
		
	}
}

func (bt *bPlusTree) Close() {
	bt.bootDataitem.Release()
}


