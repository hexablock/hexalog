package hexalog

import (
	"fmt"
	"log"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
)

// healFromLeader takes the current local last entry for the key and a set of locations,
// determines the node with the highest height and tries to replicate log entries from
// it
func (hlog *Hexalog) healFromLeader(last *hexatype.Entry, locs hexaring.LocationSet) error {
	keyLeader, err := hlog.Leader(last.Key, locs)
	if err != nil {
		return err
	}

	loc := keyLeader.Location()
	// Can't heal self
	if loc.Vnode.Host == hlog.conf.Hostname {
		return fmt.Errorf("cannot heal from self")
	}

	_, err = hlog.trans.FetchKeylog(loc.Vnode.Host, last, &hexatype.RequestOptions{})
	return err
}

func (hlog *Hexalog) heal(req *hexatype.ReqResp) error {
	ent := req.Entry
	locs := req.Options.LocationSet()
	keylog, err := hlog.store.GetKey(ent.Key)
	// Heal from the leader if we don't have the key at all
	if err != nil {
		// Get our location id to create new key
		sloc, err := locs.GetByHost(hlog.conf.Hostname)
		if err != nil {
			return err
		}
		// Create new key
		if _, err = hlog.store.NewKey(ent.Key, sloc.ID); err != nil {
			return err
		}

		// If we don't have the key try to get it from the leader
		last := &hexatype.Entry{Key: ent.Key}
		er := hlog.healFromLeader(last, locs)
		return mergeErrors(err, er)
	}

	// Try each location
	for _, loc := range locs {
		vn := loc.Vnode
		if vn.Host == hlog.conf.Hostname {
			continue
		}

		last := keylog.LastEntry()
		if last == nil {
			// Dont set Previous so we can signal a complete keylog download
			last = &hexatype.Entry{Key: ent.Key}
		}

		if _, er := hlog.trans.FetchKeylog(vn.Host, last, &hexatype.RequestOptions{}); er != nil {
			log.Printf("[ERROR] Failed to fetch KeyLog key=%s vnode=%s/%x error='%v'", ent.Key, vn.Host, vn.Id, er)
		}

	}

	return nil
}

func (hlog *Hexalog) healKeys() {
	for req := range hlog.hch {

		if err := hlog.heal(req); err != nil {
			log.Printf("[ERROR] Failed to heal key=%s height=%d id=%x error='%v'", req.Entry.Key, req.Entry.Height, req.ID, err)
		}

	}

	// Notify that we have shutdown
	log.Printf("[INFO] Healer shutdown!")
	hlog.shutdownCh <- struct{}{}
}
