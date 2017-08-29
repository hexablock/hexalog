package store

import (
	"io/ioutil"
	"os"
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

func TestBadgerEntryStore(t *testing.T) {
	testdir, err := ioutil.TempDir("", "bes-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testdir)

	st := NewBadgerEntryStore("")
	if err = st.Open(); err == nil {
		t.Fatal("Should fail")
	}

	st = NewBadgerEntryStore(testdir)
	if err = st.Open(); err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tid := []byte("foob")
	if err = st.Set(tid, &hexatype.Entry{Key: []byte("somekey")}); err != nil {
		t.Fatal(err)
	}

	ent, err := st.Get(tid)
	if err != nil {
		t.Fatal(err)
	}

	if string(ent.Key) != "somekey" {
		t.Fatal("key mismatch")
	}

	st.Delete(tid)
	if ent, err = st.Get(tid); err != hexatype.ErrEntryNotFound {
		t.Fatal("should fail with", hexatype.ErrEntryNotFound, err)
	}

}
