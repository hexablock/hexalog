package store

import (
	"bytes"
	"io/ioutil"
	"os"
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

func TestBadgerStableStore(t *testing.T) {
	td, err := ioutil.TempDir("/tmp", "sstore")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(td)

	ss := NewBadgerStableStore(td)
	if err = ss.Open(); err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	if err = ss.Set([]byte("key"), []byte("data")); err != nil {
		t.Fatal(err)
	}

	_, err = ss.Get([]byte("key"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err = ss.Get([]byte("keyber")); err != hexatype.ErrKeyNotFound {
		t.Fatal("should fail with", hexatype.ErrKeyNotFound, err)
	}

	if err = ss.Set([]byte("key2"), []byte("data")); err != nil {
		t.Fatal(err)
	}

	var c int
	ss.Iter(func(key []byte, val []byte) error {
		c++
		return nil
	})

	if c != 2 {
		t.Fatal("should have 2 keys")
	}
}
