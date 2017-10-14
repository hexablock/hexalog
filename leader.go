package hexalog

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
)

// Leader returns the leader for a key.  It gets the last entry from the given location
// set and finds the one with the max height which it considered to be the leader.
// This is technically the pseudo-leader
func (hlog *Hexalog) Leader(key []byte, locs hexaring.LocationSet) (*KeyLeader, error) {
	l := len(locs)

	// Get last entry for a key from each location
	lasts := make([]*Entry, l)
	for i := 0; i < l; i++ {

		loc := locs[i]
		if loc.Vnode.Host == hlog.conf.Hostname {
			lasts[i] = hlog.store.LastEntry(key)
			continue
		}

		entry, err := hlog.trans.LastEntry(loc.Vnode.Host, key, &RequestOptions{})
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

	return &KeyLeader{locs: locs, lasts: lasts, idx: maxIdx, hasher: hlog.conf.Hasher}, nil
}

// KeyLeader represens a leader for a key for a location set.
type KeyLeader struct {
	key    []byte               // key in question
	idx    int                  // leader index in the set for locs and lasts
	locs   hexaring.LocationSet // participating locations
	lasts  []*Entry             // last entry from each participant
	hasher hexatype.Hasher      // hash function
}

// Key returns the key in question
func (l *KeyLeader) Key() []byte {
	return l.key
}

// IsConsistent returns true if all entries are consistent i.e all last
// entries for each location are the same.  If consistent the natural key
// location is returned; if the key is inconsistent then the first
// inconsistent location is returned.
func (l *KeyLeader) IsConsistent() (bool, *hexaring.Location) {
	// Use leader id to compare against
	id := l.lasts[l.idx].Hash(l.hasher.New())
	for i, ent := range l.lasts {
		if i == l.idx {
			continue
		}

		if ent == nil {
			return false, l.locs[i]
		}

		nid := ent.Hash(l.hasher.New())
		if bytes.Compare(id, nid) != 0 {
			// Return inconsistent location
			return false, l.locs[i]
		}
	}
	// Return leader location
	return true, l.locs[l.idx]
}

// LocationSet returns a slice of participating locations
func (l *KeyLeader) LocationSet() hexaring.LocationSet {
	return l.locs
}

// Location returns the Location of the leader
func (l *KeyLeader) Location() *hexaring.Location {
	return l.locs[l.idx]
}

// LastEntry returns the last entry of the leader
func (l *KeyLeader) LastEntry() *Entry {
	return l.lasts[l.idx]
}

// Entries returns the last entries for each location
func (l *KeyLeader) Entries() []*Entry {
	return l.lasts
}

// MarshalJSON is a custom marshaller to output a user friendly structure.
func (l KeyLeader) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key    string
		Leader *hexaring.Location
		Entry  *Entry
	}{string(l.key), l.Location(), l.LastEntry()})
}
