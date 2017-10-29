package hexalog

import (
	"github.com/hexablock/log"
)

func (hlog *Hexalog) heal(key []byte, locs []*Participant) error {

	leader, err := hlog.Leader(key, locs)
	if err != nil {
		return err
	}

	lle := leader.LastEntry()
	// Nothing to do as leader has nothing
	if lle == nil {
		return nil
	}

	var (
		h   = hlog.conf.Hasher()
		llh = lle.Hash(h)
	)

	// Leader location
	lloc := leader.Location()

	// We are the leader, so nothing to do
	if lloc.Host == hlog.conf.Hostname {
		return nil
	}

	if err = hlog.checkLastEntryOrPull(lloc.Host, key, llh); err != nil {
		return err
	}

	// // Get local keylog
	// keylog, err := hlog.store.GetKey(key)
	// if err != nil {
	// 	// Create new key
	// 	if err == hexatype.ErrKeyNotFound {
	// 		if keylog, err = hlog.store.NewKey(key); err != nil {
	// 			return err
	// 		}
	// 	} else {
	// 		return err
	// 	}
	// }
	// defer keylog.Close()
	//
	// var (
	// 	h    = hlog.conf.Hasher()
	// 	llh  = lle.Hash(h)
	// 	slh  []byte
	// 	last = keylog.LastEntry()
	// )
	//
	// if last == nil {
	// 	last = &Entry{Key: key, Height: 0}
	// 	slh = make([]byte, h.Size())
	// } else {
	// 	h.Reset()
	// 	slh = last.Hash(h)
	// }
	//
	// // Fetch if there is a mismatch.
	// if bytes.Compare(slh, llh) != 0 {
	// 	_, er := hlog.trans.PullKeylog(lloc.Host, last, nil)
	// 	return mergeErrors(err, er)
	// }

	//Check consistency
	leader, er := hlog.Leader(key, locs)
	if er == nil {

		ok, loc := leader.IsConsistent()
		if ok {
			log.Printf("[INFO] Key consistent key=%s", key)
		} else {
			log.Printf("[TODO] Key inconsistent key=%s host=%s", key, loc.Host)
		}

	} else {
		log.Printf("[ERROR] Failed to get leader key=%s", key)
	}

	return err
}

// healKeys starts listening to the heal channel and tries to heal the given keys as they
// come in.
func (hlog *Hexalog) healKeys() {
	for req := range hlog.hch {
		participants := req.Options.PeerSet
		key := req.Entry.Key

		if err := hlog.heal(key, participants); err != nil {
			log.Printf("[ERROR] Failed to heal key=%s height=%d id=%x error='%v'", req.Entry.Key, req.Entry.Height, req.ID, err)
		}

	}

	// Notify that we have shutdown
	log.Printf("[INFO] Healer shutdown!")
	hlog.shutdownCh <- struct{}{}
}
