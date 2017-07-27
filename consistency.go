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

	// Check height
	// if entry.Height != prevHeight+1 {
	// 	err = errPreviousHash
	// 	return
	// }

	//
	// TODO:
	// Find how far we are behind if we are actually behind.
	// Get the entries and start appending
	//

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

		log.Printf("[DEBUG] Ballot reaped key=%s error='%v'", k, b.Error())
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

// broadcastCommits starts consuming the commit broadcast channel to broadcast locally
// committed entries to the network as part of voting.  This is mean to be run in a
// go-routine.
func (hlog *Hexalog) broadcastCommits() {
	for msg := range hlog.cch {
		err := hlog.broadcastCommit(msg.Entry, msg.Options)
		if err == nil {
			continue
		}
		hlog.ballotGetClose(msg.Entry.Key, err)

		// Rollback the entry.
		if er := hlog.store.RollbackEntry(msg.Entry); er != nil {
			log.Println("[ERROR]", er)
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
			// Close ballot with error if we still have the ballot and remove it
			hlog.ballotGetClose(msg.Entry.Key, err)
		}

	}

	log.Println("[INFO] Proposal broadcaster shutdown!")
	// Notify that we have shutdown
	hlog.shutdownCh <- struct{}{}
}

//
// func (hlog *Hexalog) healKeys() {
// 	for r := range hlog.hch {
// 		e := r.Entry
// 		opts := r.Options
// 		loc := opts.SourcePeer()
//
// 		keylog, err := hlog.store.GetKey(e.Key)
// 		if err != nil {
// 			if keylog, err = hlog.store.NewKey(e.Key, loc.ID); err != nil {
// 				log.Printf("[ERROR] Heal failed to create key key=%s error='%v'", e.Key, err)
// 				continue
// 			}
// 		}
//
// 		log.Printf("[DEBUG] Heal key=%s height=%d prev=%x", e.Key, e.Height, e.Previous)
//
// 		for _, peer := range opts.PeerSet {
// 			// Skip ourself
// 			if peer.Vnode.Host == hlog.conf.Hostname {
// 				continue
// 			}
//
// 			// Get last entry we have locally as it may have changed
// 			last := keylog.LastEntry()
// 			if last == nil {
// 				last = &Entry{Key: e.Key}
// 			}
//
// 			// Try to get as many entries as we can from a peer
// 			if _, err = hlog.trans.FetchKeylog(peer.Vnode.Host, last); err != nil {
// 				log.Printf("[ERROR] Fetch key log failed host=%s error='%v'", peer.Vnode.Host, err)
// 			}
//
// 		}
//
// 	}
//
// 	log.Println("[INFO] Healer shutdown!")
// 	hlog.shutdownCh <- struct{}{}
// }
