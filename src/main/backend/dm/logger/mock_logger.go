package logger

type mockLogger struct {
}

func OpenMock(path string) *mockLogger   { return new(mockLogger) }
func CreateMock(path string) *mockLogger { return new(mockLogger) }

func (ml *mockLogger) Log(data []byte) error        { return nil }
func (ml *mockLogger) Truncate(x int64) error { return nil }
func (ml *mockLogger) Next() ([]byte, bool, error)   { return nil, true, nil }
func (ml *mockLogger) Rewind()                {}
func (ml *mockLogger) Close() error                { return nil }
