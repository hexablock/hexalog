package hexalog

import (
	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

func (hlog *Hexalog) heal(key []byte, locs hexaring.LocationSet) error {
	// Make sure we are part of the set
	if _, err := locs.GetByHost(hlog.conf.Hostname); err != nil {
		return err
	}

	var last *hexatype.Entry
	keylog, err := hlog.store.GetKey(key)
	if err != nil {
		// Create new key
		if _, err = hlog.store.NewKey(key); err != nil {
			return err
		}
		last = &hexatype.Entry{Key: key}

	} else {

		if last = keylog.LastEntry(); last == nil {
			last = &hexatype.Entry{Key: key}
		}

	}

	er := hlog.healFromLeader(last, locs)
	return mergeErrors(err, er)
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
	// If we are the leader there's nothing to do
	if loc.Host() == hlog.conf.Hostname {
		//return fmt.Errorf("cannot heal from self")
		return nil
	}

	_, err = hlog.trans.FetchKeylog(loc.Host(), last, &hexatype.RequestOptions{})
	return err
}

// healKeys starts listening to the heal channel and tries to heal the given keys as they
// come in.
func (hlog *Hexalog) healKeys() {
	for req := range hlog.hch {

		if err := hlog.heal(req.Entry.Key, req.Options.PeerSet); err != nil {
			log.Printf("[ERROR] Failed to heal key=%s height=%d id=%x error='%v'", req.Entry.Key, req.Entry.Height, req.ID, err)
		}

	}

	// Notify that we have shutdown
	log.Printf("[INFO] Healer shutdown!")
	hlog.shutdownCh <- struct{}{}
}
