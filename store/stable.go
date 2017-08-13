package store

import (
	"errors"
	"sync"
)

var (
	errNotFound = errors.New("not found")
)

// InMemStableStore implements an in-memory StableStore interface
type InMemStableStore struct {
	mu sync.RWMutex
	m  map[string][]byte
}

// Open initializes the in-memory data structure to begin writing.  This must be called
// before attempting to write or read data.
func (store *InMemStableStore) Open() error {
	store.m = make(map[string][]byte)
	return nil
}

// Get gets a key from the in-memory data structure
func (store *InMemStableStore) Get(key []byte) ([]byte, error) {
	store.mu.RLock()
	if val, ok := store.m[string(key)]; ok {
		defer store.mu.RUnlock()
		return val, nil
	}
	store.mu.RUnlock()

	return nil, errNotFound
}

// Set sets a key to the value to the in memory structure
func (store *InMemStableStore) Set(key, value []byte) error {
	store.mu.Lock()
	store.m[string(key)] = value
	store.mu.Unlock()
	return nil
}

// Close does nothing aside from satsifying the StableStore interface
func (store *InMemStableStore) Close() error {
	return nil
}

// // BadgerStableStore implements a stable storage to persist FSM state.
// type BadgerStableStore struct {
// 	opt badger.Options
// 	bdg *badger.KV
// }
//
// // NewBadgerStableStore instantiates a new Badger key-value store backed stable store.
// func NewBadgerStableStore(opt badger.Options) *BadgerStableStore {
// 	return &BadgerStableStore{
// 		opt: opt,
// 	}
// }
//
// // Open opens the store.  This needs to be called before operations can be made against
// // the store.
// func (store *BadgerStableStore) Open() (err error) {
// 	store.bdg, err = badger.NewKV(&store.opt)
// 	return
// }
//
// // Get retrieves a key either from memory or from the persistent store
// func (store *BadgerStableStore) Get(key []byte) (val []byte, err error) {
// 	// Get key from the persistent store
// 	var item badger.KVItem
// 	if err = store.bdg.Get(key, &item); err == nil {
// 		val = item.Value()
// 	}
//
// 	return
// }
//
// // Set writes the key value to the in memory map.
// func (store *BadgerStableStore) Set(key, value []byte) error {
// 	return store.bdg.Set(key, value)
// }
//
// // Close closes the store
// func (store *BadgerStableStore) Close() error {
// 	return store.bdg.Close()
// }
