package hexalog

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"golang.org/x/net/context"

	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

// Stats contains various pieces of information regarding the current
// state of the log
type Stats struct {
	Keys      int64
	Entries   int64
	Ballots   int
	Proposals int
	Commits   int
	Heals     int
}

// Transport implements a Hexalog network transport
type Transport interface {
	// New entry from the remote host
	NewEntry(host string, key []byte, opts *RequestOptions) (*Entry, error)
	// Gets an entry from a remote host
	GetEntry(host string, key, id []byte, opts *RequestOptions) (*Entry, error)
	// Get last entry for the key
	LastEntry(host string, key []byte, opts *RequestOptions) (*Entry, error)

	// Proposes an entry on the remote host
	ProposeEntry(ctx context.Context, host string, entry *Entry, opts *RequestOptions) (*ReqResp, error)

	// Commits an entry on the remote host.  This is not directly called by the
	// user
	CommitEntry(ctx context.Context, host string, entry *Entry, opts *RequestOptions) error

	// Sends a complete key log to the remote host based on what the remote
	// has
	PushKeylog(host string, key []byte, opts *RequestOptions) error

	// Gets all entries from the remote host for a key starting at entry
	PullKeylog(host string, entry *Entry, opts *RequestOptions) (*FutureEntry, error)

	// Returns a stream of keys used to seed a new or partial log
	GetSeedKeys(host string) (*KeySeedStream, error)

	// Makes the log available to the transport
	Register(hlog *Hexalog)

	// Shutdown the transport closing outbound connections
	Shutdown()
}

// Hexalog is the core log that is responsible for consensus, voting, election,
// serialization and all other aspects pertaining to the log consistency
type Hexalog struct {
	conf *Config

	// Internal fsm.  This wraps the application FSM from the config
	fsm *fsm

	// Underlying transport
	trans Transport

	// The store containing log entires that are committed, but not necessary
	// applied to the FSM
	store *LogStore

	// Currently active ballots
	mu      sync.RWMutex
	ballots map[string]*Ballot

	// Propose broadcast channel to broadcast proposals to the network peer set
	pch chan *ReqResp

	// Commit broadcast channel to broadcast commits to the network peer set
	cch chan *ReqResp

	// Channel for heal requests.  When previous hash mismatches occur, the log
	// will send a request down this channel to allow applications to try to
	// recover. This is usually the case when a keylog falls behind.
	hch chan *ReqResp

	// Gets set when once a shutdown is signalled
	shutdown int32

	// This is initialized with a static size of 3 as we launch 3 go-routines.
	// The heal queue is not part of this number
	shutdownCh chan struct{}
}

// NewHexalog initializes a new Hexalog and starts the entry broadcaster.  This
// call will block until fsm checks are performed. ie. the log is caught up based
// on the stable store
func NewHexalog(conf *Config, appFSM FSM, logstore *LogStore, stableStore StableStore,
	trans Transport) (*Hexalog, error) {

	conf.hashSize = conf.Hasher().Size()

	// Init internal FSM that manages the user provided application fsm
	ifsm, err := newFsm(appFSM, stableStore, logstore, conf.Hasher)
	if err != nil {
		return nil, err
	}

	log.Printf("[INFO] Hexalog store type='stable' name='%s'", stableStore.Name())

	hlog := &Hexalog{
		conf:       conf,
		fsm:        ifsm,
		trans:      trans,
		ballots:    make(map[string]*Ballot),
		pch:        make(chan *ReqResp, conf.BroadcastBufSize),
		cch:        make(chan *ReqResp, conf.BroadcastBufSize),
		hch:        make(chan *ReqResp, conf.HealBufSize),
		store:      logstore,
		shutdownCh: make(chan struct{}, 4),
	}

	// Check to make sure the fsm has all the entries in the log applied.  If
	// not then submit the remaining entries to be applied to the fsm.
	if err = hlog.fsm.check(); err != nil {
		return nil, err
	}

	// Start
	hlog.start()

	return hlog, nil
}

func (hlog *Hexalog) start() {
	// Increment LamportClock on start
	hlog.conf.LamportClock.Increment()

	// Register Hexalog to the transport to handle RPC requests
	hlog.trans.Register(hlog)

	go hlog.broadcastProposals()
	go hlog.broadcastCommits()
	go hlog.healKeys()
	go hlog.reapBallots()
}

// Stats returns internal information of the log regarding its current activity
func (hlog *Hexalog) Stats() *Stats {
	stats := hlog.store.Stats()
	stats.Ballots = len(hlog.ballots)
	stats.Proposals = len(hlog.pch)
	stats.Commits = len(hlog.cch)
	stats.Heals = len(hlog.hch)

	return stats
}

// New returns a new Entry to be appended to the log for the given key.
func (hlog *Hexalog) New(key []byte) *Entry {
	entry := hlog.store.NewEntry(key)
	entry.LTime = uint64(hlog.conf.LamportClock.Time())
	return entry
}

// Propose proposes an entry to the log.  It votes on a ballot if it exists or
// creates one then votes. If required votes has been reach it also moves to the
// commit phase.
func (hlog *Hexalog) Propose(entry *Entry, opts *RequestOptions) (*Ballot, error) {
	// Check request options
	if err := hlog.checkOptions(opts); err != nil {
		return nil, err
	}

	// Get our location index in the peerset and location
	idx, ok := hlog.getSelfIndex(opts.PeerSet)
	if !ok {
		return nil, fmt.Errorf("%s not in peer set", hlog.conf.Hostname)
	}
	loc := opts.PeerSet[idx]

	// entry id
	id := entry.Hash(hlog.conf.Hasher())

	// Verify proposed entry
	prevHeight, err := hlog.verifyEntry(entry)
	if err != nil {
		// Check for heal if previous hash mismatch or a degraded key i.e.
		// marked key
		if err == hexatype.ErrPreviousHash || err == hexatype.ErrKeyDegraded {
			// Try to heal if the new height is > then the current one.  This
			// prevents an infinite retry. If the height <= we do nothing
			if entry.Height > prevHeight {
				// Submit heal request
				hlog.hch <- &ReqResp{
					ID:      id,    // entry hash id
					Entry:   entry, // entry itself
					Options: opts,  // participating peers
				}
				// Return here to allow for a retry.
				return nil, err
			}

		}

		hlog.ballotGetClose(id, err)
		return nil, err
	}

	key := string(id)

	// Get or create ballot as necessary
	hlog.mu.Lock()
	ballot, ok := hlog.ballots[key]
	if !ok {
		// Create a new ballot and track it.
		fentry := NewFutureEntry(id, entry)
		ballot = newBallot(fentry, hlog.conf.Votes, hlog.conf.TTL)
		hlog.ballots[key] = ballot
		hlog.mu.Unlock()

		// Cast a vote for ourself if we are not the SourceIndex, on top of
		// casting the remote vote taking place below
		if idx != int(opts.SourceIndex) {
			if _, err = ballot.votePropose(id, loc.Host); err != nil {
				return nil, err
			}

			// Create a new key if height is zero. We ignore the error as it may
			// already have been created
			if err = hlog.upsertKeyAndBroadcast(prevHeight, entry, opts); err != nil {
				ballot.close(err)
				return nil, err
			}

		}

	} else {
		hlog.mu.Unlock()
	}

	vid := opts.SourcePeer().Host
	pvotes, err := ballot.votePropose(id, vid)

	log.Printf("[DEBUG] Propose ltime=%d host=%s key=%s index=%d ballot=%p votes=%d voter=%x error='%v'",
		opts.LTime, hlog.conf.Hostname, entry.Key, opts.SourceIndex, ballot, pvotes, vid, err)

	if err != nil {
		return ballot, err
	}

	if pvotes == 1 {
		// Create a new key if height is 0 and we don't have the key.  We ignore
		// the error as it may already have been created.
		if err = hlog.upsertKeyAndBroadcast(prevHeight, entry, opts); err != nil {
			ballot.close(err)
			return nil, err
		}

	} else if pvotes == hlog.conf.Votes {
		log.Printf("[DEBUG] Proposal accepted host=%s key=%s", hlog.conf.Hostname, entry.Key)
		// Start local commit phase.  We use our index as the voter
		var cvotes int
		cvotes, err = ballot.voteCommit(id, opts.PeerSet[idx].Host)
		if err == nil {
			// Take action if we have the required commits by appending the log
			// entry and calling app fsm.Apply
			hlog.checkCommitAndAct(cvotes, ballot, id, entry, opts)

		} else {

			// We rollback here as we appended but the vote failed
			log.Printf("[INFO] Rolling back key=%s height=%d id=%x",
				entry.Key, entry.Height, id)
			if er := hlog.store.RollbackEntry(entry); er != nil {
				log.Printf("[ERROR] Rollback failed key=%s height=%d error='%v'",
					entry.Key, entry.Height, er)
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

	id := entry.Hash(hlog.conf.Hasher())
	// Make sure we have the ballot for the entry id
	ballot := hlog.getBallot(id)
	if ballot == nil {
		return nil, errBallotNotFound
	}

	//
	// TODO: Verify & Validate
	//

	sloc := opts.SourcePeer()
	vid := sloc.Host
	votes, err := ballot.voteCommit(id, vid)
	log.Printf("[DEBUG] Commit ltime=%d host=%s key=%s index=%d ballot=%p votes=%d voter=%x error='%v'",
		opts.LTime, hlog.conf.Hostname, entry.Key, opts.SourceIndex, ballot, votes, vid, err)

	// We do not rollback here as we could have a faulty voter trying to commit
	// without having a proposal.
	if err != nil {
		return ballot, err
	}

	hlog.checkCommitAndAct(votes, ballot, id, entry, opts)

	return ballot, nil
}

// Heal submits a heal request for the given key.  It returns an error if the
// options are invalid.
func (hlog *Hexalog) Heal(key []byte, opts *RequestOptions) error {
	err := hlog.checkOptions(opts)
	if err == nil {
		ent := &Entry{Key: key}
		hlog.hch <- &ReqResp{Options: opts, Entry: ent}
	}

	return err
}

// Seed gets seed keys from host and starts seeding the local log.  bufsize
// is the seed buffer size, parallel is the number of parallel keys to seed.
func (hlog *Hexalog) Seed(existing string, bufsize, parallel int) error {
	stream, err := hlog.trans.GetSeedKeys(existing)
	if err != nil {
		return err
	}

	if parallel < 1 {
		parallel = 1
	}

	seeds := make(chan *KeySeed, bufsize)
	var wg sync.WaitGroup
	wg.Add(parallel)

	for i := 0; i < parallel; i++ {

		go func(hlog *Hexalog, remote string, idx, total int) {
			var c int
			for seed := range seeds {
				if er := hlog.checkLastEntryOrPull(remote, seed.Key, seed.Marker); er != nil {
					log.Println("[ERROR]", er)
					continue
				}

				c++
			}
			wg.Done()
			log.Printf("[INFO] Seeded set=%d/%d keys=%d remote=%s", idx, total, c, remote)

		}(hlog, existing, i, parallel)

	}

	var seed *KeySeed
	for {
		if seed, err = stream.Recv(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		seeds <- seed
	}

	close(seeds)

	if err != nil {
		return err
	}

	wg.Wait()
	return nil
}

// Shutdown signals a shutdown and waits for all go-routines to exit before
// returning.  It will take atleast the amount of time specified as the ballot
// reap interval as shutdown for the ballot reaper is checked at the top of the
// loop.  It does not shutdown the stores as they may be in use by other
// components
func (hlog *Hexalog) Shutdown() {
	atomic.StoreInt32(&hlog.shutdown, 1)

	close(hlog.pch)
	close(hlog.cch)
	close(hlog.hch)

	// Wait for go-routines to exit their loop - propose, commit, reap ballots
	for i := 0; i < 3; i++ {
		<-hlog.shutdownCh
	}

	log.Println("[INFO] Hexalog shutdown complete!")
}
