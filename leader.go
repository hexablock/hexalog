package hexalog

import (
	"encoding/json"
	"fmt"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
)

// Leader returns the leader for a key.  It gets the last entry from the given location
// set and finds the one with the max height which it uses as the leader.
func (hlog *Hexalog) Leader(key []byte, locs hexaring.LocationSet) (*KeyLeader, error) {
	l := len(locs)

	// Get last entry for a key from each location
	lasts := make([]*hexatype.Entry, l)
	for i := 0; i < l; i++ {

		loc := locs[i]
		if loc.Vnode.Host == hlog.conf.Hostname {
			lasts[i] = hlog.store.LastEntry(key)
			continue
		}

		entry, err := hlog.trans.LastEntry(loc.Vnode.Host, key, &hexatype.RequestOptions{})
		if err == nil && entry != nil {
			lasts[i] = entry
		}

	}

	var (
		maxIdx = -1
		max    uint32
		nils   int
	)
	// Find the location with the max height for the key.
	for i, last := range lasts {
		if last == nil {
			nils++
		} else if last.Height > max {
			maxIdx = i
			max = last.Height
		}
	}

	if nils == l {
		maxIdx = 0
	} else if maxIdx < 0 {
		return nil, fmt.Errorf("unable to find max height")
	}

	return &KeyLeader{locs: locs, lasts: lasts, idx: maxIdx}, nil
}

// KeyLeader represens a leader for a key for a location set.
type KeyLeader struct {
	key   []byte
	idx   int // leader index
	locs  hexaring.LocationSet
	lasts []*hexatype.Entry
}

// Key returns the key in question
func (l *KeyLeader) Key() []byte {
	return l.key
}

// LocationSet returns a slice of participating locations
func (l *KeyLeader) LocationSet() hexaring.LocationSet {
	return l.locs
}

// Location returns the Location for the leader
func (l *KeyLeader) Location() *hexaring.Location {
	return l.locs[l.idx]
}

// LastEntry returns the last entry of the selected leader
func (l *KeyLeader) LastEntry() *hexatype.Entry {
	return l.lasts[l.idx]
}

// MarshalJSON is a custom marshaller to output a user friendly structure.
func (l KeyLeader) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key    string
		Leader *hexaring.Location
		Entry  *hexatype.Entry
	}{string(l.key), l.Location(), l.LastEntry()})
}