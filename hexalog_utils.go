package hexalog

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

// check if our last matches the other last.  if not pull from the given host
func (hlog *Hexalog) checkLastEntryOrPull(host string, key []byte, otherLast []byte) error {
	var (
		h = hlog.conf.Hasher()
		// local last entry
		last *Entry
		// local last id
		lid []byte
	)

	// Get local keylog
	keylog, err := hlog.store.GetKey(key)
	if err != nil {
		// Create new key
		if err == hexatype.ErrKeyNotFound {
			if keylog, err = hlog.store.NewKey(key); err != nil {
				return err
			}
			defer keylog.Close()

			last = &Entry{Key: key}
			lid = make([]byte, h.Size())
		} else {
			return err
		}

	} else {
		defer keylog.Close()

		if last = keylog.LastEntry(); last == nil {
			last = &Entry{Key: key}
			lid = make([]byte, h.Size())
		} else {
			lid = last.Hash(h)
		}

	}

	// Fetch if there is a mismatch.
	if bytes.Compare(lid, otherLast) != 0 {
		_, er := hlog.trans.PullKeylog(host, last, nil)
		err = mergeErrors(err, er)
	}

	return err
}

// append appends the entry to the log.  If it succeeds is submits the entry to be applied
// to the FSM otherwise returns an error.  This call bypasses the voting process and tries
// to append to the log directly.  This is only to be used during rebalancing and healing.
// These entries would already have been accepted by the network and thus be valid.
func (hlog *Hexalog) append(id []byte, entry *Entry) (fentry *FutureEntry, err error) {

	if err = hlog.store.AppendEntry(entry); err == nil {
		fentry = NewFutureEntry(id, entry)
		hlog.fsm.apply(fentry)
	}

	return
}

// verifyEntry verifies an entry. It returns an errPreviousHash is the previous hash does
// not match
func (hlog *Hexalog) verifyEntry(entry *Entry) (prevHeight uint32, err error) {
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
	} else {
		defer kl.Close()
		if kl.Marker() != nil {
			return 0, hexatype.ErrKeyDegraded
		}
	}

	// Set the default last id to a zero hash
	lastID := make([]byte, hlog.conf.hashSize)
	// Try to get the last entry
	last := hlog.store.LastEntry(entry.Key)
	if last != nil {
		prevHeight = last.Height
		lastID = last.Hash(hlog.conf.Hasher())
	}

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

// checkOptions checks if sufficient peers have been provided.  On success
// it witnesses the lamport clock and sets the new lamport time on the options
func (hlog *Hexalog) checkOptions(opts *RequestOptions) error {
	if opts.PeerSet == nil || len(opts.PeerSet) < hlog.conf.Votes {
		return hexatype.ErrInsufficientPeers
	}

	hlog.conf.LamportClock.Witness(hexatype.LamportTime(opts.LTime))
	opts.LTime = uint64(hlog.conf.LamportClock.Time())

	return nil
}

// getSelfIndex gets the index of this node in the PeerSet
func (hlog *Hexalog) getSelfIndex(peerset []*Participant) (int, bool) {
	for i, p := range peerset {
		if p.Host == hlog.conf.Hostname {
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
func (hlog *Hexalog) checkCommitAndAct(currVotes int, ballot *Ballot, key []byte, entry *Entry, opts *RequestOptions) {
	if currVotes == 1 {
		// Broadcast commit entry
		hlog.cch <- &ReqResp{Entry: entry, Options: opts}
		hlog.conf.LamportClock.Increment()

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
		hlog.conf.LamportClock.Increment()
	}

	// Do nothing as it may be a repetative vote
}

func (hlog *Hexalog) upsertKeyAndBroadcast(prevHeight uint32, entry *Entry, opts *RequestOptions) error {
	if prevHeight == 0 {

		kli, err := hlog.store.NewKey(entry.Key)
		if err == nil {
			kli.Close()
		} else if err != hexatype.ErrKeyExists {
			// Ignore key exists error
			return err
		}

	}

	// Broadcast proposal
	hlog.pch <- &ReqResp{Entry: entry, Options: opts}
	hlog.conf.LamportClock.Increment()

	return nil
}

func mergeErrors(e1, e2 error) (err error) {
	if e1 != nil && e2 != nil {
		return fmt.Errorf("%v; %v", e1, e2)
	} else if e1 != nil {
		return e1
	}
	return e2
}
