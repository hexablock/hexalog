package store

import (
	"bytes"
	"testing"

	"github.com/hexablock/hexatype"
)

func TestInMemStableStore(t *testing.T) {
	ss := &InMemStableStore{}
	err := ss.Open()
	if err != nil {
		t.Fatal(err)
	}

	ss.Set([]byte("key1"), []byte("value"))
	ss.Set([]byte("key2"), []byte("value"))
	ss.Set([]byte("key3"), []byte("value"))
	ss.Set([]byte("key4"), []byte("value"))

	val, err := ss.Get([]byte("key1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(val, []byte("value")) != 0 {
		t.Fatal("wrong value")
	}

	val, err = ss.Get([]byte("key3"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(val, []byte("value")) != 0 {
		t.Fatal("wrong value")
	}

	if err = ss.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err = ss.Get([]byte("not-found")); err != hexatype.ErrKeyNotFound {
		t.Fatal("key should not be found")
	}

}
