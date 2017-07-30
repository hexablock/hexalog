package hexalog

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hexablock/hexaring"
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
func (hlog *Hexalog) append(id []byte, entry *Entry) (fentry *FutureEntry, err error) {

	if err = hlog.store.AppendEntry(entry); err == nil {
		fentry = NewFutureEntry(id, entry)
		hlog.fsm.apply(fentry)
	}

	return
}

func (hlog *Hexalog) verifyEntry(entry *Entry) (prevHeight uint32, err error) {
	//
	// TODO: verify signature
	//

	// Set the default last id to a zero hash
	lastID := make([]byte, hlog.conf.Hasher.New().Size())
	// Try to get the last entry
	last := hlog.store.LastEntry(entry.Key)
	if last != nil {
		prevHeight = last.Height
		lastID = last.Hash(hlog.conf.Hasher.New())
	}

	// TODO: Re-visit
	// Check height
	// if entry.Height != prevHeight+1 {
	// 	err = errPreviousHash
	// 	return
	// }

	// Check the previous hash
	if bytes.Compare(entry.Previous, lastID) != 0 {
		err = errPreviousHash
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

		props, commits := b.Proposals(), b.Commits()
		log.Printf("[DEBUG] Ballot reaped key=%s id=%x proposals=%d commits=%d error='%v'",
			b.fentry.Entry.Key, k, props, commits, b.Error())

		delete(hlog.ballots, k)
		c++
	}
	hlog.mu.Unlock()

	return
}

func (hlog *Hexalog) broadcastPropose(entry *Entry, opts *RequestOptions) error {
	// Get self index in the PeerSet.
	idx, ok := hlog.getSelfIndex(opts.PeerSet)
	if !ok {
		return fmt.Errorf("%s not in PeerSet", hlog.conf.Hostname)
	}

	for _, p := range opts.PeerSet {
		// Do not broadcast to self
		if p.Vnode.Host == hlog.conf.Hostname {
			continue
		}

		host := p.Vnode.Host

		o := opts.CloneWithSourceIndex(int32(idx))
		log.Printf("[DEBUG] Broadcast phase=propose %s -> %s index=%d",
			hlog.conf.Hostname, host, o.SourceIndex)
		if err := hlog.trans.ProposeEntry(host, entry, o); err != nil {
			return err
		}

	}

	return nil
}

func (hlog *Hexalog) broadcastCommit(entry *Entry, opts *RequestOptions) error {
	// Get self index in the PeerSet.
	idx, ok := hlog.getSelfIndex(opts.PeerSet)
	if !ok {
		return fmt.Errorf("%s not in PeerSet", hlog.conf.Hostname)
	}

	for _, p := range opts.PeerSet {
		// Do not broadcast to self
		if p.Vnode.Host == hlog.conf.Hostname {
			continue
		}

		o := opts.CloneWithSourceIndex(int32(idx))
		log.Printf("[DEBUG] Broadcast phase=commit %s -> %s index=%d", hlog.conf.Hostname,
			p.Vnode.Host, o.SourceIndex)

		if err := hlog.trans.CommitEntry(p.Vnode.Host, entry, o); err != nil {
			return err
		}

	}

	return nil
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

func (hlog *Hexalog) checkOptions(opts *RequestOptions) error {
	if opts.PeerSet == nil || len(opts.PeerSet) < hlog.conf.Votes {
		return errInsufficientPeers
	}

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
func (hlog *Hexalog) checkCommitAndAct(currVotes int, ballot *Ballot, key []byte, entry *Entry, opts *RequestOptions) {
	if currVotes == 1 {
		// Broadcast commit entry
		hlog.cch <- &RPCRequest{Entry: entry, Options: opts}

	} else if currVotes == hlog.conf.Votes {

		log.Printf("[DEBUG] Commit accepted host=%s key=%s height=%d ", hlog.conf.Hostname, entry.Key, entry.Height)
		// Queue future entry to be applied to the FSM.
		hlog.fsm.apply(ballot.fentry)
		// Close the ballot after we've submitted to the fsm
		ballot.close(nil)
		// Ballot is closed.  Remove ballot and stop tracking
		hlog.removeBallot(key)
	}

}

// broadcastCommits starts consuming the commit broadcast channel to broadcast locally
// committed entries to the network as part of voting.  This is mean to be run in a
// go-routine.
func (hlog *Hexalog) broadcastCommits() {
	for msg := range hlog.cch {

		en := msg.Entry

		err := hlog.broadcastCommit(en, msg.Options)
		if err == nil {
			continue
		}

		id := en.Hash(hlog.conf.Hasher.New())
		hlog.ballotGetClose(id, err)

		// Rollback the entry.
		if er := hlog.store.RollbackEntry(en); er != nil {
			log.Printf("[ERROR] Failed to rollback key=%s height=%d id=%x error='%v'", en.Key, en.Height, id, er)
		}

	}

	log.Println("[INFO] Commit broadcaster shutdown!")
	// Notify that we have shutdown
	hlog.shutdownCh <- struct{}{}
}

// broadcastProposals starts consuming the proposal broadcast channel to broadcast
// locally proposed entries to the network.  This is mean to be run in a go-routine.
func (hlog *Hexalog) broadcastProposals() {
	for msg := range hlog.pch {

		if err := hlog.broadcastPropose(msg.Entry, msg.Options); err != nil {

			id := msg.Entry.Hash(hlog.conf.Hasher.New())
			hlog.ballotGetClose(id, err)

		}

	}

	log.Println("[INFO] Proposal broadcaster shutdown!")
	// Notify that we have shutdown
	hlog.shutdownCh <- struct{}{}
}
