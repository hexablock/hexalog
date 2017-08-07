package hexalog

import (
	"bytes"
	"errors"

	"github.com/hexablock/hexalog/store"
	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

var (
	errNotLastEntry = errors.New("not last entry")
)

// Keylog represents a log for a given key.  It manages operations against log entries for
// a key.
type Keylog struct {
	// Datastore containing entries
	entries store.EntryStore
	// KeylogIndex interface
	idx store.KeylogIndex
	// Hash function
	hasher hexatype.Hasher
}

// NewKeylog initializes a new log for a key. It takes a key, location id and hash function used
// to compute hash id's of log entries.
func NewKeylog(entries store.EntryStore, idx store.KeylogIndex, hasher hexatype.Hasher) *Keylog {
	return &Keylog{
		hasher:  hasher,
		entries: entries,
		idx:     idx,
	}
}

// LocationID returns the LocationID for the replicated key
func (keylog *Keylog) LocationID() []byte {
	return keylog.idx.LocationID()
}

// LastEntry returns the last entry in the Keylog.  If there are no entries in the log ,
// nil is returned.
func (keylog *Keylog) LastEntry() *hexatype.Entry {
	// Get last id from index
	lid := keylog.idx.Last()
	if lid == nil {
		return nil
	}
	// Get entry based on id
	entry, _ := keylog.entries.Get(lid)
	return entry
}

// AppendEntry appends an entry to the log.  It returns an error if there is a previous
// hash mismatch
func (keylog *Keylog) AppendEntry(entry *hexatype.Entry) (err error) {
	id := entry.Hash(keylog.hasher.New())

	// Add id to index.  The index will check for order
	if err = keylog.idx.Append(id, entry.Previous); err != nil {
		return err
	}
	// Add entry to store
	if err = keylog.entries.Set(id, entry); err == nil {
		log.Printf("[DEBUG] Appended key=%s height=%d id=%x", entry.Key, entry.Height, entry.Hash(keylog.hasher.New()))
	}

	return
}

// GetEntry gets and entry from the Keylog
func (keylog *Keylog) GetEntry(id []byte) (*hexatype.Entry, error) {
	return keylog.entries.Get(id)
}

// RollbackEntry rolls the keylog back by the entry.  It only rolls back if the entry is
// the last entry.  It returns the number of entries remaining in the log and/or an error
func (keylog *Keylog) RollbackEntry(entry *hexatype.Entry) (int, error) {
	// Compute hash id outside of lock
	id := entry.Hash(keylog.hasher.New())

	lid := keylog.idx.Last()
	if lid == nil {
		// Return 0 entry count
		return 0, hexatype.ErrEntryNotFound
	}

	if bytes.Compare(id, lid) != 0 {
		return -1, errNotLastEntry
	}

	// Remove the entry from the EntryStore
	if err := keylog.entries.Delete(id); err != nil {
		return keylog.idx.Count(), err
	}

	// Remove id from index
	return keylog.idx.Rollback(), nil
}

// Iter iterates over entries starting from the seek position.  It iterates over all
// entries if seek is nil
func (keylog *Keylog) Iter(seek []byte, cb func(entry *hexatype.Entry) error) error {

	err := keylog.idx.Iter(seek, func(eid []byte) error {
		entry, er := keylog.entries.Get(eid)
		if er == nil {
			er = cb(entry)
		}
		return er
	})

	return err
}
