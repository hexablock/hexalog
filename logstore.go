package hexalog

import (
	"time"

	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

// IndexStore implements a datastore for the log indexes.  It contains all keys on a node
// with their associated keylog.  The interface must be thread-safe
type IndexStore interface {
	// Create a new KeylogIndex and add it to the store.
	NewKey(key []byte) (hexatype.KeylogIndex, error)
	// Get a KeylogIndex from the store
	GetKey(key []byte) (hexatype.KeylogIndex, error)
	// Create and/or get a KeylogIndex setting the marker if it is created.
	MarkKey(key []byte, marker []byte) (hexatype.KeylogIndex, error)
	// Remove key if exists or return an error
	RemoveKey(key []byte) error
	// Iterate over each key
	Iter(cb func(key []byte, kli hexatype.KeylogIndex) error) error
}

// EntryStore implements a datastore for log entries.
type EntryStore interface {
	Get(id []byte) (*hexatype.Entry, error)
	Set(id []byte, entry *hexatype.Entry) error
	Delete(id []byte) error
	Close() error
}

// LogStore is the whole log containing all keys.  It manages serialization of
// operations and all validation and checks required therein.
type LogStore struct {
	// KeylogIndex datastore interface
	index IndexStore
	// Entry datastore interface
	entries EntryStore
	// Hash function to use when calculating id's
	hasher hexatype.Hasher
}

// NewLogStore initializes a new in-memory log store
func NewLogStore(entries EntryStore, index IndexStore, hasher hexatype.Hasher) *LogStore {
	return &LogStore{
		hasher:  hasher,
		entries: entries,
		index:   index,
	}
}

// NewKey creates a new keylog for a key.  It returns an error if the key already exists
func (hlog *LogStore) NewKey(key []byte) (keylog *Keylog, err error) {
	idx, err := hlog.index.NewKey(key)
	if err != nil {
		return nil, err
	}
	return NewKeylog(hlog.entries, idx, hlog.hasher), nil
}

// GetEntry gets an entry by key and id of the entry
func (hlog *LogStore) GetEntry(key, id []byte) (*hexatype.Entry, error) {
	return hlog.entries.Get(id)
}

// LastEntry gets the last entry for a key form the log
func (hlog *LogStore) LastEntry(key []byte) *hexatype.Entry {
	idx, err := hlog.index.GetKey(key)
	if err != nil {
		return nil
	}

	lh := idx.Last()
	if lh == nil {
		return nil
	}

	entry, _ := hlog.entries.Get(lh)
	return entry
}

// NewEntry sets the previous hash and height for a new Entry and returns it
func (hlog *LogStore) NewEntry(key []byte) *hexatype.Entry {
	var (
		prev   []byte
		height uint32
	)

	idx, err := hlog.index.GetKey(key)
	if err == nil {
		prev = idx.Last()
		height = uint32(idx.Count() + 1)
	}
	// First entry
	if prev == nil {
		prev = make([]byte, hlog.hasher.New().Size())
		height = 1
	}

	return &hexatype.Entry{
		Key:       key,
		Previous:  prev,
		Height:    height,
		Timestamp: uint64(time.Now().UnixNano()),
	}
}

// RollbackEntry rolls the log back to the given entry.  The entry must be the last entry in
// the log
func (hlog *LogStore) RollbackEntry(entry *hexatype.Entry) error {
	keylog, err := hlog.GetKey(entry.Key)
	if err != nil {
		return err
	}

	c, err := keylog.RollbackEntry(entry)
	// If we have no entries in the log after the rollback, remove the Keylog completely
	if c == 0 {
		if er := hlog.RemoveKey(entry.Key); er != nil {
			log.Printf("[ERROR] Logstore failed to remove empty key key=%s error='%v'", entry.Key, er)
		}
	}

	// Return rollback error
	return err
}

// GetKey returns the log for the given key
func (hlog *LogStore) GetKey(key []byte) (keylog *Keylog, err error) {
	idx, err := hlog.index.GetKey(key)
	if err == nil {
		return NewKeylog(hlog.entries, idx, hlog.hasher), nil
	}

	return nil, err
}

// RemoveKey marks a key log to be removed.  It is actually removed during compaction
func (hlog *LogStore) RemoveKey(key []byte) error {
	keylog, err := hlog.GetKey(key)
	if err != nil {
		return err
	}

	if err = hlog.index.RemoveKey(key); err == nil {
		err = keylog.Iter(nil, func(id []byte, entry *hexatype.Entry) error {
			//
			// TODO: Schedule removal of all log entries for the key
			//
			return nil
		})
	}

	return err
}

// AppendEntry appends an entry to a KeyLog.  If the key does not exist it returns an error
func (hlog *LogStore) AppendEntry(entry *hexatype.Entry) error {
	keylog, err := hlog.GetKey(entry.Key)
	if err == nil {
		return keylog.AppendEntry(entry)
	}
	return err
}
