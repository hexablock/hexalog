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
	errBallotTimedOut     = errors.New("ballot timed out")
	errBallotClosed       = errors.New("ballot closed")
	errBallotNotFound     = errors.New("ballot not found")
	errProposalNotFound   = errors.New("proposal not found")
	errNotEnoughProposals = errors.New("not enough proposals")
	errVoterAlreadyVoted  = errors.New("voter already voted")
	errInvalidVoteID      = errors.New("invalid vote id")
)

// Ballot holds information to execute a singular distributed operation ensuring consistency.  It is
// primarily used to track the state of proposal and commit votes to appropriately handle an operation.
type Ballot struct {
	// Future for the Entry being voted on.  This is used to know and wait on when an actual
	// entry is applied to the application defined FSM
	fentry *FutureEntry
	// TTL for the ballot. The ballot is closed once the ttl has been reached
	ttl time.Duration
	// TTL timer
	timer *time.Timer
	// Map to count proposed votes
	pmu      sync.RWMutex
	proposed map[string]struct{}
	// Map to count committed votes
	cmu       sync.RWMutex
	committed map[string]struct{}
	// Votes required for both proposed and committed
	votes int
	// Error channel used to wait for completion
	err chan error
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
		proposed:  make(map[string]struct{}),
		committed: make(map[string]struct{}),
		err:       make(chan error, requiredVotes),
		done:      make(chan struct{}, requiredVotes),
	}
}

// Wait blocks until voting on the ballot is complete
func (b *Ballot) Wait() error {
	var err error

	select {
	case <-b.done:
	case err = <-b.err:
	}

	return err
}

// votePropose submit a vote for the propose phase.  It takes an Entry hash id and a voter
// as parameters.  If the voter has already voted it simply returns the proposed votes
// with no error.  An error is returned if the supplied id does not match the the entry id
// of the ballot.
func (b *Ballot) votePropose(id []byte, voter string) (int, error) {
	if atomic.LoadInt32(&b.closed) == 1 {
		return -1, errBallotClosed
	}
	// Check the entry id to make sure it matches the ballot.
	if bytes.Compare(b.fentry.ID(), id) != 0 {
		return -1, errInvalidVoteID
	}

	b.pmu.RLock()
	proposals := len(b.proposed)

	// Check if voter already voted.
	if _, ok := b.proposed[voter]; ok {
		b.pmu.RUnlock()
		return proposals, nil
	}
	b.pmu.RUnlock()

	// Return if required proposals are reached
	if proposals == b.votes {
		return proposals, nil
	}

	b.pmu.Lock()
	b.proposed[voter] = struct{}{}
	proposals = len(b.proposed)
	b.pmu.Unlock()

	// Initiaze timer if this is the first proposal for ballot.  Do not set ttl if it is
	// the same voter,trying to vote again.
	if proposals == 1 {
		b.setTTL()
	}

	return proposals, nil
}

// voteCommit submits a commit vote.  If the voter has already voted it simply
// returns the committed votes with no error
func (b *Ballot) voteCommit(id []byte, voter string) (int, error) {
	if atomic.LoadInt32(&b.closed) == 1 {
		return -1, errBallotClosed
	}
	// Check the entry id to make sure it matches the ballot.
	if bytes.Compare(b.fentry.ID(), id) != 0 {
		return -1, errInvalidVoteID
	}

	// Check if the voter has a proposal
	b.pmu.RLock()
	if _, ok := b.proposed[voter]; !ok {
		b.pmu.RUnlock()
		return -1, errProposalNotFound
	}
	b.pmu.RUnlock()

	// Make sure voter hasn't already voted
	b.cmu.RLock()
	if _, ok := b.committed[voter]; ok {
		defer b.cmu.RUnlock()
		return len(b.committed), nil
	}
	b.cmu.RUnlock()

	// Add commit vote
	b.cmu.Lock()
	b.committed[voter] = struct{}{}
	commits := len(b.committed)
	b.cmu.Unlock()

	// Check if we have enough commit votes to close the ballot.
	if commits == b.votes {
		b.close(nil)
	}

	return commits, nil
}

// setTTL starts the counter to appropriately expire the ballot
func (b *Ballot) setTTL() {
	log.Printf("[DEBUG] Ballot opened %p", b)
	// Setup ballot expiration
	b.timer = time.AfterFunc(b.ttl, func() {
		atomic.StoreInt32(&b.closed, 1)
		b.err <- errBallotTimedOut
	})
}

// Closed returns whether or not a ballot has been closed
func (b *Ballot) Closed() bool {
	return atomic.LoadInt32(&b.closed) == 1
}

// close the ballot stopping the timer and writing err to the done chan.
func (b *Ballot) close(err error) error {
	if b.Closed() {
		return errBallotClosed
	}

	b.timer.Stop()
	atomic.StoreInt32(&b.closed, 1)

	switch err {
	case nil:
		b.done <- struct{}{}
	default:
		b.err <- err
	}
	//log.Printf("[DEBUG] Ballot closed: ballot=%p errors=%d completed=%d msg='%v'", b, len(b.err), len(b.done), err)
	return nil
}
