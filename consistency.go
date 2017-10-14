package hexalog

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

//
// This file contains hexalog functions that handle various aspects of maintaining
// consistency.
//

// append appends the entry to the log.  If it succeeds is submits the entry to be applied
// to the FSM otherwise returns an error.  This call bypasses the voting process and tries
// to append to the log directly.  This is only to be used during rebalancing and healing.
// These entries would already have been accepted by the network and thus be valid.
func (hlog *Hexalog) append(id []byte, entry *hexatype.Entry) (fentry *FutureEntry, err error) {

	if err = hlog.store.AppendEntry(entry); err == nil {
		fentry = NewFutureEntry(id, entry)
		hlog.fsm.apply(fentry)
	}

	return
}

// verifyEntry verifies an entry. It returns an errPreviousHash is the previous hash does
// not match
func (hlog *Hexalog) verifyEntry(entry *hexatype.Entry) (prevHeight uint32, err error) {
	//
	// TODO: verify signature
	//

	// Check the marker.  Continue if key not found as it may be the first entry for the key
	kl, err := hlog.store.index.GetKey(entry.Key)
	if err != nil {
		if err != hexatype.ErrKeyNotFound {
			return 0, err
		}
		err = nil
		// Continue if key not found as it may be the first entry for the key
	} else if kl.Marker() != nil {
		return 0, fmt.Errorf("key degraded")
	}

	// Set the default last id to a zero hash
	lastID := make([]byte, hlog.conf.Hasher.New().Size())
	// Try to get the last entry
	last := hlog.store.LastEntry(entry.Key)
	if last != nil {
		prevHeight = last.Height
		lastID = last.Hash(hlog.conf.Hasher.New())
	}

	// TODO: CHeck keylog for marker.  if present fail the verification as we are not
	// allowed to write via voting until the marker has been cleared.

	// TODO: Re-visit
	// Check height
	// if entry.Height != prevHeight+1 {
	// 	err = errPreviousHash
	// 	return
	// }

	// Check the previous hash
	if bytes.Compare(entry.Previous, lastID) != 0 {
		err = hexatype.ErrPreviousHash
	}

	return
}

func (hlog *Hexalog) reapBallots() {
	for {

		time.Sleep(hlog.conf.BallotReapInterval)

		if atomic.LoadInt32(&hlog.shutdown) == 1 {
			break
		}
		// Only reap if we have ballots.  This is because reap actually acquires a write lock
		// and would yield better performance this way
		var reap bool
		hlog.mu.RLock()
		if len(hlog.ballots) > 0 {
			reap = true
		}
		hlog.mu.RUnlock()

		if reap {
			c := hlog.reapBallotsOnce()
			log.Printf("[DEBUG] Ballots reaped: %d", c)
		}

	}

	log.Println("[INFO] Ballot reaper shutdown!")
	// Notify that we have shutdown
	hlog.shutdownCh <- struct{}{}
}

// reapBallotsOnce aquires a lock and purges all ballots that have been closed returning
// the number of ballots purged
func (hlog *Hexalog) reapBallotsOnce() (c int) {

	hlog.mu.Lock()
	for k, b := range hlog.ballots {
		if !b.Closed() {
			continue
		}

		// props, commits := b.Proposals(), b.Commits()
		// log.Printf("[DEBUG] Ballot reaped key=%s id=%x proposals=%d commits=%d error='%v'",
		//	b.fentry.Entry.Key, k, props, commits, b.Error())

		delete(hlog.ballots, k)
		c++
	}
	hlog.mu.Unlock()

	return
}

// getBallot gets a ballot for a key.  It returns nil if a ballot does not exist
func (hlog *Hexalog) getBallot(key []byte) *Ballot {
	hlog.mu.RLock()
	ballot, _ := hlog.ballots[string(key)]
	hlog.mu.RUnlock()
	return ballot
}

func (hlog *Hexalog) removeBallot(key []byte) {
	k := string(key)

	hlog.mu.Lock()
	if _, ok := hlog.ballots[k]; ok {
		delete(hlog.ballots, k)
	}
	hlog.mu.Unlock()
}

func (hlog *Hexalog) checkOptions(opts *hexatype.RequestOptions) error {
	if opts.PeerSet == nil || len(opts.PeerSet) < hlog.conf.Votes {
		return hexatype.ErrInsufficientPeers
	}
	hlog.ltime.Witness(hexatype.LamportTime(opts.LTime))
	opts.LTime = uint64(hlog.ltime.Time())

	return nil
}

// getSelfIndex gets the index of this node in the PeerSet
func (hlog *Hexalog) getSelfIndex(peerset []*hexaring.Location) (int, bool) {
	for i, p := range peerset {
		if p.Vnode.Host == hlog.conf.Hostname {
			return i, true
		}
	}
	return -1, false
}

// ballotGetClose gets a ballot and closes it with the given error if not already closed
func (hlog *Hexalog) ballotGetClose(key []byte, err error) {
	if ballot := hlog.getBallot(key); ballot != nil {
		ballot.close(err)
	}
}

// checkVoteAct checks the number of commits and takes the appropriate action
func (hlog *Hexalog) checkCommitAndAct(currVotes int, ballot *Ballot, key []byte, entry *hexatype.Entry, opts *hexatype.RequestOptions) {
	if currVotes == 1 {
		// Broadcast commit entry
		hlog.cch <- &hexatype.ReqResp{Entry: entry, Options: opts}
		hlog.ltime.Increment()

	} else if currVotes == hlog.conf.Votes {

		if err := hlog.store.AppendEntry(entry); err != nil {
			ballot.close(err)
			return
		}

		log.Printf("[DEBUG] Commit accepted host=%s key=%s height=%d ", hlog.conf.Hostname, entry.Key, entry.Height)
		// Queue future entry to be applied to the FSM.
		hlog.fsm.apply(ballot.fentry)
		// Close the ballot after we've submitted to the fsm
		ballot.close(nil)
		// Ballot is closed.  Remove ballot and stop tracking
		//hlog.removeBallot(key)
		hlog.ltime.Increment()
	}

	// Do nothing as it may be a repetative vote
}
