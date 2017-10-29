package hexalog

import (
	"bytes"
	"io"
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
	fut := ballot.Future()
	if _, err = fut.Wait(2 * time.Second); err != nil {
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

	if err = s1.hlog.trans.PushKeylog("127.0.0.1:43213", []byte("testkey"), opts); err != nil {
		t.Fatal(err)
	}

	if _, err = s1.hlog.trans.PullKeylog("127.0.0.1:43213", entry, opts); err != nil {
		t.Fatal(err)
	}

	<-time.After(100 * time.Millisecond)

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

	if err = s2.hlog.trans.PushKeylog("127.0.0.1:43213", []byte("does-not-exist"), opts); err != hexatype.ErrKeyNotFound {
		t.Fatal("should fail with", hexatype.ErrKeyNotFound, err)
	}

	tk, _ := s1.hlog.store.GetKey([]byte("testkey"))
	idx := tk.GetIndex()
	if idx.LTime == 0 {
		t.Error("ltime is 0")
	}

	seeds, err := s2.hlog.trans.GetSeedKeys("127.0.0.1:43213")
	if err != nil {
		t.Error(err.Error())
	}

	var seed *KeySeed
	var c int
	for {
		seed, err = seeds.Recv()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		t.Log(seed)
		c++
	}
	seeds.Recycle()

	if err != nil {
		t.Error(err.Error())
	}
	if c == 0 {
		t.Fatal("have 0 seeds")
	}

	testOpts.WaitApply = true
	testOpts.WaitBallot = true
	n := s3.hlog.New([]byte("foobar"))
	s3.hlog.Propose(n, testOpts)
	n = s3.hlog.New([]byte("foobar1"))
	s3.hlog.Propose(n, testOpts)
	n = s3.hlog.New([]byte("foobar2"))
	s3.hlog.Propose(n, testOpts)

	s4, err := initTestServer("127.0.0.1:43214")
	if err != nil {
		t.Fatal(err)
	}

	if err = s4.hlog.Seed("127.0.0.1:43211", 16, 3); err != nil {
		t.Error(err)
	}

	if err = s4.hlog.heal([]byte("foobar1"), testOpts.PeerSet); err != nil {
		t.Error(err)
	}
	if err = s4.hlog.Heal([]byte("foobar2"), testOpts); err != nil {
		t.Error(err)
	}

	s1.stop()
	s2.stop()
	s3.stop()
	s4.stop()
}
