package booter

import (
	"io"
	"os"
)

/*
	Booter 管理其他模块的启动文件
	提供了两个函数：（确保其原子性）
		1. Load()：启动其他模块
		2. Update()：更新其他模块

*/

const (
	_SUFFIX     = ".bt"
	_SUFFIX_TMP = ".bt_tmp"
)

type Booter interface {
	Load() []byte
	Update(data []byte) // 原子性的更新
}

type booter struct {
	path string
	file *os.File
}

func Create(path string) *booter {
	removeBadTMP(path)

	file, err := os.OpenFile(path+_SUFFIX, os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
	return &booter{
		path: path,
		file: file,
	}
}

// removeBadTMP 移除之前因为数据库崩坏遗留下来的tmp文件
func removeBadTMP(path string) {
	os.Remove(path + _SUFFIX_TMP)
}

func (bt *booter) Load() []byte {
	_, err := bt.file.Seek(0, 0)
	if err != nil {
		panic(err)
	}
	buf, err := io.ReadAll(bt.file)
	if err != nil {
		panic(err)
	}
	return buf
}

func (bt *booter) Update(data []byte) {
	f, err := os.OpenFile(bt.path + _SUFFIX_TMP, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}

	_, err = f.Write(data)
	if err != nil {
		panic(err)
	}

	err = f.Sync()
	if err != nil {
		panic(err)
	}


	err = f.Close()
	if err != nil {
		panic(err)
	}

	//os.Rename 被当作是原子性的
	err = os.Rename(bt.path + _SUFFIX_TMP, bt.path + _SUFFIX)
	if err != nil {
		panic(err)
	}

	bt.file, err = os.OpenFile(bt.path + _SUFFIX, os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
}
