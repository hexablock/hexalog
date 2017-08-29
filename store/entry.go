package store

import (
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hexablock/hexatype"
)

// InMemEntryStore is an in-memory implementation of an EntryStore
type InMemEntryStore struct {
	mu sync.RWMutex
	m  map[string]*hexatype.Entry
}

// NewInMemEntryStore creates a new in memory EntryStore initializing the internal map
func NewInMemEntryStore() *InMemEntryStore {
	return &InMemEntryStore{m: make(map[string]*hexatype.Entry)}
}

// Get tries to get an entry by or returns an error
func (store *InMemEntryStore) Get(id []byte) (*hexatype.Entry, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	if v, ok := store.m[string(id)]; ok {
		return v, nil
	}

	return nil, hexatype.ErrEntryNotFound
}

// Set sets the entry in the store by the given id.  It returns no errors
func (store *InMemEntryStore) Set(id []byte, entry *hexatype.Entry) error {
	store.mu.Lock()
	store.m[string(id)] = entry
	store.mu.Unlock()
	return nil
}

// Delete removes an entry from the store by the given id. It returns an error if the
// entry does not exist.
func (store *InMemEntryStore) Delete(id []byte) error {
	sid := string(id)

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.m[sid]; ok {
		delete(store.m, sid)
		return nil
	}

	return hexatype.ErrEntryNotFound
}

// Count returns the number of entries currently in the store.
func (store *InMemEntryStore) Count() int {
	store.mu.RLock()
	defer store.mu.RUnlock()

	return len(store.m)
}

// Close is a no-op to satisfy the EntryStore interface
func (store *InMemEntryStore) Close() error {
	return nil
}

// BadgerEntryStore is an EntryStore using badger for persistence
type BadgerEntryStore struct {
	*baseBadger
}

// NewBadgerEntryStore instantiates a badger based entry store
func NewBadgerEntryStore(dataDir string) *BadgerEntryStore {
	return &BadgerEntryStore{
		baseBadger: newBaseBadger(dataDir),
	}
}

// Get gets an entry by id
func (store *BadgerEntryStore) Get(id []byte) (*hexatype.Entry, error) {
	val, _, err := store.get(id)
	if err != nil {
		return nil, err
	} else if val == nil {
		// The key is consider to exist only if the value is not nil.  This is due to how badger
		// performs deletes.  This needs to  verified.
		return nil, hexatype.ErrEntryNotFound
	}

	entry := &hexatype.Entry{}
	err = proto.Unmarshal(val, entry)
	return entry, err
}

// Set sets the entry to the given id.  The id should be an entry hash
func (store *BadgerEntryStore) Set(id []byte, entry *hexatype.Entry) error {
	val, err := proto.Marshal(entry)
	if err == nil {
		return store.kv.Set(id, val, 0)
	}
	return err
}

// Delete removes the entry with the given id from the store
func (store *BadgerEntryStore) Delete(id []byte) error {
	return store.kv.Delete(id)
}
