package hexalog

import (
	"fmt"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

// sendProposal makes a single proposal request to a location.  if a hexatype.ErrPreviousHash
// error is returned a  heal request is submitted
func (hlog *Hexalog) sendProposal(entry *hexatype.Entry, loc *hexaring.Location, idx int, opts *hexatype.RequestOptions) error {
	host := loc.Vnode.Host
	o := opts.CloneWithSourceIndex(int32(idx))

	log.Printf("[DEBUG] Broadcast phase=propose %s -> %s index=%d", hlog.conf.Hostname, host, o.SourceIndex)
	err := hlog.trans.ProposeEntry(host, entry, o)

	switch err {
	case hexatype.ErrPreviousHash:
		hlog.hch <- &hexatype.ReqResp{
			Options: opts,
			Entry:   entry,
		}

		log.Printf("[DEBUG] Healing from sending proposal key=%s height=%d", entry.Key, entry.Height)

	}

	return err
}

// broadcastPropose broadcasts proposal entries to all members in the peer set
func (hlog *Hexalog) broadcastPropose(entry *hexatype.Entry, opts *hexatype.RequestOptions) error {
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

		if err := hlog.sendProposal(entry, p, idx, opts); err != nil {
			return err
		}

	}

	return nil
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

// broadcastCommit broadcasts the commit entry to all members in the peer set
func (hlog *Hexalog) broadcastCommit(entry *hexatype.Entry, opts *hexatype.RequestOptions) error {
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
