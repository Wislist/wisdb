/*
	visibility.go 实现了sm内部的visibility结构，该结构内保存了sm中事务的可见性信息

*/

package sm

import (
	"mydb/src/main/backend/tm"
)

func IsVersionSkip(tm tm.TransactionManager, t *transaction, e *entry) bool {
	xmax := e.XMAX()
	if t.Level == 0 {
		return false
	} else {
		return tm.IsCommitted(xmax) && (xmax > t.XID || t.InSnapShot(xmax))
	}

}

// IsVisible 测试e是否对t可见
func IsVisible(tm tm.TransactionManager, t *transaction, e *entry) bool {
	if t.Level == 0 {
		return readCommitted(tm, t, e)
	}
	return repeatableRead(tm, t, e)
}

// 读已提交：只要XMIN已提交，且XMAX未提交或为0，就可见
func readCommitted(tm tm.TransactionManager, t *transaction, e *entry) bool {
	xid := t.XID
	xmin := e.XMIN()
	xmax := e.XMAX()

	if xmin == xid && xmax == 0 {
		return true			// 自己创建且未删除
	}

	isCommitted := tm.IsCommitted(xmin)
	if isCommitted {
		if xmax == 0 {
			return true
		}
		if xmax != xid {
			isCommitted = tm.IsCommitted(xmax)
			if isCommitted == false{
				return true
			}
		}
	}
	return false
}
// 可串行化：基于事务开始时的快照判断
func repeatableRead(tm tm.TransactionManager, t *transaction, e *entry) bool {
	xid := t.XID
	xmin := e.XMIN()
	xmax := e.XMAX()

	if xmin == xid && xmax == 0 {
		return true
	}

	isCommitted := tm.IsCommitted(xmin)
	if isCommitted && xmin < xid && t.InSnapShot(xmin) == false {
		if xmax == 0 {
			return true
		}
		if xmax != xid {
			isCommitted = tm.IsCommitted(xmax)
			if isCommitted == false || xmax > xid || t.InSnapShot(xmax){
				return true
			}
		}
	}
	return false
}