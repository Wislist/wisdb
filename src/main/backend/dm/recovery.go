/*
	对数据库进行恢复
*/

package dm

import (
	"mydb/src/main/backend/dm/logger"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
)

const (
	_LOG_TYPE_INSERT = 0
	_LOG_TYPE_UPDATE = 1

	_REDO = 0
	_UNDO = 1
)

func Recovery(tm tm.TransactionManager,lg logger.Logger, pc pcacher.Pcacher) {
	utils.Info("Recovering...")
	defer utils.Info("Recovery Over")

	lg.Rewind()
	var maxPgno pcacher.Pgno
	for {
		log,ok := lg.Next()
		if ok == false {
			break
		}
		var pgno pcacher.Pgno
		if isInsertLog(log){
			_, pgno, _, _ = parseInsertLog(log)
		} else {
			_, pgno, _, _, _ = parseUpdateLog(log)
		}
		if pgno > maxPgno {
			maxPgno = pgno
		}
	}
	if maxPgno == 0 {
		maxPgno = 1
	}
	pc.TruncateByPgno(maxPgno)
	utils.Info("Truncate to",maxPgno,"pages.")

	redoTransactions(tm,lg,pc)
	utils.Info("Redo Transactions Over.")

	undoTransactions(tm,lg,pc)
	utils.Info("Undo Transactions Over.")
}

func redoTransactions(tm tm.TransactionManager,lg logger.Logger, pc pcacher.Pcacher){
	lg.Rewind()
	for {
		log,ok := lg.Next()
		if ok == false {
			break
		}
		if isInsertLog(log) {
			xid,_,_,_ := parseInsertLog(log)
			if tm.IsActive(xid) == false {
				doInsertLog(pc,log,_REDO)
			}
		} else {
			xid,_,_,_ := parseInsertLog(log)
			if tm.IsActive(xid) == false {
				doUpdateLog(pc,log,_REDO)
			}
		}
		
	}
}

func undoTransactions(tm0 tm.TransactionManager,lg logger.Logger, pc pcacher.Pcacher){
	logCache := make(map[tm.XID][][]byte)
	lg.Rewind()
	for {
		log , ok := lg.Next()
		if ok == false {
			break
		}
		if isInsertLog(log){
			xid,_,_,_ := parseInsertLog(log)
			if tm0.IsActive(xid) == true {
				logCache[xid] = append(logCache[xid], log)
			}
		} else {
			xid, _, _, _, _ := parseUpdateLog(log)
			if tm0.IsActive(xid) == true {
				logCache[xid] = append(logCache[xid], log)
			}
		}
	}

	for xid,logs := range logCache {
		for i := len(logs)-1; i >= 0; i--	{
			log := logs[i]
			if isInsertLog(log) {
				doInsertLog(pc,log,_UNDO)
			} else {
				doUpdateLog(pc,log,_UNDO)
			}
		}
		tm0.Abort(xid)
	}
}

func isInsertLog(log []byte) bool {
	return log[0] == _LOG_TYPE_INSERT
}

/*
	[Log Type] [XID] [UUID] [OldRaw] [NewRaw]
	表示XID将UUID这个dataitem从OldRaw更新为了NewRaw.
*/


func UpdateLog(xid tm.XID, di *dataitem) []byte {
	log := make([]byte,1 + tm.LEN_XID + utils.LEN_UUID + len(di.raw) * 2)
	pos := 0
	log[pos] = _LOG_TYPE_UPDATE
	pos++
	tm.PutXID(log[pos:],xid)
	pos += tm.LEN_XID
	utils.PutUUID(log[pos:],di.uid)
	pos += utils.LEN_UUID
	copy(log[pos:],di.oldraw)
	pos += len(di.oldraw)
	copy(log[pos:],di.raw)
	return log
}

func parseUpdateLog(log []byte) (tm.XID,pcacher.Pgno,Offset,[]byte,[]byte) {
	pos := 1
	xid := tm.ParseXID(log[pos:])
	pos += tm.LEN_XID
	uuid := utils.ParseUUID(log[pos:])
	pgno, offset := UUID2Address(uuid)
	pos += utils.LEN_UUID
	length := (len(log) - pos) / 2
	oldraw := log[pos : pos+length]
	newraw := log[pos + length : pos + length * 2]
	return xid,pgno,offset,oldraw,newraw
}

func doUpdateLog(pc pcacher.Pcacher, log []byte, flag int) {
	var pgno pcacher.Pgno
	var offset Offset
	var raw []byte
	if flag == _REDO {
		_, pgno, offset, _, raw = parseUpdateLog(log)
	} else {
		_,pgno,offset,raw,_ = parseUpdateLog(log)
	}
	pg, err := pc.GetPage(pgno)
	if err != nil {
		// 因为在恢复的时候是单线程的，所以err不可能为ErrCacheFull等并发错误
		//如果此时错误，那么一定是不能解决的问题，直接panic
		panic(err)
	}
	defer pg.Release()
	PXRecoverUpdate(pg, offset, raw)
}

func InsertLog(xid tm.XID, pg pcacher.Page, raw []byte) []byte {
	log := make([]byte, 1+tm.LEN_XID+pcacher.LEN_PGNO+2+len(raw))
	pos := 0
	log[pos] = _LOG_TYPE_INSERT
	pos++
	tm.PutXID(log[pos:], xid)
	pos += tm.LEN_XID
	pcacher.PutPgno(log[pos:], pg.Pgno())
	pos += pcacher.LEN_PGNO
	PutOffset(log[pos:], PxFSO(pg))
	pos += LEN_OFFSET
	copy(log[pos:], raw)
	return log
}

func parseInsertLog(log []byte) (tm.XID, pcacher.Pgno, Offset, []byte)  {
	pos := 1
	xid := tm.ParseXID(log[pos:])
	pos += tm.LEN_XID
	pgno := pcacher.ParsePgno(log[pos:])
	pos += pcacher.LEN_PGNO
	offset := ParseOffset(log[pos:])
	pos += LEN_OFFSET
	return xid, pgno, offset, log[pos:]
}


/*
	redoInsertLog 对insertLog进行redo
	redo的方式为将原数据重新插入到对应page的位置，然后将page的FSO设置为较大的那一个

	BUG：如果之前数据库刚好崩坏在对page的FSO做修改时，那么坏的FSO可能会非常大，导致到最后
	恢复完成时，FSO保留那个错误的最大值
*/
func doInsertLog(pc pcacher.Pcacher, log []byte, flag int) {
	_, pgno, offset, raw := parseInsertLog(log)
	pg, err := pc.GetPage(pgno)
	if err != nil {
		panic(err)
	}
	defer pg.Release()
	if flag == _UNDO {
		InValidRawDataitem(raw)
	}
	PXRecoverInsert(pg, offset, raw)
}