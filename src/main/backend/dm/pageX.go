/*
		pageX.go实现了对普通页的管理

		构造如下：
		[Free Space Offset] uint16
	    [Data] *

		[Free Space Offset] 表示空闲空间的位置指针.
*/
package dm

import "mydb/src/main/backend/dm/pcacher"

const (
	_PX_OF_FREE = 0
	_PX_OF_DATA = 2
)

func PXInitRaw() []byte {
	raw := make([]byte, pcacher.PAGE_SIZE)
	pxRawUpdateFSO(raw, _PX_OF_DATA) // 初始时将FSO初始化为DATA的位移
	return raw
}

// 返回普通页的最大空闲空间大小
func PXMaxFreeSpace() int {
	return pcacher.PAGE_SIZE - _PX_OF_DATA
}

// 通过raw，取得free space offset的内容
func pxRawFSO(raw []byte) Offset {
	return ParseOffset(raw[_PX_OF_FREE:])
}

func PxFSO(pg pcacher.Page) Offset {
	return pxRawFSO(pg.Data())
}

// 更新raw中FSO的内容
func pxRawUpdateFSO(raw []byte, offset Offset) {
	PutOffset(raw[_PX_OF_FREE:], offset)
}

// 将raw插入到pg这一页，并返回插入到位移
func PXInsert(pg pcacher.Page, raw []byte) Offset {
	pg.Dirty()
	offset := pxRawFSO(pg.Data())
	copy(pg.Data()[offset:], raw)
	pxRawUpdateFSO(pg.Data(), offset+Offset(len(raw)))
	return offset
}

// 返回pg的freespace大小
func PXFreeSpace(pg pcacher.Page) int {
	return pcacher.PAGE_SIZE - int(pxRawFSO(pg.Data()))
}

// PXRecoverUpdate 辅助Recovery，直接将raw的值复制到pg的offset位置
func PXRecoverUpdate(pg pcacher.Page, offset Offset, raw []byte) {
	pg.Dirty()
	copy(pg.Data()[offset:], raw)
}

// PXRecoverInsert 辅助Recovery，直接将raw插入到pg这一页，并返回插入到位移
func PXRecoverInsert(pg pcacher.Page, offset Offset, raw []byte) {
	pg.Dirty()
	copy(pg.Data()[offset:], raw)

	maxFSO := pxRawFSO(pg.Data())
	fso2 := offset + Offset(len(raw))
	if fso2 > maxFSO {
		maxFSO = fso2
	}
	pxRawUpdateFSO(pg.Data(), maxFSO)
}
