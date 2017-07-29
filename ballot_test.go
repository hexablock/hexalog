package hexalog

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

var testEID = []byte("test-id")

func TestBallot_votePropose(t *testing.T) {

	fe1 := NewFutureEntry(testEID, nil)
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

	fe2 := NewFutureEntry(testEID, nil)
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

	fe1 := NewFutureEntry(testEID, nil)
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

	fe2 := NewFutureEntry(testEID, nil)
	b2 := newBallot(fe2, votes, ttl)
	// if _, err := b2.voteCommit(testEID, "voter"); err != errProposalNotFound {
	// 	t.Fatal("should fail with", errProposalNotFound.Error())
	// }
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
	if c, _ := b2.voteCommit(testEID, "2"); c != 1 {
		t.Fatal("should have 1 commit vote", c)
	}

	if _, err := b2.voteCommit([]byte("fooby"), "0"); err != errInvalidVoteID {
		t.Fatalf("should fail with '%v' have='%v'", errInvalidVoteID, err)
	}

	b2.voteCommit(testEID, "0")
	b2.voteCommit(testEID, "1")
	if _, err := b2.voteCommit(testEID, "2"); err != errBallotClosed {
		t.Fatal("should fail with ballot closed", err)
	}

}
