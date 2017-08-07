package store

import (
	"bytes"
	"errors"

	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

var (
	errNotLastEntry = errors.New("not last entry")
)

type keylogBase struct {
	key []byte
	// Location id
	locationID []byte
	// Hash function used to generate id's
	hasher hexatype.Hasher
}

func (klb *keylogBase) LocationID() []byte {
	return klb.locationID
}

func (klb *keylogBase) Key() []byte {
	return klb.key
}

// InMemKeylogStore implements an in-memory KeylogStore
type InMemKeylogStore struct {
	*keylogBase
	// Datastore containing entries by id
	entries EntryStore
	// Index of entry id's for the key
	idx KeylogIndex
}

// NewInMemKeylogStore initializes a new log for a key. It takes a key, location id and hash function used
// to compute hash id's of log entries.
func NewInMemKeylogStore(key, locationID []byte, hasher hexatype.Hasher) *InMemKeylogStore {
	return &InMemKeylogStore{
		keylogBase: &keylogBase{key, locationID, hasher},
		entries:    NewInMemEntryStore(),
		idx:        NewInMemKeylogIndex(key, locationID),
	}
}

// LastEntry returns the last entry in the InMemKeylogStore.  If there are no entries in the log ,
// nil is returned.
func (keylog *InMemKeylogStore) LastEntry() *hexatype.Entry {

	lid := keylog.idx.Last()
	if lid == nil {
		return nil
	}

	entry, _ := keylog.entries.Get(lid)
	return entry
}

// AppendEntry appends an entry to the log.  It returns an error if there is a previous
// hash mismatch
func (keylog *InMemKeylogStore) AppendEntry(entry *hexatype.Entry) (err error) {
	id := entry.Hash(keylog.hasher.New())

	// Add id to index.  The index will check for order
	if err = keylog.idx.Append(id, entry.Previous); err != nil {
		return err
	}

	if err = keylog.entries.Set(id, entry); err == nil {
		log.Printf("[DEBUG] Appended key=%s height=%d id=%x", entry.Key, entry.Height, entry.Hash(keylog.hasher.New()))
	}

	return
}

// GetEntry gets and entry from the InMemKeylogStore
func (keylog *InMemKeylogStore) GetEntry(id []byte) (*hexatype.Entry, error) {
	return keylog.entries.Get(id)
}

// RollbackEntry rolls the keylog back by the entry.  It only rolls back if the entry is
// the last entry.  It returns the number of entries remaining in the log and/or an error
func (keylog *InMemKeylogStore) RollbackEntry(entry *hexatype.Entry) (int, error) {
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
func (keylog *InMemKeylogStore) Iter(seek []byte, cb func(entry *hexatype.Entry) error) error {

	err := keylog.idx.Iter(seek, func(eid []byte) error {
		entry, er := keylog.entries.Get(eid)
		if er == nil {
			er = cb(entry)
		}
		return er
	})

	return err
}
