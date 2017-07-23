package hexalog

import (
	"bytes"
	"testing"
	"time"
)

func TestNetTransport(t *testing.T) {

	s1 := initTestServer("127.0.0.1:43211")
	s2 := initTestServer("127.0.0.1:43212")
	s3 := initTestServer("127.0.0.1:43213")

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

	id := entry.Hash(s1.hlog.conf.Hasher.New())
	e2, err := s1.hlog.trans.GetEntry("127.0.0.1:43212", []byte("testkey"), id, &RequestOptions{})
	if err != nil {
		t.Error(err)
	}

	id2 := e2.Hash(s1.hlog.conf.Hasher.New())
	if bytes.Compare(id, id2) != 0 {
		t.Error("id mismatch")
	}

	if err = s1.hlog.trans.TransferKeylog("127.0.0.1:43213", []byte("testkey")); err != nil {
		t.Fatal(err)
	}

	<-time.After(100 * time.Millisecond)

	s1.stop()
	s2.stop()
	s3.stop()
}
