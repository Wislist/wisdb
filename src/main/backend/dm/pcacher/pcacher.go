package pcacher

/**
* PageCacher 实现对页面的缓存
实际上pcacher已经将缓存的逻辑托管到cacher.Cacher了
所以在pcacher只需要实现对磁盘的操作的部分逻辑
*/
import (
	"errors"
	"mydb/src/main/backend/utils"
	"os"
	"sync"
	"sync/atomic"

	"mydb/src/main/backend/utils/cacher"
)

var (
	ErrMemTooSmall = errors.New("Memory is too small.")
)

const (
	PAGE_SIZE = 1 << 13
	_MEM_LIM  = 10

	SUFFIX_DB = ".db"
)

type Pcacher interface {
	/*
	* NewPage 创建一个新的页面
	* initData 初始化页面的内容
	* 返回新页面的页号
	 */
	NewPage(initData []byte) Pgno
	GetPage(pgno Pgno) (Page, error)
	Close()

	/*recobery的时候才会被调用*/
	TruncateByPgno(maxPgno Pgno) //将DB扩充为maxPgno这么多页的空间
	NoPages() int                //返回DB有多少页
	FlushPage(pg Page)           //强制刷新pg到磁盘

}

type pcacher struct {
	file     *os.File
	fileLock sync.Mutex

	noPages uint32

	c cacher.Cacher
}

func Create(path string, mem int64) *pcacher {
	file, err := os.OpenFile(path+SUFFIX_DB, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	return newPcacher(file, mem)
}

func Open(path string, mem int64) *pcacher {
	file, err := os.OpenFile(path+SUFFIX_DB, os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
	return newPcacher(file, mem)
}
func newPcacher(file *os.File, mem int64) *pcacher {
	if mem/PAGE_SIZE < _MEM_LIM {
		panic(ErrMemTooSmall)
	}

	info, err := file.Stat()
	if err != nil {
		panic(err)
	}
	//计算文件的总页数
	size := info.Size()

	p := new(pcacher)
	options := new(cacher.Options)
	options.Get = p.getForCacher
	options.MaxHandles = uint32(mem / PAGE_SIZE)
	options.Release = p.releaseForCacher
	c := cacher.NewCacher(options)
	p.c = c
	p.file = file
	p.noPages = uint32(size / PAGE_SIZE)

	return p

}

func (p *pcacher) Close() {
	p.c.Close()
	p.file.Close()
}

func (p *pcacher) NewPage(initData []byte) Pgno {
	//对noPages增加1，且预留出一个页面的位置
	pgno := Pgno(atomic.AddUint32(&p.noPages, 1))
	pg := NewPage(pgno, initData, nil)
	p.flush(pg)
	return pgno
}

func (p *pcacher) GetPage(pgno Pgno) (Page, error) {
	uid := Pgno2UUID(pgno)
	underlying, err := p.c.Get(uid)
	if err != nil {
		return nil, err
	}
	return underlying.(*page), nil
}

// get 根据pgno从DB文件中读取页的内容，并包裹成一页返回
// get必须能够支持并发
func (p *pcacher) getForCacher(uid utils.UUID) (interface{}, error) {
	pgno := UUID2Pgno(uid)
	offset := pageOffset(pgno)

	buf := make([]byte, PAGE_SIZE)
	p.fileLock.Lock()
	_, err := p.file.ReadAt(buf, offset)
	if err != nil {
		utils.Fatal(uid, "Read:", pgno, ",", offset, ",", err) //DB文件出现问题了，应该立刻停止
	}
	p.fileLock.Unlock()

	pg := NewPage(pgno, buf, p)
	return pg, nil
}

// release 当cacher释放页缓存时，会回调到这里
// release必须能够支持并发
func (p *pcacher) releaseForCacher(underlying interface{}) {
	pg := underlying.(*page)
	if pg.dirty == true {
		p.flush(pg)
		pg.dirty = false
	}
}

func (p *pcacher) release(pg *page) {
	p.c.Release(Pgno2UUID(pg.pgno))
}

func (p *pcacher) flush(pg *page) {
	pgno := pg.pgno
	offset := pageOffset(pgno)

	p.fileLock.Lock()
	defer p.fileLock.Unlock()
	_, err := p.file.WriteAt(pg.data, offset)
	if err != nil {
		panic(err)
	}
	err = p.file.Sync()
	if err != nil {
		panic(err)
	}
}

func (p *pcacher) TruncateByPgno(maxPgno Pgno) {
	size := pageOffset(maxPgno + 1)
	err := p.file.Truncate(size)
	if err != nil {
		panic(err)
	}
	p.noPages = uint32(maxPgno)
}

func (p *pcacher) NoPages() int {
	return int(p.noPages)
}

func (p *pcacher) FlushPage(pgi Page) {
	pg := pgi.(*page)
	p.flush(pg)
}

func pageOffset(pgno Pgno) int64 {
	//页号从1开始 偏移量 = (页号 - 1) * 页大小
	return int64(pgno-1) * PAGE_SIZE
}
