package hexalog

import (
	"bytes"
	"testing"
	"time"

	"github.com/hexablock/hexatype"
)

func TestNetTransport(t *testing.T) {

	s1, err := initTestServer("127.0.0.1:43211")
	if err != nil {
		t.Fatal(err)
	}
	s2, err := initTestServer("127.0.0.1:43212")
	if err != nil {
		t.Fatal(err)
	}
	s3, err := initTestServer("127.0.0.1:43213")
	if err != nil {
		t.Fatal(err)
	}

	//s4 := initTestServer("127.0.0.1:43214")

	entry := s2.hlog.New([]byte("testkey"))

	ballot, err := s1.hlog.Propose(entry, testOpts)
	if err != nil {
		t.Fatal(err, ballot)
	}

	if err = ballot.Wait(); err != nil {
		t.Fatal(err, ballot)
	}

	if _, err = ballot.fentry.Wait(2 * time.Second); err != nil {
		t.Fatal(err, ballot)
	}

	t.Logf("time ballot=%v fsm=%v", ballot.Runtime(), ballot.fentry.Runtime())
	<-time.After(50 * time.Millisecond)

	id := entry.Hash(s1.hlog.conf.Hasher())
	e2, err := s1.hlog.trans.GetEntry("127.0.0.1:43212", []byte("testkey"), id, &RequestOptions{})
	if err != nil {
		t.Fatalf("key=testkey id=%x %v", id, err)
	}

	id2 := e2.Hash(s1.hlog.conf.Hasher())
	if bytes.Compare(id, id2) != 0 {
		t.Error("id mismatch")
	}

	if _, err = s3.hlog.store.GetEntry(entry.Key, id); err != nil {
		t.Fatal(err)
	}

	opts := &RequestOptions{}

	if err = s1.hlog.trans.TransferKeylog("127.0.0.1:43213", []byte("testkey"), opts); err != nil {
		t.Fatal(err)
	}

	if _, err = s1.hlog.trans.FetchKeylog("127.0.0.1:43213", entry, opts); err != nil {
		t.Fatal(err)
	}

	<-time.After(100 * time.Millisecond)

	//s4.hlog.trans.FetchKeylog("127.0.0.1:43212", entry)

	l2, err := s2.hlog.trans.LastEntry("127.0.0.1:43213", []byte("testkey"), &RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if l2 == nil {
		t.Fatal("last should not be nil")
	}

	if _, err = s2.hlog.store.NewKey([]byte("does-not-exist")); err != nil {
		t.Fatal(err)
	}
	ne := s2.hlog.store.NewEntry([]byte("does-not-exist"))
	if err = s2.hlog.store.AppendEntry(ne); err != nil {
		t.Fatal(err)
	}

	if err = s2.hlog.trans.TransferKeylog("127.0.0.1:43213", []byte("does-not-exist"), opts); err != hexatype.ErrKeyNotFound {
		t.Fatal("should fail with", hexatype.ErrKeyNotFound, err)
	}

	s1.stop()
	s2.stop()
	s3.stop()
}
