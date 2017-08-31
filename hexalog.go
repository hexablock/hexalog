package hexalog

import (
	"fmt"
	"sync"
	"sync/atomic"

	"golang.org/x/net/context"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

// Transport implements a Hexalog network transport
type Transport interface {
	// Gets an entry from a remote host
	GetEntry(host string, key, id []byte, opts *hexatype.RequestOptions) (*hexatype.Entry, error)
	// Get last entry for the key
	LastEntry(host string, key []byte, opts *hexatype.RequestOptions) (*hexatype.Entry, error)
	// Proposes an entry on the remote host
	ProposeEntry(ctx context.Context, host string, entry *hexatype.Entry, opts *hexatype.RequestOptions) error
	// Commits an entry on the remote host
	CommitEntry(ctx context.Context, host string, entry *hexatype.Entry, opts *hexatype.RequestOptions) error
	// Transfers a complete key log to the remote host
	TransferKeylog(host string, key []byte, opts *hexatype.RequestOptions) error
	// Gets all entries from the remote host for a key starting at entry
	FetchKeylog(host string, entry *hexatype.Entry, opts *hexatype.RequestOptions) (*FutureEntry, error)
	// Registers the log when available
	Register(hlog *Hexalog)
	// Shutdown the transport closing outbound connections
	Shutdown()
}

// Hexalog is the core log that is responsible for consensus, election, serialization and
// all other aspects pertaining to consistency
type Hexalog struct {
	conf *Config
	// Internal fsm.  This wraps the application FSM from the config
	fsm *fsm
	// Underlying transport
	trans Transport
	// The store containing log entires that are committed, but not necessary applied
	// to the FSM
	store *LogStore
	// Currently active ballots
	mu      sync.RWMutex
	ballots map[string]*Ballot
	// Propose broadcast channel to broadcast proposals to the network peer set
	pch chan *hexatype.ReqResp
	// Commit broadcast channel to broadcast commits to the network peer set
	cch chan *hexatype.ReqResp
	// Channel for heal requests.  When previous hash mismatches occur, the log will send a
	// request down this channel to allow applications to try to recover. This is usually
	// the case when a keylog falls behind.
	hch chan *hexatype.ReqResp
	// Gets set when once a shutdown is signalled
	shutdown int32
	// This is initialized with a static size of 3 as we launch 3 go-routines.  The heal
	// queue is not part of this number
	shutdownCh chan struct{}
}

// NewHexalog initializes a new Hexalog and starts the entry broadcaster
func NewHexalog(conf *Config, appFSM FSM, logstore *LogStore, stableStore StableStore,
	trans Transport) (*Hexalog, error) {

	// Init internal FSM that manages the user provided application fsm
	ifsm, err := newFsm(appFSM, stableStore, logstore, conf.Hasher)
	if err != nil {
		return nil, err
	}

	hlog := &Hexalog{
		conf:       conf,
		fsm:        ifsm,
		trans:      trans,
		ballots:    make(map[string]*Ballot),
		pch:        make(chan *hexatype.ReqResp, conf.BroadcastBufSize),
		cch:        make(chan *hexatype.ReqResp, conf.BroadcastBufSize),
		hch:        make(chan *hexatype.ReqResp, conf.HealBufSize),
		store:      logstore,
		shutdownCh: make(chan struct{}, 4),
	}

	// Check to make sure the fsm has all the entries in the log applied.  If not then submit
	// the remaining entries to be applied to the fsm.
	if err = hlog.fsm.check(); err != nil {
		return nil, err
	}

	// Start
	hlog.start()

	return hlog, nil
}

func (hlog *Hexalog) start() {
	// Register Hexalog to the transport to handle RPC requests
	hlog.trans.Register(hlog)

	go hlog.broadcastProposals()
	go hlog.broadcastCommits()
	go hlog.healKeys()
	go hlog.reapBallots()
}

// New returns a new Entry to be appended to the log for the given key.
func (hlog *Hexalog) New(key []byte) *hexatype.Entry {
	return hlog.store.NewEntry(key)
}

// Propose proposes an entry to the log.  It votes on a ballot if it exists or creates one
// then votes. If required votes has been reach it also moves to the commit phase.
func (hlog *Hexalog) Propose(entry *hexatype.Entry, opts *hexatype.RequestOptions) (*Ballot, error) {
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
		// Check for heal if previous hash mismatch
		if err == hexatype.ErrPreviousHash {
			// Try to heal if the new height is > then the current one
			if entry.Height > prevHeight {

				hlog.hch <- &hexatype.ReqResp{
					ID:      id,    // entry hash id
					Entry:   entry, // entry itself
					Options: opts,  // participating peers
				}

				// TODO: Gate to avoid an infinite retry.  Currently gated only by height check.
				// TODO: Retry propose request
				//return hlog.Propose(entry, opts)

			} else if entry.Height == prevHeight {
				log.Printf("[TODO] Heal same height entry key=%s height=%d id=%x", entry.Key, entry.Height, id)

				// TODO: deep reconciliation

			} else {
				log.Printf("[DEBUG] Not healing key=%s curr-height=%d proposed-height=%d", entry.Key, prevHeight, entry.Height)
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

		// Cast a vote for ourself if we are not the SourceIndex, on top of casting the remote
		// vote taking place below
		if idx != int(opts.SourceIndex) {
			if _, err = ballot.votePropose(id, string(loc.Vnode.Id)); err != nil {
				return nil, err
			}

			// Create a new key if height is zero and we don't have it.
			if prevHeight == 0 {
				if _, er := hlog.store.GetKey(entry.Key); er != nil {
					if _, err = hlog.store.NewKey(entry.Key); err != nil {
						ballot.close(err)
						return ballot, err
					}
				}
			}

			// Broadcast proposal
			hlog.pch <- &hexatype.ReqResp{Entry: entry, Options: opts}
		}

	} else {
		hlog.mu.Unlock()
	}

	vid := opts.SourcePeer().Vnode.Id
	pvotes, err := ballot.votePropose(id, string(vid))

	log.Printf("[DEBUG] Propose host=%s key=%s index=%d ballot=%p votes=%d voter=%x error='%v'",
		hlog.conf.Hostname, entry.Key, opts.SourceIndex, ballot, pvotes, vid, err)

	if err != nil {
		return ballot, err
	}

	if pvotes == 1 {

		// Create a new key if height is 0 and we don't have the key
		if prevHeight == 0 {
			if _, er := hlog.store.GetKey(entry.Key); er != nil {
				if _, err = hlog.store.NewKey(entry.Key); err != nil {
					ballot.close(err)
					return ballot, err
				}
			}
		}

		hlog.pch <- &hexatype.ReqResp{Entry: entry, Options: opts}

	} else if pvotes == hlog.conf.Votes {
		log.Printf("[DEBUG] Proposal accepted host=%s key=%s", hlog.conf.Hostname, entry.Key)
		// Start local commit phase.  We use our index as the voter
		var cvotes int
		cvotes, err = ballot.voteCommit(id, string(opts.PeerSet[idx].Vnode.Id))
		if err == nil {

			// Take action if we have the required commits by appending the log entry and calling
			// app fsm.Apply
			hlog.checkCommitAndAct(cvotes, ballot, id, entry, opts)

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
func (hlog *Hexalog) Commit(entry *hexatype.Entry, opts *hexatype.RequestOptions) (*Ballot, error) {
	// Check request options
	if err := hlog.checkOptions(opts); err != nil {
		return nil, err
	}

	//
	// TODO: verify signature
	//

	id := entry.Hash(hlog.conf.Hasher.New())
	// Make sure we have the ballot for the entry id
	ballot := hlog.getBallot(id)
	if ballot == nil {
		return nil, errBallotNotFound
	}

	//
	// TODO: Verify & Validate
	//

	sloc := opts.SourcePeer()
	vid := sloc.Vnode.Id
	votes, err := ballot.voteCommit(id, string(vid))
	log.Printf("[DEBUG] Commit host=%s key=%s index=%d ballot=%p votes=%d voter=%x error='%v'",
		hlog.conf.Hostname, entry.Key, opts.SourceIndex, ballot, votes, vid, err)

	// We do not rollback here as we could have a faulty voter trying to commit without
	// having a proposal.
	if err != nil {
		return ballot, err
	}

	hlog.checkCommitAndAct(votes, ballot, id, entry, opts)

	return ballot, nil
}

// Heal submits a heal request for the given key.  It returns an error if the options are
// invalid or if the node is not part of the set.
func (hlog *Hexalog) Heal(key []byte, opts *hexatype.RequestOptions) error {
	if err := hlog.checkOptions(opts); err != nil {
		return err
	}
	locs := hexaring.LocationSet(opts.PeerSet)
	if _, err := locs.GetByHost(hlog.conf.Hostname); err != nil {
		return err
	}

	ent := &hexatype.Entry{Key: key}
	hlog.hch <- &hexatype.ReqResp{Options: opts, Entry: ent}

	return nil
}

// Shutdown signals a shutdown and waits for all go-routines to exit before returning.  It
// will take atleast the amount of time specified as the ballot reap interval as shutdown
// for the ballot reaper is checked at the top of the loop
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
