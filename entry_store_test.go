package hexalog

import (
	"testing"

	"github.com/hexablock/hexatype"
)

func TestInMemEntryStore(t *testing.T) {
	entries := NewInMemEntryStore()
	if err := entries.Set([]byte("id1"), &Entry{Key: []byte("key1")}); err != nil {
		t.Fatal(err)
	}
	if entries.Count() != 1 {
		t.Fatal("count should be 1")
	}

	if _, err := entries.Get([]byte("foo")); err != hexatype.ErrEntryNotFound {
		t.Fatal("should fail with", hexatype.ErrEntryNotFound, err)
	}

	if err := entries.Delete([]byte("id1")); err != nil {
		t.Fatal(err)
	}

	if _, err := entries.Get([]byte("id1")); err != hexatype.ErrEntryNotFound {
		t.Fatal("should fail with", hexatype.ErrEntryNotFound, err)
	}
}
