package pcacher

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestPcacherCreateGetReleasePersist 验证页创建、修改释放后可持久读取。
func TestPcacherCreateGetReleasePersist(t *testing.T) {
	base := filepath.Join(t.TempDir(), "pcacher_rw")
	p := Create(base, PAGE_SIZE*20)
	defer p.Close()

	raw := make([]byte, PAGE_SIZE)
	for i := 0; i < len(raw); i++ {
		raw[i] = byte(i % 251)
	}

	pgno := p.NewPage(raw)
	if pgno != 1 {
		t.Fatalf("pgno=%d, want=1", pgno)
	}
	if p.NoPages() != 1 {
		t.Fatalf("NoPages=%d, want=1", p.NoPages())
	}

	pg, err := p.GetPage(pgno)
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	if !bytes.Equal(pg.Data(), raw) {
		t.Fatalf("page data mismatch after first read")
	}

	expected := make([]byte, PAGE_SIZE)
	copy(expected, pg.Data())
	expected[0] = 77
	expected[PAGE_SIZE-1] = 99
	pg.Dirty()
	pg.Data()[0] = 77
	pg.Data()[PAGE_SIZE-1] = 99
	pg.Release()

	pg2, err := p.GetPage(pgno)
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	defer pg2.Release()
	if !bytes.Equal(pg2.Data(), expected) {
		t.Fatalf("page data mismatch after dirty+release")
	}
}

// TestPcacherOpenAndTruncate 验证重开后页数恢复与截断行为正确。
func TestPcacherOpenAndTruncate(t *testing.T) {
	base := filepath.Join(t.TempDir(), "pcacher_truncate")
	p := Create(base, PAGE_SIZE*20)
	empty := make([]byte, PAGE_SIZE)
	p.NewPage(empty)
	p.NewPage(empty)
	p.Close()

	p2 := Open(base, PAGE_SIZE*20)
	if p2.NoPages() != 2 {
		t.Fatalf("NoPages=%d, want=2", p2.NoPages())
	}

	p2.TruncateByPgno(1)
	if p2.NoPages() != 1 {
		t.Fatalf("NoPages=%d, want=1", p2.NoPages())
	}
	p2.Close()

	info, err := os.Stat(base + SUFFIX_DB)
	if err != nil {
		t.Fatalf("stat db file error: %v", err)
	}
	if info.Size() != int64(PAGE_SIZE) {
		t.Fatalf("file size=%d, want=%d", info.Size(), PAGE_SIZE)
	}

	p3 := Open(base, PAGE_SIZE*20)
	defer p3.Close()
	if p3.NoPages() != 1 {
		t.Fatalf("NoPages=%d, want=1", p3.NoPages())
	}
}

// TestPcacherCreateMemTooSmallPanics 验证内存不足时会触发预期 panic。
func TestPcacherCreateMemTooSmallPanics(t *testing.T) {
	base := filepath.Join(t.TempDir(), "pcacher_small_mem")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic type=%T, want error", r)
		}
		if !errors.Is(err, ErrMemTooSmall) {
			t.Fatalf("panic=%v, want=%v", err, ErrMemTooSmall)
		}
	}()

	_ = Create(base, PAGE_SIZE*5)
}
