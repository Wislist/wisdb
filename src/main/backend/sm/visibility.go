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
	} else {
		return repeatableRead(tm, t, e)
	}
	return false
}

func readCommitted(tm tm.TransactionManager, t *transaction, e *entry) bool {
	xid := t.XID
	xmin := e.XMIN()
	xmax := e.XMAX()

	if xmin == xid && xmax == 0 {
		return true
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