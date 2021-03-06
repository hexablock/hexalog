package hexalog

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"hash"
	"log"
	"testing"
	"time"
)

func newEntry(ls *LogStore, key string) error {
	k := []byte(key)
	ent := ls.NewEntry(k)
	ent.Data = k
	return ls.AppendEntry(ent)
}

func TestLogStore(t *testing.T) {
	es := NewInMemEntryStore()
	is := NewInMemIndexStore()
	ls := NewLogStore(es, is, func() hash.Hash { return sha1.New() })

	kl, err := ls.NewKey([]byte("key"))
	if err != nil {
		log.Fatal(err)
	}

	nent := ls.NewEntry([]byte("key"))
	if len(nent.Previous) == 0 {
		t.Fatal("prev should not be empty")
	}
	for _, v := range nent.Previous {
		if v != 0 {
			t.Fatal("shoulde be a zero hash")
		}
	}

	for i := 0; i < 3; i++ {
		if err = newEntry(ls, "key"); err != nil {
			t.Fatal(err)
		}
	}

	if kl.idx.Count() != 3 {
		t.Fatal("should have 3 entries for key index")
	}

	est := kl.entries.(*InMemEntryStore)
	if est.Count() != 3 {
		t.Fatal("should have 3 entries")
	}

	if kl, err = ls.GetKey([]byte("key")); err != nil {
		t.Fatal(err)
	}

	if err = ls.RemoveKey([]byte("key")); err != nil {
		t.Fatal(err)
	}

}

func Test_logStore_integration(t *testing.T) {
	s1, err := initTestServer("127.0.0.1:53211")
	if err != nil {
		t.Fatal(err)
	}
	s2, err := initTestServer("127.0.0.1:53212")
	if err != nil {
		t.Fatal(err)
	}
	s3, err := initTestServer("127.0.0.1:53213")
	if err != nil {
		t.Fatal(err)
	}

	e1 := s1.hlog.New([]byte("key"))

	ballot, err := s1.hlog.Propose(e1, testOpts1)
	if err != nil {
		t.Fatal(err, ballot)
	}
	if err = ballot.Wait(); err != nil {
		t.Fatal(err, ballot)
	}

	st1 := s1.hlog.store
	st2 := s2.hlog.store
	st3 := s3.hlog.store

	e1hash := e1.Hash(s1.hlog.conf.Hasher())

	<-time.After(100 * time.Millisecond)
	if _, err = st1.GetEntry(e1.Key, e1hash); err != nil {
		t.Fatal(err)
	}

	if _, err = st3.GetEntry(e1.Key, e1.Hash(s3.hlog.conf.Hasher())); err != nil {
		t.Fatal(err)
	}

	if _, err = st2.GetEntry(e1.Key, e1.Hash(s2.hlog.conf.Hasher())); err != nil {
		t.Fatal(err)
	}

	e2 := s1.hlog.New([]byte("key"))

	if bytes.Compare(e2.Previous, e1hash) != 0 {
		t.Fatal("previous hash wrong")
	}

	ballot, err = s1.hlog.Propose(e2, testOpts1)
	if err != nil {
		t.Fatal(err, ballot)
	}
	if err = ballot.Wait(); err != nil {
		t.Fatal(err, ballot)
	}

}

func Test_mergeErrors(t *testing.T) {
	e1 := fmt.Errorf("e1")
	e2 := fmt.Errorf("e2")

	if err := mergeErrors(e1, e2); err.Error() != "e1; e2" {
		t.Fatal("wrong error")
	}

	if err := mergeErrors(e1, nil); err.Error() != "e1" {
		t.Fail()
	}

	if err := mergeErrors(nil, e1); err.Error() != "e1" {
		t.Fail()
	}

}
