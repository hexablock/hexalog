package hexalog

import "testing"

func Test_SafeKeylogIndex(t *testing.T) {
	//ukli := NewUnsafeKeylogIndex([]byte("key"))
	ukli := NewSafeKeylogIndex([]byte("key"), nil)

	if err := ukli.Append([]byte("foo"), make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	if err := ukli.Append([]byte("bar"), []byte("foo")); err != nil {
		t.Fatal(err)
	}

	if ok, _ := ukli.SetMarker([]byte("marker")); !ok {
		t.Fatal("should be able to set marker")
	}
	if ok, _ := ukli.SetMarker([]byte("bar")); ok {
		t.Fatal("should fail to set marker")
	}
	c, ok := ukli.Rollback()
	if !ok {
		t.Fatal("rollback failed")
	}
	if c != 1 || ukli.Height() != 1 {
		t.Fatal("should have 1")
	}

	if !ukli.Contains([]byte("foo")) {
		t.Fatal("should have foo")
	}

	idx := ukli.Index()
	if idx.Count() != ukli.Count() {
		t.Fatal("count mismatch")
	}
	if _, err := idx.MarshalJSON(); err != nil {
		t.Fatal(err)
	}
}
