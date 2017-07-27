package hexalog

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	"github.com/hexablock/log"
)

var (
	errNotLastEntry = errors.New("not last entry")
)

// Common field for stores to share.
type keylogStore struct {
	Key []byte
	// Location id
	locationID []byte
	// Hash function used to generate id's
	hasher Hasher
}

// InMemKeylogStore contains all log entries for a key.
type InMemKeylogStore struct {
	*keylogStore
	// Log entries for the key.
	mu      sync.RWMutex
	entries []*Entry
}

// NewInMemKeylogStore initializes a new log for a key. It takes a key, location id and hash function used
// to compute hash id's of log entries.
func NewInMemKeylogStore(key, locationID []byte, hasher Hasher) *InMemKeylogStore {
	return &InMemKeylogStore{
		keylogStore: &keylogStore{Key: key, hasher: hasher, locationID: locationID},
		entries:     []*Entry{},
	}
}

// Height returns the height of the log i.e the height value of the last entry in the store
func (keylog *InMemKeylogStore) Height() uint32 {
	keylog.mu.RLock()
	defer keylog.mu.RUnlock()

	if len(keylog.entries) == 0 {
		return 0
	}

	e := keylog.entries[len(keylog.entries)-1]
	return e.Height
}

// LocationID returns the location id for the key
func (keylog *InMemKeylogStore) LocationID() []byte {
	return keylog.locationID
}

// LastEntry returns the last entry in the InMemKeylogStore.  If there are no entries in the log ,
// nil is returned.
func (keylog *InMemKeylogStore) LastEntry() *Entry {
	keylog.mu.RLock()
	defer keylog.mu.RUnlock()

	if len(keylog.entries) > 0 {
		return keylog.entries[len(keylog.entries)-1]
	}
	return nil
}

// AppendEntry appends an entry to the log.  It returns an error if there is a previous
// hash mismatch
func (keylog *InMemKeylogStore) AppendEntry(entry *Entry) error {

	lastHash, currHeight := keylog.lastHashHeight()

	if bytes.Compare(entry.Previous, lastHash) != 0 {
		log.Printf("[ERROR] Previous hash mismatch key=%s curr-height=%d new-height=%d want=%x have=%x",
			entry.Key, currHeight, entry.Height, lastHash, entry.Previous)
		return errPreviousHash
	}

	keylog.mu.Lock()
	defer keylog.mu.Unlock()

	keylog.entries = append(keylog.entries, entry)
	log.Printf("[DEBUG] Appended key=%s last=%x id=%x", entry.Key, lastHash, entry.Hash(keylog.hasher.New()))

	return nil
}

// GetEntry gets and entry from the InMemKeylogStore
func (keylog *InMemKeylogStore) GetEntry(id []byte) (*Entry, error) {
	keylog.mu.RLock()
	defer keylog.mu.RUnlock()

	for _, v := range keylog.entries {
		if bytes.Compare(id, v.Hash(keylog.hasher.New())) == 0 {
			return v, nil
		}
	}

	return nil, ErrEntryNotFound
}

// RollbackEntry rolls the keylog back by the entry.  It only rolls back if the entry is
// the last entry.  It returns the number of entries remaining in the log and/or an error
func (keylog *InMemKeylogStore) RollbackEntry(entry *Entry) (int, error) {
	hasher := keylog.hasher.New()
	id := entry.Hash(hasher)
	// Reset for re-use
	hasher.Reset()

	last := keylog.LastEntry()
	if last == nil {
		// Return 0 entry count
		return 0, ErrEntryNotFound
	}

	lid := last.Hash(hasher)

	if bytes.Compare(id, lid) != 0 {
		return -1, errNotLastEntry
	}

	keylog.mu.Lock()
	keylog.entries = keylog.entries[:len(keylog.entries)-1]
	keylog.mu.Unlock()

	return int(entry.Height) - 1, nil
}

// Entries returns all log entries in the key log
func (keylog *InMemKeylogStore) Entries() []*Entry {
	keylog.mu.RLock()
	defer keylog.mu.RUnlock()

	return keylog.entries
}

// Iter iterates over entries starting from the seek position.  It iterates over all
// entries if seek is nil
func (keylog *InMemKeylogStore) Iter(seek []byte, cb func(entry *Entry) error) error {
	keylog.mu.RLock()
	defer keylog.mu.RUnlock()

	s := -1
	if seek != nil {
		// Seek to the position
		h := keylog.hasher.New()
		//keylog.mu.RLock()
		for i, e := range keylog.entries {
			h.Reset()
			id := e.Hash(h)

			if equalBytes(seek, id[:]) {
				s = i
				break
			}

		}

		// Return error if we can't find the seek position
		if s < 0 {
			//keylog.mu.RUnlock()
			return errNotFound
		}

	} else {
		// Iterate all entries if seek is nil
		s = 0
		//keylog.mu.RLock()
	}

	// Issue callback based on seek
	l := len(keylog.entries)
	var err error
	for i := s; i < l; i++ {
		if err = cb(keylog.entries[i]); err != nil {
			break
		}
	}

	//keylog.mu.RUnlock()

	return err
}

// MarshalJSON is a custom marshaller for the Keylog
func (keylog *InMemKeylogStore) MarshalJSON() ([]byte, error) {
	keylog.mu.RLock()
	defer keylog.mu.RUnlock()

	m := map[string]interface{}{
		"Key":        string(keylog.Key),
		"LocationID": hex.EncodeToString(keylog.locationID),
		"Entries":    keylog.entries,
	}

	return json.Marshal(m)
}

// lastHashHeight returns the last hash and height or a zero hash if no entries exist
func (keylog *InMemKeylogStore) lastHashHeight() (lastID []byte, height uint32) {
	last := keylog.LastEntry()
	if last == nil {
		lastID = make([]byte, keylog.hasher.New().Size())
		height = 0
	} else {
		lastID = last.Hash(keylog.hasher.New())
		height = last.Height
	}
	return
}

// func (keylog *InMemKeylogStore) Snapshot() (KeylogStore, error) {
// 	ks := NewInMemKeylogStore(keylog.Key, keylog.locationID, keylog.hasher)
//
// 	ks.entries = make([]*Entry, len(ks.entries))
// 	for i, entry := range keylog.entries {
// 		ks.entries[i] = entry.Clone()
// 	}
//
// 	return ks, nil
// }
