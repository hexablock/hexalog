package hexalog

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/hexablock/log"
)

// sendProposal makes a single proposal request to a location.  if a hexatype.ErrPreviousHash
// error is returned a  heal request is submitted
func (hlog *Hexalog) sendProposal(ctx context.Context, entry *Entry, loc *Participant, idx int, opts *RequestOptions) (*ReqResp, error) {
	host := loc.Host
	o := opts.CloneWithSourceIndex(int32(idx))

	log.Printf("[DEBUG] Propose broadcast key=%s %s -> %s index=%d",
		entry.Key, hlog.conf.AdvertiseHost, host, o.SourceIndex)

	resp, err := hlog.trans.ProposeEntry(ctx, host, entry, o)

	// switch err {
	//
	// case hexatype.ErrPreviousHash:
	// 	hlog.hch <- &ReqResp{
	// 		Options: opts,
	// 		Entry:   entry,
	// 	}
	//
	// 	log.Printf("[DEBUG] Healing from sending proposal key=%s height=%d", entry.Key, entry.Height)
	//
	// }

	return resp, err
}

// broadcastPropose broadcasts proposal entries to all members in the peer set
func (hlog *Hexalog) broadcastPropose(entry *Entry, opts *RequestOptions) ([]*ReqResp, error) {
	// Get self index in the PeerSet.
	idx, ok := hlog.getSelfIndex(opts.PeerSet)
	if !ok {
		return nil, fmt.Errorf("%s not in PeerSet", hlog.conf.AdvertiseHost)
	}

	l := len(opts.PeerSet) - 1

	resp := make(chan *ReqResp, l)
	errCh := make(chan error, l)

	ctx, cancel := context.WithCancel(context.Background())

	for _, p := range opts.PeerSet {
		// Skip broadcasting to self
		if hlog.conf.AdvertiseHost == p.Host {
			continue
		}

		go func(ent *Entry, loc *Participant, i int, o *RequestOptions) {
			rsp, err := hlog.sendProposal(ctx, ent, loc, i, o)
			if err == nil {
				resp <- rsp
			} else {
				errCh <- err
			}
		}(entry, p, idx, opts)

	}

	defer cancel()

	out := make([]*ReqResp, 0, l)

	for i := 0; i < l; i++ {
		select {
		case err := <-errCh:
			return out, err
		case resp := <-resp:
			out = append(out, resp)
		}

	}

	return out, nil
}

// broadcastProposals starts consuming the proposal broadcast channel to broadcast
// locally proposed entries to the network.  This is mean to be run in a go-routine.
func (hlog *Hexalog) broadcastProposals() {

	for msg := range hlog.pch {

		_, err := hlog.broadcastPropose(msg.Entry, msg.Options)
		if err != nil {
			id := msg.Entry.Hash(hlog.conf.Hasher())
			hlog.ballotGetClose(id, err)
			continue
		}

		// Commit once the proposal is accepted by all participants
		if _, err = hlog.Commit(msg.Entry, msg.Options); err != nil {
			id := msg.Entry.Hash(hlog.conf.Hasher())
			hlog.ballotGetClose(id, err)
		}

	}

	log.Println("[INFO] Proposal broadcaster shutdown!")
	// Notify that we have shutdown
	hlog.shutdownCh <- struct{}{}
}

// broadcastCommit broadcasts the commit entry to all members in the peer set
func (hlog *Hexalog) broadcastCommit(entry *Entry, opts *RequestOptions) error {
	// Get self index in the PeerSet.
	idx, ok := hlog.getSelfIndex(opts.PeerSet)
	if !ok {
		return fmt.Errorf("%s not in PeerSet", hlog.conf.AdvertiseHost)
	}

	l := len(opts.PeerSet) - 1
	resp := make(chan error, l)
	ctx, cancel := context.WithCancel(context.Background())

	for _, p := range opts.PeerSet {
		// Do not broadcast to self
		if hlog.conf.AdvertiseHost == p.Host {
			continue
		}

		go func(ent *Entry, loc *Participant, i int, opt *RequestOptions) {

			o := opt.CloneWithSourceIndex(int32(i))
			host := loc.Host

			err := hlog.trans.CommitEntry(ctx, host, ent, o)

			log.Printf("[DEBUG] Commit broadcast key=%s %s -> %s index=%d error='%v'",
				ent.Key, hlog.conf.AdvertiseHost, host, o.SourceIndex, err)

			resp <- err

		}(entry, p, idx, opts)

	}

	defer cancel()

	for i := 0; i < l; i++ {
		if err := <-resp; err != nil {
			return err
		}

	}

	return nil
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

		// Close the ballot with the given error
		id := en.Hash(hlog.conf.Hasher())
		hlog.ballotGetClose(id, err)

		// Rollback the entry. This may fail and should be harmless
		// if err = hlog.store.RollbackEntry(en); err != nil {
		// 	log.Printf("[WARN] Rollback failed: %v key=%s height=%d id=%x", err,
		// 		en.Key, en.Height, id)
		// }

	}

	log.Println("[INFO] Commit broadcaster shutdown!")
	// Notify that we have shutdown
	hlog.shutdownCh <- struct{}{}
}
