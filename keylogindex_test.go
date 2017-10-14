package hexalog

import "testing"

func Test_UnsafeKeylogIndex(t *testing.T) {
	ukli := NewUnsafeKeylogIndex([]byte("key"))
	ukli.Append([]byte("foo"), make([]byte, 32))
}
