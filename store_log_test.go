package hexalog

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func Test_logStore(t *testing.T) {
	s1 := initTestServer("127.0.0.1:53211")
	s2 := initTestServer("127.0.0.1:53212")
	s3 := initTestServer("127.0.0.1:53213")

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

	e1hash := e1.Hash(s1.hlog.conf.Hasher.New())

	<-time.After(100 * time.Millisecond)
	if _, err = st1.GetEntry(e1.Key, e1hash); err != nil {
		t.Fatal(err)
	}

	if _, err = st3.GetEntry(e1.Key, e1.Hash(s3.hlog.conf.Hasher.New())); err != nil {
		t.Fatal(err)
	}

	if _, err = st2.GetEntry(e1.Key, e1.Hash(s2.hlog.conf.Hasher.New())); err != nil {
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
