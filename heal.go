package hexalog

import (
	"bytes"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

func (hlog *Hexalog) heal(key []byte, locs hexaring.LocationSet) error {
	// Make sure we are part of the set.  We check this first to avoid rpcs for
	// the leader call
	// _, err := locs.GetByHost(hlog.conf.Hostname)
	// if err != nil {
	// 	return err
	// }

	leader, err := hlog.Leader(key, locs)
	if err != nil {
		return err
	}

	lle := leader.LastEntry()
	// Nothing to do as leader has nothing
	if lle == nil {
		return nil
	}

	// Leader location
	lloc := leader.Location()

	// We are the leader, so nothing to do
	if lloc.Host() == hlog.conf.Hostname {
		return nil
	}

	keylog, err := hlog.store.GetKey(key)
	if err != nil {
		// Create new key
		if err == hexatype.ErrKeyNotFound {
			if keylog, err = hlog.store.NewKey(key); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	var (
		h   = hlog.conf.Hasher.New()
		llh = lle.Hash(h)
		slh []byte
		//lasts = leader.Entries()
		//last  = lasts[sloc.Priority]
		last = keylog.LastEntry()
		//lastID =
	)

	if last == nil {
		last = &hexatype.Entry{Key: key, Height: 0}
		//lastID = make([]byte, h.Size())
		slh = make([]byte, h.Size())
	} else {
		h.Reset()
		slh = last.Hash(h)
	}

	// Fetch if there is a mismatch.
	if bytes.Compare(slh, llh) != 0 {
		_, er := hlog.trans.FetchKeylog(lloc.Host(), last, nil)
		return mergeErrors(err, er)
	}

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
