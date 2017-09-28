package store

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/hexablock/hexatype"
)

var (
	testBIdx *BadgerIndexStore
	testkey  = []byte("testkey")
)

func TestMain(m *testing.M) {

	tdir, _ := ioutil.TempDir("", "hexalog")
	testBIdx = NewBadgerIndexStore(tdir)
	if err := testBIdx.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ec := m.Run()
	testBIdx.Close()
	os.RemoveAll(tdir)

	os.Exit(ec)
}

func TestBadgerIndexStore(t *testing.T) {

	nk, err := testBIdx.NewKey(testkey)
	if err != nil {
		t.Fatal(err)
	} else if nk == nil {
		t.Fatal("keylog index should not be nil")
	}

	if _, err = testBIdx.GetKey(testkey); err != nil {
		t.Fatal(err)
	}

	if _, err = testBIdx.NewKey(testkey); err != hexatype.ErrKeyExists {
		t.Fatal("should fail with", hexatype.ErrKeyExists, err)
	}

	for i := 0; i < 10; i++ {
		if _, err = testBIdx.MarkKey([]byte(fmt.Sprintf("key%d", i)), nil); err != nil {
			t.Fatal(err)
		}
	}

	var cnt int
	if err = testBIdx.Iter(func(key []byte, idx hexatype.KeylogIndex) error {
		cnt++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if cnt != 11 {
		t.Fatal("should have 11 keys. got", cnt)
	}

	if err = testBIdx.RemoveKey([]byte("key3")); err != nil {
		t.Fatal(err)
	}

	if _, err = testBIdx.GetKey([]byte("key3")); err != hexatype.ErrKeyNotFound {
		t.Fatal("should fail with", hexatype.ErrKeyNotFound, err)
	}

	if err = testBIdx.RemoveKey([]byte("non-exist")); err != hexatype.ErrKeyNotFound {
		t.Fatal("should fail with", hexatype.ErrKeyNotFound, err)
	}

	if _, err = testBIdx.NewKey([]byte("key3")); err != nil {
		t.Fatal(err)
	}

}

func TestBadgerKeylogIndex(t *testing.T) {

	gk, _ := testBIdx.GetKey(testkey)

	z := make([]byte, 7)
	fst := []byte{1, 1, 1, 1, 1, 1, 1}
	err := gk.Append(fst, z)
	if err != nil {
		t.Fatal(err)
	}

	if err = gk.Append(fst, z); err == nil {
		t.Fatal("should fail")
	}

	sec := []byte{2, 2, 2, 2, 2, 2, 2}
	if err = gk.Append(sec, fst); err != nil {
		t.Fatal(err)
	}

	if last := gk.Last(); bytes.Compare(sec, last) != 0 {
		t.Fatal("last mismatch")
	}

	if _, err = gk.SetMarker([]byte("marker")); err != nil {
		t.Fatal(err)
	}
	if string(gk.Marker()) != "marker" {
		t.Fatal("marker mismatch")
	}

	bdg := gk.(*BadgerKeylogIndex)
	if string(gk.Key()) != string(bdg.idx.Key) {
		t.Fatal("key mismatch")
	}

	g2, _ := testBIdx.GetKey(testkey)

	if g2.Height() != gk.Height() {
		t.Fatal("height mismatch")
	}

	if string(gk.Marker()) != string(g2.Marker()) {
		t.Fatal("marker mismatch")
	}

	g2.Rollback()

	if _, ok := gk.Rollback(); ok {
		t.Fatal("should fail rollback")
	}

	c1 := gk.Count()
	c2 := g2.Count()
	if c1 != c2 {
		t.Fatal("entry cound mismatch", c1, c2)
	}

	if bytes.Compare(gk.Last(), g2.Last()) != 0 {
		t.Fatal("last entry mismatch")
	}

}
