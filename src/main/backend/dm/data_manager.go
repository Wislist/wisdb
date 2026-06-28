package dm

import (
	"errors"
	"mydb/src/main/backend/dm/logger"
	"mydb/src/main/backend/dm/pcacher"
	"mydb/src/main/backend/dm/pindex"
	"mydb/src/main/backend/tm"
	"mydb/src/main/backend/utils"
	"mydb/src/main/backend/utils/cacher"
)

var (
	ErrBusy         = errors.New("data manager is busy")
	ErrDataTooLarge = errors.New("data is too large")
)

type DataManager interface {
	Read(uid utils.UUID) (Dataitem, bool, error)
	Insert(xid tm.XID, data []byte) (utils.UUID, error)

	Close() error
}

type dataManager struct {
	tm tm.TransactionManager
	pc pcacher.Pcacher
	lg logger.Logger

	pidx pindex.Pindex
	dic  cacher.Cacher

	page1 pcacher.Page
}

func NewDataManager(pc pcacher.Pcacher, lg logger.Logger, tm tm.TransactionManager) *dataManager {
	pidx := pindex.NewPindex()
	dm := &dataManager{
		tm:   tm,
		pc:   pc,
		lg:   lg,
		pidx: pidx,
	}

	options := new(cacher.Options)
	options.MaxHandles = 0
	options.Get = dm.getForCacher
	options.Release = dm.releaseForCacher
	dm.dic = cacher.NewCacher(options)

	return dm
}

func (dm *dataManager) getForCacher(uid utils.UUID) (interface{}, error) {
	pgno, offset := UUID2Address(uid)
	pg, err := dm.pc.GetPage(pgno)
	if err != nil {
		return nil, err
	}
	return ParseDataitem(pg, offset, dm), nil
}

func (dm *dataManager) releaseForCacher(h interface{}) {
	di := h.(*dataitem)
	di.pg.Release()
}

func Open(path string, mem int64, tm tm.TransactionManager) (*dataManager, error) {
	pc, err := pcacher.Open(path, mem)
	if err != nil {
		return nil, err
	}
	lg, err := logger.Open(path)
	if err != nil {
		return nil, err
	}

	dm := NewDataManager(pc, lg, tm)
	ok, err := dm.loadAndCheckPage1()
	if err != nil {
		return nil, err
	}
	if !ok {
		Recovery(dm.tm, dm.lg, dm.pc)
	}

	if err := dm.fillPindex(); err != nil {
		return nil, err
	}

	P1SetVCOpen(dm.page1)
	dm.pc.FlushPage(dm.page1)

	return dm, nil

}

func Create(path string, mem int64, tm tm.TransactionManager) (*dataManager, error) {
	pc, err := pcacher.Create(path, mem)
	if err != nil {
		return nil, err
	}
	lg, err := logger.Create(path)
	if err != nil {
		return nil, err
	}

	dm := NewDataManager(pc, lg, tm)
	if err := dm.initPage1(); err != nil {
		return nil, err
	}

	return dm, nil
}

// fillPindex 构建pindex
func (dm *dataManager) fillPindex() error {
	noPages := dm.pc.NoPages()
	for i := 2; i <= noPages; i++ {
		pg, err := dm.pc.GetPage(pcacher.Pgno(i))
		if err != nil {
			return err
		}
		dm.pidx.Add(pg.Pgno(), PXFreeSpace(pg))
		pg.Release()
	}
	return nil
}

// loadAndeCheckPage1 在openDB的时候读入page1，并检验其正确性
func (dm *dataManager) loadAndCheckPage1() (bool, error) {
	var err error
	dm.page1, err = dm.pc.GetPage(1)
	if err != nil {
		return false, err
	}
	return P1CheckVC(dm.page1), nil
}

// initPage1 在CreateDB的时候用于初始化page1
func (dm *dataManager) initPage1() error {
	pgno := dm.pc.NewPage(P1InitRaw())
	utils.Assert(pgno == 1)
	var err error
	dm.page1, err = dm.pc.GetPage(pgno)
	if err != nil {
		return err
	}

	return dm.pc.FlushPage(dm.page1)
}

func (dm *dataManager) Close() error {
	// Mark page1 as cleanly closed, so recovery is skipped on next open.
	// The caller (launcher) must ensure all transactions are resolved before
	// calling Close — the server drains active connections via WaitGroup,
	// and each connection's executor aborts its active transaction on close.
	if dm.page1 != nil {
		P1SetVCClose(dm.page1)
		dm.page1.Release()
	}
	if err := dm.lg.Close(); err != nil {
		return err
	}
	return dm.pc.Close()
}

func (dm *dataManager) Insert(xid tm.XID, data []byte) (utils.UUID, error) {
	//1、将data包装成dataitem raw 并检测raw长度是不是过长
	raw := WrapDataitemRaw(data)
	if len(raw) > PXMaxFreeSpace() {
		return 0, ErrDataTooLarge
	}
	/*2、选出用来插入的raw的pgno.
	    若选择不成功 则创建新的页，然后再次尝试选择
		由于多线程，有可能在该次创建新页后，到下次它选择之前，该页已经被其他线程选走了，
		所以需要在创建新页后，再次检查是否还有空闲空间
	*/
	var pgno pcacher.Pgno
	var freeSpace int
	var pg pcacher.Page
	var err error
	for try := 0; try < 5; try++ {
		var ok bool
		pgno, freeSpace, ok = dm.pidx.Select(len(raw))
		if ok == true {
			break
		} else {
			newPgno := dm.pc.NewPage(PXInitRaw())
			dm.pidx.Add(newPgno, PXMaxFreeSpace())
		}
	}
	//选择失败 返回0和ErrBusy
	if pgno == 0 {
		return 0, ErrBusy
	}
	//该函数用于将pgno重新插回pidx
	defer func() {
		if pg != nil {
			dm.pidx.Add(pgno, PXFreeSpace(pg))
		} else {
			dm.pidx.Add(pgno, freeSpace)
		}
	}()

	/*
		3、获得该页Page实例
	*/
	pg, err = dm.pc.GetPage(pgno)
	if err != nil {
		return 0, err
	}
	/*
		4、日志记录
	*/
	log := InsertLog(xid, pg, raw)
	if err := dm.lg.Log(log); err != nil {
		return 0, err
	}

	/*
		5、将raw插入该页，并返回插入的位移
	*/
	offset := PXInsert(pg, raw)
	//释放该页,并返回UUID
	pg.Release()
	return Address2UUID(pgno, offset), nil
}

func (dm *dataManager) Read(uid utils.UUID) (Dataitem, bool, error) {
	h, err := dm.dic.Get(uid)
	if err != nil {
		return nil, false, err
	}

	di := h.(*dataitem)
	//如果dataitem为非法，则进行拦截，返回空值
	if di.IsValid() == false {
		di.Release()
		return nil, false, err
	}

	return di, true, nil
}

func (dm *dataManager) logDataitem(xid tm.XID, di *dataitem) error {
	log := UpdateLog(xid, di)
	return dm.lg.Log(log)
}

func (dm *dataManager) ReleaseDataitem(di *dataitem) {
	dm.dic.Release(di.uid)
}
