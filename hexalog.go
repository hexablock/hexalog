package hexalog

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexablock/log"
)

var (
	errInsufficientPeers = errors.New("insufficient peers")
	errPreviousHash      = errors.New("previous hash mismatch")
)

// Transport implements a Hexalog network transport
type Transport interface {
	// Gets an entry from a remote host
	GetEntry(host string, key, id []byte, opts *RequestOptions) (*Entry, error)
	// Proposes an entry on the remote host
	ProposeEntry(host string, entry *Entry, opts *RequestOptions) error
	// Commits an entry on the remote host
	CommitEntry(host string, entry *Entry, opts *RequestOptions) error
	// Transfers a key to the remote host
	TransferKeylog(host string, key []byte) error
	// Gets all entries for a key starting at entry
	FetchKeylog(host string, entry *Entry) (*FutureEntry, error)
	// Registers the log when available
	Register(hlog *Hexalog)
	// Shutdown the transport closing outbound connections
	Shutdown()
}

// LogStore implements a persistent store for the log
type LogStore interface {
	NewKey(key, locationID []byte) (KeylogStore, error)
	GetKey(key []byte) (KeylogStore, error)
	RemoveKey(key []byte) error
	NewEntry(key []byte) *Entry
	GetEntry(key, id []byte) (*Entry, error)
	LastEntry(key []byte) *Entry
	Iter(func(key string, locationID []byte))
	AppendEntry(entry *Entry) error
	RollbackEntry(entry *Entry) error
}

// Config holds the configuration for the log.  This is used to initialize the log.
type Config struct {
	Hostname           string
	HealBufSize        int // Buffer size for heal requests
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
		BroadcastBufSize:   32,
		HealBufSize:        32,
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
	// Underlying transport
	trans Transport
	// Currently active ballots
	mu      sync.RWMutex
	ballots map[string]*Ballot
	// The store containing log entires that are committed, but not necessary applied
	// to the FSM
	store LogStore
	// Propose broadcast channel to broadcast proposals to the network peer set
	pch chan *RPCRequest
	// Commit broadcast channel to broadcast commits to the network peer set
	cch chan *RPCRequest
	// Channel for heal requests.  When previous hash mismatches occur, the log will send a
	// request down this channel to allow applications to try to recover. This is usually
	// the case when a keylog falls behind.
	hch chan *RPCRequest

	shutdown int32
	// This is initialized with a static size of 3 as we launch 3 go-routines.  The heal
	// queue is not part of this number
	shutdownCh chan struct{}
}

// NewHexalog initializes a new Hexalog and starts the entry broadcaster
func NewHexalog(conf *Config, appFSM FSM, logStore LogStore, stableStore StableStore, trans Transport) (*Hexalog, error) {
	// Init internal FSM that manages the user provided application fsm
	ifsm, err := newFsm(appFSM, stableStore, conf.Hasher)
	if err != nil {
		return nil, err
	}

	hlog := &Hexalog{
		conf:       conf,
		fsm:        ifsm,
		trans:      trans,
		ballots:    make(map[string]*Ballot),
		pch:        make(chan *RPCRequest, conf.BroadcastBufSize),
		cch:        make(chan *RPCRequest, conf.BroadcastBufSize),
		hch:        make(chan *RPCRequest, conf.HealBufSize),
		store:      logStore,
		shutdownCh: make(chan struct{}, 3),
	}

	// Register Hexalog to the transport to handle RPC requests
	trans.Register(hlog)

	// Start broadcasting
	go hlog.broadcastProposals()
	go hlog.broadcastCommits()
	go hlog.reapBallots()

	return hlog, nil
}

// Heal returns a readonly channel containing information on keys that need healing.  This
// is consumed by the client application to take action when unhealthy keys are found in
// order to repair them.
func (hlog *Hexalog) Heal() <-chan *RPCRequest {
	return hlog.hch
}

// New returns a new Entry to be appended to the log.
func (hlog *Hexalog) New(key []byte) *Entry {
	return hlog.store.NewEntry(key)
}

// Propose proposes an entry to the log.  It votes on a ballot if it exists or creates one
// then votes. If required votes has been reach it also moves to the commit phase.
func (hlog *Hexalog) Propose(entry *Entry, opts *RequestOptions) (*Ballot, error) {
	// Check request options
	if err := hlog.checkOptions(opts); err != nil {
		return nil, err
	}

	// Get our location index in the peerset
	idx, ok := hlog.getSelfIndex(opts.PeerSet)
	if !ok {
		return nil, fmt.Errorf("%s not in peer set", hlog.conf.Hostname)
	}

	// our location
	loc := opts.PeerSet[idx]
	// entry id
	id := entry.Hash(hlog.conf.Hasher.New())

	// Verify entry
	prevHeight, err := hlog.verifyEntry(entry)
	if err != nil {
		// Only try to heal if the new height is > then the current one
		if entry.Height > prevHeight {
			hlog.hch <- &RPCRequest{
				ID:      id,    // entry hash id
				Entry:   entry, // entry itself
				Options: opts,  // participating peers
			}
		}

		//
		// TODO:
		// Signal a retry
		// Do not close ballot
		//

		hlog.ballotGetClose(entry.Key, err)
		return nil, err
	}

	key := string(entry.Key)

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
					if _, err = hlog.store.NewKey(entry.Key, loc.ID); err != nil {
						ballot.close(err)
						return ballot, err
					}
				}
			}

			// Broadcast proposal
			hlog.pch <- &RPCRequest{Entry: entry, Options: opts}
		}

	} else {
		hlog.mu.Unlock()
	}

	vid := opts.SourcePeer().Vnode.Id
	pvotes, err := ballot.votePropose(id, string(vid))
	if err == errBallotClosed {
		// TODO: This is a temporary fix needs to be addressed elsewhere
		hlog.removeBallot(entry.Key)
	}

	log.Printf("[INFO] Propose host=%s key=%s index=%d ballot=%p votes=%d voter=%x error='%v'",
		hlog.conf.Hostname, entry.Key, opts.SourceIndex, ballot, pvotes, vid, err)

	if err != nil {
		return ballot, err
	}

	if pvotes == 1 {

		// Create a new key if height is 0 and we don't have the key
		if prevHeight == 0 {
			if _, er := hlog.store.GetKey(entry.Key); er != nil {
				if _, err = hlog.store.NewKey(entry.Key, loc.ID); err != nil {
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

			// Take action if we have the required commits
			hlog.checkCommitAndAct(cvotes, ballot, entry, opts)

		} else {

			// We rollback here as we appended but the vote failed
			log.Printf("[DEBUG] Rolling back key=%s height=%d id=%x", entry.Key, entry.Height, id)
			if er := hlog.store.RollbackEntry(entry); er != nil {
				log.Printf("[ERROR] Rollback failed key=%s height=%d error='%v'", entry.Key, entry.Height, er)
			}

		}

	}

	return ballot, err
}

// Commit tries to commit an already proposed entry to the log.
func (hlog *Hexalog) Commit(entry *Entry, opts *RequestOptions) (*Ballot, error) {
	// Check request options
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
	log.Printf("[INFO] Commit host=%s key=%s index=%d ballot=%p votes=%d voter=%x error='%v'",
		hlog.conf.Hostname, entry.Key, opts.SourceIndex, ballot, votes, vid, err)

	if err == errBallotClosed {
		// TODO: This is a temporary fix needs to be addressed elsewhere
		hlog.removeBallot(entry.Key)
	}

	// We do not rollback here as we could have a faulty voter trying to commit without
	// having a proposal.
	if err != nil {
		return ballot, err
	}

	hlog.checkCommitAndAct(votes, ballot, entry, opts)

	return ballot, nil
}

// Shutdown signals a shutdown and waits for all go-routines to exit before returning.  It
// will take atleast the amount of time specified as the ballot reap interval as shutdown
// for the ballot reaper is checked at the top of the loop
func (hlog *Hexalog) Shutdown() {
	atomic.StoreInt32(&hlog.shutdown, 1)

	close(hlog.pch)
	close(hlog.cch)
	close(hlog.hch)

	// Wait for echo go-routine to exit their loop
	for i := 0; i < 3; i++ {
		<-hlog.shutdownCh
	}

	log.Println("[INFO] Hexalog shutdown complete!")
}
