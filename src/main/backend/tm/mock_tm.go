package tm

/*
	 test transaction manager
*/
type MockTranManager struct {
}

func CreateMock(path string) *MockTranManager {
	return new(MockTranManager)
}

func OpenMock(path string) *MockTranManager {
	return new(MockTranManager)
}

func (mtm *MockTranManager) Begin() (XID, error) {
	return 0, nil
}

func (mtm *MockTranManager) Commit(xid XID) error {
	return nil
}

func (mtm *MockTranManager) Abort(xid XID) error {
	return nil
}

func (mtm *MockTranManager) IsActive(xid XID) bool {
	return false
}


func (mtm *MockTranManager) IsCommitted(xid XID) bool {
	return false
}

func (mtm *MockTranManager) IsAborted(xid XID) bool {
	return false
}

func (mtm *MockTranManager) Close() error {
	return nil
}






















