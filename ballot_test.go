package hexalog

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/hexablock/hexatype"
)

var testEID = []byte("test-id")

func TestBallot_votePropose(t *testing.T) {
	e1 := &hexatype.Entry{Key: []byte("key"), Height: 1}
	fe1 := NewFutureEntry(testEID, e1)
	ballot := newBallot(fe1, 3, 1*time.Second)
	for i := 0; i < 3; i++ {
		if _, err := ballot.votePropose(testEID, fmt.Sprintf("%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	//ballot.votePropose(testEID, "voter")
	if len(ballot.proposed) != 3 {
		t.Fatalf("proposal mismatch want=3 have=%d", ballot.Proposals())
	}

	e2 := &hexatype.Entry{Key: []byte("key"), Height: 2}
	fe2 := NewFutureEntry(testEID, e2)
	b2 := newBallot(fe2, 3, 1*time.Second)
	_, err := b2.votePropose(testEID, "voter")
	if err != nil {
		t.Fatal(err)
	}
	if c2, _ := b2.votePropose(testEID, "voter"); c2 != 1 {
		t.Fatal("should have 1 proposal vote")
	}

	if err = b2.Wait(); err != errBallotTimedOut {
		t.Fatal("should time out")
	}

	if _, err = b2.votePropose(testEID, "voter"); err != errBallotClosed {
		t.Fatal("should fail with ballot closed")
	}
}

func TestBallot_voteCommit(t *testing.T) {
	ttl := 1 * time.Second
	votes := 3

	e1 := &hexatype.Entry{Key: []byte("key"), Height: 1}
	fe1 := NewFutureEntry(testEID, e1)
	b1 := newBallot(fe1, votes, ttl)
	for i := 0; i < votes; i++ {
		if _, err := b1.votePropose(testEID, fmt.Sprintf("%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < votes; i++ {
		if _, err := b1.voteCommit(testEID, fmt.Sprintf("%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	if b1.Proposals() != b1.Commits() {
		t.Fatal("proposals != commits")
	} else {
		b1.close(nil)
	}

	if err := b1.Wait(); err != nil {
		t.Fatal(err)
	}

	fut := b1.Future()
	if bytes.Compare(fut.ID(), testEID) != 0 {
		t.Error("id's should match")
	}

	e2 := &hexatype.Entry{Key: []byte("key"), Height: 2}
	fe2 := NewFutureEntry(testEID, e2)
	b2 := newBallot(fe2, votes, ttl)
	for i := 0; i < votes-1; i++ {
		if _, err := b2.votePropose(testEID, fmt.Sprintf("%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := b2.votePropose([]byte("fooby"), "0"); err != errInvalidVoteID {
		t.Fatalf("should fail with '%v' have='%v'", errInvalidVoteID, err)
	}

	b2.votePropose(testEID, "2")

	if _, err := b2.voteCommit(testEID, "2"); err != nil {
		t.Fatal(err)
	}

	c, _ := b2.voteCommit(testEID, "2")
	if c != 1 {
		t.Fatal("should have 1 commit vote", c)
	}

	_, err := b2.voteCommit([]byte("fooby"), "0")
	if err != errInvalidVoteID {
		t.Fatalf("should fail with '%v' have='%v'", errInvalidVoteID, err)
	}

	b2.close(hexatype.ErrPreviousHash)
	// if _, err := b2.voteCommit(testEID, "2"); err != errBallotClosed {
	// 	t.Fatal("should fail with ballot closed", err)
	// }
	ci, err := b2.voteCommit(testEID, "2")
	if err != nil {
		t.Fatal(err)
	}
	if ci != c {
		t.Fatalf("votes want=%d have=%d", c, ci)
	}

	if c1, _ := b2.voteCommit(testEID, "new"); c1 != ci+1 {
		t.Fatal("should have votes", ci+1)
	}

	if er := b2.close(nil); er != errBallotAlreadyClosed {
		t.Fatal("should fail with", errBallotAlreadyClosed)
	}

	if b2.Error().Error() != hexatype.ErrPreviousHash.Error() {
		t.Fatal("should have error", hexatype.ErrPreviousHash)
	}

	t.Logf("Ballot runtime: %v", b2.Runtime())
}
