package hexalog

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/log"
)

var (
	errInsufficientPeers = errors.New("insufficient peers")
	errPreviousHash      = errors.New("previous hash mismatch")
)

// Transport implements a Hexalog network transport
type Transport interface {
	GetEntry(host string, key, id []byte, opts *RequestOptions) (*Entry, error)
	ProposeEntry(host string, entry *Entry, opts *RequestOptions) error
	CommitEntry(host string, entry *Entry, opts *RequestOptions) error
	TransferKeylog(host string, key []byte) error
	Register(hlog *Hexalog)
}

// LogStore implements a persistent store for the log
type LogStore interface {
	NewKey(key, locationID []byte) error
	GetKey(key []byte) (KeylogStore, error)
	RemoveKey(key []byte) error
	NewEntry(key []byte) *Entry
	GetEntry(key, id []byte) (*Entry, bool)
	LastEntry(key []byte) *Entry
	Iter(func(key string, locationID []byte))
	AppendEntry(entry *Entry) error
	RollbackEntry(entry *Entry) error
}

// Config holds the configuration for the log.  This is used to initialize the log.
type Config struct {
	Hostname           string
	BroadcastBufSize   int // proposal and commit broadcast buffer
	BallotReapInterval time.Duration
	TTL                time.Duration // ttl for each ballot
	Votes              int           // votes required
	Hasher             Hasher        // hash function generator
}

// DefaultConfig returns a sane set of default configurations.  The default hash function
// used is SHA1
func DefaultConfig(hostname string) *Config {
	return &Config{
		Hostname:           hostname,
		BroadcastBufSize:   16,
		BallotReapInterval: 30 * time.Second,
		TTL:                3 * time.Second,
		Votes:              3,
		Hasher:             &SHA1Hasher{},
	}
}

// Hexalog is the core log that is responsible for consensus, election, serialization and
// all other aspects pertaining to consistency
type Hexalog struct {
	conf *Config
	// Internal fsm.  This wraps the application FSM from the config
	fsm *fsm

	trans Transport
	// Currently active ballots
	mu      sync.RWMutex
	ballots map[string]*Ballot
	// The store containing log entires that are committed, but not necessary applied
	// to the FSM
	store LogStore
	// Propose broadcast channel
	pch chan *RPCRequest
	// Commit broadcast channel
	cch chan *RPCRequest
}

// NewHexalog initializes a new Hexalog and starts the entry broadcaster
func NewHexalog(conf *Config, appFSM FSM, logStore LogStore, stableStore StableStore, trans Transport) (*Hexalog, error) {
	// Init internal FSM that manages the user provided application fsm
	ifsm, err := newFsm(appFSM, stableStore, conf.Hasher)
	if err != nil {
		return nil, err
	}

	hlog := &Hexalog{
		conf:    conf,
		fsm:     ifsm,
		trans:   trans,
		ballots: make(map[string]*Ballot),
		pch:     make(chan *RPCRequest, conf.BroadcastBufSize),
		cch:     make(chan *RPCRequest, conf.BroadcastBufSize),
		store:   logStore,
	}

	// Register Hexalog to the transport to handle RPC requests
	trans.Register(hlog)

	// Start broadcasting
	go hlog.startBroadcastPropose()
	go hlog.startBroadcastCommit()
	go hlog.reapBallots()

	return hlog, nil
}

// New returns a new Entry to be appended to the log.
func (hlog *Hexalog) New(key []byte) *Entry {
	return hlog.store.NewEntry(key)
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
	if entry.Height != prevHeight+1 {
		err = errPreviousHash
		return
	}

	// Check the previous hash
	if bytes.Compare(entry.Previous, lastID) != 0 {
		err = errPreviousHash
	}

	return
}

// Propose proposes an entry to the log.  It votes on a ballot if it exists or creates one
// then votes. If required votes has been reach it also moves to the commit phase.
func (hlog *Hexalog) Propose(entry *Entry, opts *RequestOptions) (*Ballot, error) {
	//
	// TODO: Removed ballot on error close
	//

	if err := hlog.checkOptions(opts); err != nil {
		return nil, err
	}

	// Get our location index in the peerset
	idx, ok := hlog.getSelfIndex(opts.PeerSet)
	if !ok {
		return nil, fmt.Errorf("%s not in PeerSet", hlog.conf.Hostname)
	}
	// Get our location
	loc := opts.PeerSet[idx]

	// Verify and bootstrap entry
	prevHeight, err := hlog.verifyEntry(entry)
	if err != nil {
		return nil, err
	}

	key := string(entry.Key)
	id := entry.Hash(hlog.conf.Hasher.New())

	// Get or create ballot as necessary
	hlog.mu.Lock()
	ballot, ok := hlog.ballots[key]
	if !ok {
		// Create a new ballot and track it.
		fentry := NewFutureEntry(id, entry)
		ballot = newBallot(fentry, hlog.conf.Votes, hlog.conf.TTL)
		hlog.ballots[key] = ballot
		hlog.mu.Unlock()

		// Cast a vote for ourself if we are not the SourceIndex, on top of casting the remote
		// vote taking place below
		if idx != int(opts.SourceIndex) {
			if _, err = ballot.votePropose(id, string(loc.Vnode.Id)); err != nil {
				return nil, err
			}

			// Create a new key if height is zero and we don't have it.
			if prevHeight == 0 {
				if _, er := hlog.store.GetKey(entry.Key); er != nil {
					if err = hlog.store.NewKey(entry.Key, loc.ID); err != nil {
						ballot.close(err)
						return ballot, err
					}
				}
			}

			// Broadcast proposal
			hlog.pch <- &RPCRequest{Entry: entry, Options: opts}
		}

	} else {
		// If the current ballot is closed assume it to be stale and create a new one.
		if ballot.Closed() {
			fentry := NewFutureEntry(id, entry)
			ballot = newBallot(fentry, hlog.conf.Votes, hlog.conf.TTL)
			hlog.ballots[key] = ballot

		}
		hlog.mu.Unlock()
	}

	vid := opts.PeerSet[opts.SourceIndex].Vnode.Id
	pvotes, err := ballot.votePropose(id, string(vid))
	if err != nil {
		return nil, err
	}

	log.Printf("[INFO] Propose host=%s index=%d ballot=%p votes=%d voter=%x",
		hlog.conf.Hostname, opts.SourceIndex, ballot, pvotes, vid)

	if pvotes == 1 {

		// Create a new key if height is 0 and we don't have the key
		if prevHeight == 0 {
			if _, er := hlog.store.GetKey(entry.Key); er != nil {
				if err = hlog.store.NewKey(entry.Key, loc.ID); err != nil {
					ballot.close(err)
					return ballot, err
				}
			}
		}

		hlog.pch <- &RPCRequest{Entry: entry, Options: opts}

	} else if pvotes == hlog.conf.Votes {
		log.Printf("[INFO] Proposal accepted host=%s key=%s", hlog.conf.Hostname, entry.Key)

		if err = hlog.store.AppendEntry(entry); err != nil {

			//
			// TODO: ???
			//

			ballot.close(err)
			return ballot, err
		}

		// Start local commit phase.  We use our index as the voter
		var cvotes int
		cvotes, err = ballot.voteCommit(id, string(opts.PeerSet[idx].Vnode.Id))
		if err == nil {
			if cvotes == 1 {
				// Broadcast commit with new options
				hlog.cch <- &RPCRequest{Entry: entry, Options: opts}
			}

		} else {

			// We rollback here as we appended but the vote failed
			if er := hlog.store.RollbackEntry(entry); er != nil {
				log.Println("[ERROR] Rollback failed:", er)
			}

		}

	}

	return ballot, err
}

func (hlog *Hexalog) reapBallots() {
	for {

		time.Sleep(hlog.conf.BallotReapInterval)

		// Only reap if we have ballots.  This is because reap actually acquires a write lock
		// and would yield better performance
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
}

// reapBallotsOnce aquires a lock and purges all ballots that have been closed returning
// the number of ballots purged
func (hlog *Hexalog) reapBallotsOnce() (c int) {

	hlog.mu.Lock()
	for k, b := range hlog.ballots {
		if !b.Closed() {
			continue
		}

		delete(hlog.ballots, k)
		c++
	}
	hlog.mu.Unlock()

	return
}

// Commit tries to commit an already proposed entry to the log.
func (hlog *Hexalog) Commit(entry *Entry, opts *RequestOptions) (*Ballot, error) {
	//
	// TODO: Removed ballot on error close
	//

	if err := hlog.checkOptions(opts); err != nil {
		return nil, err
	}

	//
	// TODO: verify signature
	//

	// Make sure we have the ballot.  This locking scales better over the long run.
	ballot := hlog.getBallot(entry.Key)
	if ballot == nil {
		return nil, errBallotNotFound
	}

	//
	// TODO: Validate
	//

	//
	// TODO: Rollback if necessary
	//

	vid := opts.PeerSet[opts.SourceIndex].Vnode.Id
	id := entry.Hash(hlog.conf.Hasher.New())

	votes, err := ballot.voteCommit(id, string(vid))
	log.Printf("[DEBUG] Commit host=%s index=%d ballot=%p votes=%d voter=%x",
		hlog.conf.Hostname, opts.SourceIndex, ballot, votes, vid)

	// We do not rollback here as we could have a faulty voter trying to commit without
	// having a proposal.
	if err != nil {
		return ballot, err
	}

	if votes == 1 {
		// Broadcast commit entry
		hlog.cch <- &RPCRequest{Entry: entry, Options: opts}
	} else if votes == hlog.conf.Votes {
		// Queue future entry to be applied to the FSM.
		hlog.fsm.apply(ballot.fentry)
		// Ballot is closed.  Remove ballot and stop tracking
		hlog.removeBallot(entry.Key)
	}

	return ballot, err
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

// startBroadcastCommit starts consuming the commit broadcast channel to broadcast locally
// committed entries to the network as part of voting.  This is mean to be run in a
// go-routine.
func (hlog *Hexalog) startBroadcastCommit() {
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
}

// startBroadcastPropose starts consuming the proposal broadcast channel to broadcast
// locally proposed entries to the network.  This is mean to be run in a go-routine.
func (hlog *Hexalog) startBroadcastPropose() {
	for msg := range hlog.pch {

		if err := hlog.broadcastPropose(msg.Entry, msg.Options); err != nil {
			// Close ballot with error if we still have the ballot and remove it
			hlog.ballotGetClose(msg.Entry.Key, err)
		}

	}
}

// ballotGetClose gets a ballot and closes it with the given error if not already closed
func (hlog *Hexalog) ballotGetClose(key []byte, err error) {
	if ballot := hlog.getBallot(key); ballot != nil {
		ballot.close(err)
	}
}
