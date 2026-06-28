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
//
// BUG FIX: If the database crashed while updating the page's FSO (Free Space
// Offset), the on-disk FSO may be garbage (e.g. 0xFFFF). Previously we did
// max(current_FSO, offset+len(raw)), which preserved the garbage value
// permanently, wasting page space.
//
// Now we validate current_FSO against page bounds. If it's out of range,
// we ignore it and use only the log-derived value (offset + len(raw)).
func PXRecoverInsert(pg pcacher.Page, offset Offset, raw []byte) {
	pg.Dirty()
	copy(pg.Data()[offset:], raw)

	// Correct FSO computed from the WAL log (authoritative).
	logFSO := offset + Offset(len(raw))

	// On-disk FSO may be corrupted if the crash occurred mid-FSO-update.
	// Validate it against page geometry before trusting it.
	currentFSO := pxRawFSO(pg.Data())
	if int(currentFSO) < _PX_OF_DATA || int(currentFSO) > pcacher.PAGE_SIZE {
		// Garbage — fall back to the log-derived value.
		currentFSO = logFSO
	}

	if logFSO > currentFSO {
		pxRawUpdateFSO(pg.Data(), logFSO)
	}
}
