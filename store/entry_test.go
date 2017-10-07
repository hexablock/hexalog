package store

import (
	"testing"

	"github.com/hexablock/hexatype"
)

func TestInMemEntryStore(t *testing.T) {
	entries := NewInMemEntryStore()
	if err := entries.Set([]byte("id1"), &hexatype.Entry{Key: []byte("key1")}); err != nil {
		t.Fatal(err)
	}
	if entries.Count() != 1 {
		t.Fatal("count should be 1")
	}

	if _, err := entries.Get([]byte("foo")); err != hexatype.ErrEntryNotFound {
		t.Fatal("should fail with", hexatype.ErrEntryNotFound, err)
	}
}
