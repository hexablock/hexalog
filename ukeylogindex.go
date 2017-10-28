package hexalog

import (
	"bytes"
	"encoding/hex"
	"encoding/json"

	"github.com/hexablock/hexatype"
)

// NewUnsafeKeylogIndex creates a new keylog index
func NewUnsafeKeylogIndex(key []byte) *UnsafeKeylogIndex {
	return &UnsafeKeylogIndex{
		Key:     key,
		Entries: [][]byte{},
	}
}

// Append appends the id to the index making sure prev matches the id of the current
// last entry.  If this is the first entry, prev should be a zero hash.
func (idx *UnsafeKeylogIndex) Append(id, prev []byte) error {
	last := idx.Last()
	if last == nil {
		if !isZeroBytes(prev) {
			return hexatype.ErrPreviousHash
		}
	} else if bytes.Compare(last, prev) != 0 {
		return hexatype.ErrPreviousHash
	}

	idx.Entries = append(idx.Entries, id)
	idx.Height++

	// Check marker to see if it needs to be removed. The marker is removed once that entry
	// has been added to the index
	if idx.Marker != nil {
		if bytes.Compare(idx.Marker, id) == 0 {
			idx.Marker = nil
		}
	}

	return nil
}

// SetMarker sets the marker for he index only if it is not currently in the index
func (idx *UnsafeKeylogIndex) SetMarker(marker []byte) bool {
	if !idx.Contains(marker) {
		idx.Marker = marker
		return true
	}
	return false
}

// Contains returns true if the index contains the given entry id
func (idx *UnsafeKeylogIndex) Contains(id []byte) bool {
	for _, ent := range idx.Entries {
		if bytes.Compare(ent, id) == 0 {
			return true
		}
	}
	return false
}

// Last returns the last entry id or nil if there are no entries.
func (idx *UnsafeKeylogIndex) Last() []byte {
	l := len(idx.Entries)
	if l == 0 {
		return nil
	}
	return idx.Entries[l-1]
}

// Rollback removes the last entry from the index. It returns the remaining entries and
// whether a rollback was performed
func (idx *UnsafeKeylogIndex) Rollback() (int, bool) {
	l := len(idx.Entries)
	var ok bool
	if l > 0 {
		l--
		idx.Entries = idx.Entries[:l]
		ok = true
		idx.Height--
	}

	return l, ok
}

// Count returns the number of entries in the index
func (idx *UnsafeKeylogIndex) Count() int {
	return len(idx.Entries)
}

// Iter iterates through each entry id starting fromt he seek position.  If seek is nil
// all entries are traversed.  If the callback returns true the function exits immediately
func (idx *UnsafeKeylogIndex) Iter(seek []byte, cb func(id []byte) error) (err error) {
	var s int

	// Find the seek position from the index
	if seek != nil {
		s = -1
		for i, e := range idx.Entries {
			if bytes.Compare(seek, e) == 0 {
				s = i
				break
			}
		}
		// Return error if we can't find the seek position
		if s < 0 {
			return hexatype.ErrEntryNotFound
		}
	}

	l := idx.Count()
	// Start from seek issueing callbacks
	for i := s; i < l; i++ {
		// Bail if true
		if err = cb(idx.Entries[i]); err != nil {
			break
		}
	}

	return err
}

// MarshalJSON is a custom marshaller to handle encoding byte slices to hex.  We do not
// have an unmarshaller as the index is not directly written to.
func (idx UnsafeKeylogIndex) MarshalJSON() ([]byte, error) {
	obj := struct {
		Key     string
		Height  uint32
		Marker  string `json:",omitempty"`
		Entries []string
	}{
		Key:     string(idx.Key),
		Height:  idx.Height,
		Marker:  hex.EncodeToString(idx.Marker),
		Entries: make([]string, len(idx.Entries)),
	}

	for i, e := range idx.Entries {
		obj.Entries[i] = hex.EncodeToString(e)
	}

	return json.Marshal(obj)
}
