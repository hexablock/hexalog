package store

import (
	"bytes"
	"testing"
	"time"
)

func TestInMemStableStore(t *testing.T) {
	// td, err := ioutil.TempDir("/tmp", "sstore")
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// defer os.RemoveAll(td)

	// opt := badger.DefaultOptions
	// opt.Dir = td
	// opt.ValueDir = td
	//ss := NewBadgerStableStore(opt)
	ss := &InMemStableStore{}
	err := ss.Open()
	if err != nil {
		t.Fatal(err)
	}

	ss.Set([]byte("key"), []byte("value"))
	ss.Set([]byte("key1"), []byte("value"))
	ss.Set([]byte("key2"), []byte("value"))
	ss.Set([]byte("key3"), []byte("value"))

	val, err := ss.Get([]byte("key"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(val, []byte("value")) != 0 {
		t.Fatal("wrong value")
	}
	// Wait for flush
	<-time.After(1 * time.Second)
	val, err = ss.Get([]byte("key"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(val, []byte("value")) != 0 {
		t.Fatal("wrong value")
	}

	if err = ss.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err = ss.Get([]byte("not-found")); err == nil {
		t.Fatal("key should not be found")
	}

	// Re-open  store and check
	//ss1 := NewBadgerStableStore(opt)
	// if err = ss1.Open(); err != nil {
	// 	t.Fatal(err)
	// }
	//
	// val, err = ss1.Get([]byte("key"))
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// if bytes.Compare(val, []byte("value")) != 0 {
	// 	t.Fatalf("wrong value '%s'", val)
	// }
	//
	// if err = ss1.Close(); err != nil {
	// 	t.Fatal(err)
	// }
}
