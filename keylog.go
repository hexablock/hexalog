package hexalog

import (
	"bytes"
	"errors"

	"github.com/hexablock/hexatype"
)

var (
	errNotLastEntry = errors.New("not last entry")
)

// Keylog represents a log for a given key.  It manages operations against log entries for
// a key.
type Keylog struct {
	// Datastore containing entries
	entries EntryStore
	// KeylogIndex interface
	idx KeylogIndex
	// Hash function
	hasher hexatype.Hasher
}

// NewKeylog initializes a new log for a key. It takes a key, location id and hash function used
// to compute hash id's of log entries.
func NewKeylog(entries EntryStore, idx KeylogIndex, hasher hexatype.Hasher) *Keylog {
	return &Keylog{
		hasher:  hasher,
		entries: entries,
		idx:     idx,
	}
}

// LastEntry returns the last entry in the Keylog.  If there are no entries in the log ,
// nil is returned.
func (keylog *Keylog) LastEntry() *Entry {
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
func (keylog *Keylog) AppendEntry(entry *Entry) (err error) {
	id := entry.Hash(keylog.hasher.New())

	if err = keylog.entries.Set(id, entry); err != nil {
		return
	}

	// Add id to index.  The index will check for order based on the supplied previous.  We
	// do not rollback the above entry from the entry store on error as only entries in the
	// index matter.
	err = keylog.idx.Append(id, entry.Previous)

	//
	// TODO: Cleanup residual entries via some deferred process
	//

	return
}

// GetEntry gets and entry from the Keylog
func (keylog *Keylog) GetEntry(id []byte) (*Entry, error) {
	return keylog.entries.Get(id)
}

// RollbackEntry rolls the keylog back by the entry.  It only rolls back if the entry is
// the last entry.  It returns the number of entries remaining in the log and any errors
// occurred
func (keylog *Keylog) RollbackEntry(entry *Entry) (int, error) {
	// Compute hash id outside of lock
	id := entry.Hash(keylog.hasher.New())

	lid := keylog.idx.Last()
	if lid == nil {
		// Return current entry with the error
		return keylog.idx.Count(), hexatype.ErrEntryNotFound
	}

	if bytes.Compare(id, lid) != 0 {
		return -1, errNotLastEntry
	}

	// Remove the entry from the EntryStore
	if err := keylog.entries.Delete(id); err != nil {
		return keylog.idx.Count(), err
	}

	// Remove id from index
	n, _ := keylog.idx.Rollback()
	return n, nil
}

// Iter iterates over entries starting from the seek position.  It iterates over all
// entries if seek is nil
func (keylog *Keylog) Iter(seek []byte, cb func(id []byte, entry *Entry) error) error {

	err := keylog.idx.Iter(seek, func(eid []byte) error {
		entry, er := keylog.entries.Get(eid)
		if er == nil {
			er = cb(eid, entry)
		}
		return er
	})

	return err
}

// GetIndex returns a KeylogIndex struct
func (keylog *Keylog) GetIndex() UnsafeKeylogIndex {
	return keylog.idx.Index()
}
