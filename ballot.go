package hexalog

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexablock/log"
)

var (
	errBallotTimedOut      = errors.New("ballot timed out")
	errBallotClosed        = errors.New("ballot closed")
	errBallotAlreadyClosed = errors.New("ballot already closed")
	errBallotNotFound      = errors.New("ballot not found")
	errNotEnoughProposals  = errors.New("not enough proposals")
	errVoterAlreadyVoted   = errors.New("voter already voted")
	errInvalidVoteID       = errors.New("invalid vote id")
)

// Ballot holds information to execute a singular distributed operation ensuring consistency.  It is
// primarily used to track the state of proposal and commit votes to appropriately handle an operation.
type Ballot struct {
	// Future for the Entry being voted on.  This is used to know and wait on when an actual
	// entry is applied to the application defined FSM
	fentry *FutureEntry

	// Time voting first began on the ballot
	dispatched time.Time
	// Time ballot was closed
	completed time.Time

	// TTL for the ballot. The ballot is closed once the ttl has been reached
	ttl time.Duration
	// TTL timer
	timer *time.Timer

	// Map to count proposed votes
	pmu      sync.RWMutex
	proposed []string

	// Map to count committed votes
	cmu       sync.RWMutex
	committed []string

	// Votes required for both proposed and committed
	votes int

	// Error channel used to wait for completion
	errc chan error
	// Error the ballot was closed with.  This is the same error that would be in the error
	// channel
	err error

	// Done channel used to wait for completion
	done chan struct{}

	// This is set once the ballot has been closed to stop further processing/
	closed int32
}

func newBallot(fentry *FutureEntry, requiredVotes int, ttl time.Duration) *Ballot {
	return &Ballot{
		fentry:    fentry,
		ttl:       ttl,
		votes:     requiredVotes,
		proposed:  make([]string, requiredVotes),
		committed: make([]string, requiredVotes),
		errc:      make(chan error, requiredVotes),
		done:      make(chan struct{}, requiredVotes),
	}
}

// Wait blocks until voting on the ballot is complete
func (b *Ballot) Wait() error {
	var err error

	select {
	case <-b.done:
	case err = <-b.errc:
	}

	return err
}

// Future returns a FutureEntry associated to the ballot.  It is the entry being voted on
// and can be used to wait for it to be applied to the FSM.
func (b *Ballot) Future() *FutureEntry {
	return b.fentry
}

// Commits safely returns the number of current commits
func (b *Ballot) Commits() int {
	b.cmu.RLock()
	defer b.cmu.RUnlock()

	return countVotes(b.committed)
}

// Proposals safely returns the number of current proposals
func (b *Ballot) Proposals() int {
	b.pmu.RLock()
	defer b.pmu.RUnlock()

	//return len(b.proposed)
	return countVotes(b.proposed)
}

// votePropose submit a vote for the propose phase.  It takes an Entry hash id and a voter
// as parameters.  If the voter has already voted it simply returns the proposed votes
// with no error.  An error is returned if the supplied id does not match the the entry id
// of the ballot.
func (b *Ballot) votePropose(entryID []byte, voter string, voterIdx int) (int, bool, error) {
	// Check the entry id to make sure it matches the ballot.
	if bytes.Compare(b.fentry.ID(), entryID) != 0 {
		return -1, false, errInvalidVoteID
	}

	if atomic.LoadInt32(&b.closed) == 1 {
		if b.err == nil {
			return -1, false, errBallotClosed
		}
		return -1, false, b.err
	}

	b.pmu.Lock()
	defer b.pmu.Unlock()

	if b.proposed[voterIdx] == "" {
		b.proposed[voterIdx] = voter
	} else {
		log.Println("[WARN] Already voted for propose:", voter)
		return countVotes(b.proposed), false, nil
	}

	proposals := countVotes(b.proposed)

	// Initiaze timer if this is the first proposal for ballot.  Do not set ttl if it is
	// the same voter,trying to vote again.
	if proposals == 1 {
		b.dispatched = time.Now()
		b.setTTL()
	}

	return proposals, true, nil
}

func (b *Ballot) haveProposal(voter string) bool {
	for _, s := range b.proposed {
		if s == voter {
			return true
		}
	}
	return false
}

// voteCommit submits a commit vote.  If the voter has already voted it simply
// returns the committed votes with no error
func (b *Ballot) voteCommit(entryID []byte, voter string, voterIdx int) (int, bool, error) {
	// Check the entry id to make sure it matches the ballot.
	if bytes.Compare(b.fentry.ID(), entryID) != 0 {
		return -1, false, errInvalidVoteID
	}

	if atomic.LoadInt32(&b.closed) == 1 {
		if b.err == nil {
			return -1, false, errBallotClosed
		}
		return -1, false, b.err
	}

	// Add commit vote
	b.cmu.Lock()
	defer b.cmu.Unlock()

	if b.committed[voterIdx] == "" {
		b.committed[voterIdx] = voter
	} else {
		log.Println("[DEBUG] Already voted for commit:", voter)
		return countVotes(b.committed), false, nil
	}

	return countVotes(b.committed), true, nil
}

// setTTL starts the counter to appropriately expire the ballot
func (b *Ballot) setTTL() {
	log.Printf("[DEBUG] Ballot opened key=%s ballot=%p required-votes=%d", b.fentry.Entry.Key, b, b.votes)
	// Setup ballot expiration
	b.timer = time.AfterFunc(b.ttl, func() {
		atomic.StoreInt32(&b.closed, 1)
		b.errc <- errBallotTimedOut
	})
}

// Closed returns whether or not a ballot has been closed
func (b *Ballot) Closed() bool {
	return atomic.LoadInt32(&b.closed) == 1
}

// close the ballot stopping the timer and writing err to the done chan.
func (b *Ballot) close(err error) error {
	if atomic.LoadInt32(&b.closed) == 1 {
		return b.err
	}

	b.timer.Stop()
	atomic.StoreInt32(&b.closed, 1)

	b.cmu.Lock()
	b.completed = time.Now()
	b.err = err
	b.cmu.Unlock()

	switch err {
	case nil:
		b.done <- struct{}{}
	default:
		b.errc <- err
	}

	log.Printf("[INFO] Ballot closed key=%s height=%d ballot=%p runtime=%v error='%v'",
		b.fentry.Entry.Key, b.fentry.Entry.Height, b, b.Runtime(), err)
	return nil
}

// Error returns the error the ballot was closed with if any
func (b *Ballot) Error() error {
	return b.err
}

// Runtime returns the amount of time taken for this ballot to complete
func (b *Ballot) Runtime() time.Duration {
	return b.completed.Sub(b.dispatched)
}

func countVotes(arr []string) int {
	var c int
	for _, v := range arr {
		if v != "" {
			c++
		}
	}
	return c
}
