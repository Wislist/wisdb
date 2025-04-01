package utils

import "testing"

func TestBytesToInt(t *testing.T) {
	tests := []struct {
		input  []byte
		expect int32
	}{
		{[]byte{0x00, 0x00, 0x00, 0x01}, 1},
		{[]byte{0x7F, 0xFF, 0xFF, 0xFF}, 2147483647},
	}

	for _, tt := range tests {
		got := BytesToInt(tt.input)
		if got != tt.expect {
			t.Errorf("BytesToInt(%v) = %v, want %v", tt.input, got, tt.expect)
		}
	}
}
