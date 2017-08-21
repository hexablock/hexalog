package hexalog

import (
	"fmt"
	"log"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
)

func (hlog *Hexalog) heal(req *hexatype.ReqResp) error {
	ent := req.Entry
	locs := req.Options.LocationSet()
	keylog, err := hlog.store.GetKey(ent.Key)

	// Heal from the leader if we don't have the key at all.
	if err != nil {
		// Make sure we are part of the set
		if _, err = locs.GetByHost(hlog.conf.Hostname); err != nil {
			return err
		}
		// Create new key
		if _, err = hlog.store.NewKey(ent.Key); err != nil {
			return err
		}

		// If we don't have the key try to get it from the leader
		last := &hexatype.Entry{Key: ent.Key}
		er := hlog.healFromLeader(last, locs)
		return mergeErrors(err, er)
	}

	// Try each location
	for _, loc := range locs {
		if loc.Host() == hlog.conf.Hostname {
			continue
		}

		last := keylog.LastEntry()
		if last == nil {
			// Dont set Previous so we can signal a complete keylog download
			last = &hexatype.Entry{Key: ent.Key}
		}

		if _, er := hlog.trans.FetchKeylog(loc.Host(), last, &hexatype.RequestOptions{}); er != nil {
			log.Printf("[ERROR] Failed to fetch KeyLog key=%s vnode=%s/%x error='%v'", ent.Key,
				loc.Host(), loc.Vnode.Id, er)
		}

	}

	return nil
}

// healFromLeader takes the current local last entry for the key and a set of locations,
// determines the node with the highest height and tries to replicate log entries from
// it
func (hlog *Hexalog) healFromLeader(last *hexatype.Entry, locs hexaring.LocationSet) error {
	keyLeader, err := hlog.Leader(last.Key, locs)
	if err != nil {
		return err
	}

	loc := keyLeader.Location()
	// Can't heal self from self
	if loc.Host() == hlog.conf.Hostname {
		return fmt.Errorf("cannot heal from self")
	}

	_, err = hlog.trans.FetchKeylog(loc.Host(), last, &hexatype.RequestOptions{})
	return err
}

// healKeys starts listening to the heal channel and tries to heal the given keys as they
// come in.
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
