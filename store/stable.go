package store

import (
	"errors"
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/hexablock/hexatype"
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

	return nil, hexatype.ErrKeyNotFound
}

// Set sets a key to the value to the in memory structure
func (store *InMemStableStore) Set(key, value []byte) error {
	store.mu.Lock()
	store.m[string(key)] = value
	store.mu.Unlock()
	return nil
}

// Iter iterates over each key issuing the callback for each key and entry pair
func (store *InMemStableStore) Iter(cb func([]byte, []byte) error) error {
	var err error
	store.mu.RLock()
	for k, v := range store.m {
		if err = cb([]byte(k), v); err != nil {
			break
		}
	}
	store.mu.RUnlock()
	return err
}

// Close does nothing aside from satsifying the StableStore interface
func (store *InMemStableStore) Close() error {
	return nil
}

// BadgerStableStore implements a stable storage to persist FSM state.
type BadgerStableStore struct {
	*baseBadger
}

// NewBadgerStableStore instantiates a new Badger key-value store backed stable store.
func NewBadgerStableStore(dataDir string) *BadgerStableStore {
	return &BadgerStableStore{
		baseBadger: newBaseBadger(dataDir),
	}
}

// Get retrieves a key either from memory or from the persistent store
func (store *BadgerStableStore) Get(key []byte) ([]byte, error) {
	val, _, err := store.get(key)
	if err != nil {
		return nil, err
	} else if val == nil {
		return nil, hexatype.ErrKeyNotFound
	}

	return val, nil
}

// Set writes the key value to the in memory map.
func (store *BadgerStableStore) Set(key, val []byte) error {
	return store.kv.Set(key, val, 0)
}

// Iter iterates of each key and associated entry
func (store *BadgerStableStore) Iter(cb func([]byte, []byte) error) error {
	opt := new(badger.IteratorOptions)
	*opt = badger.DefaultIteratorOptions

	it := store.kv.NewIterator(*opt)

	var err error
	for it.Rewind(); it.Valid(); it.Next() {
		item := it.Item()
		key := item.Key()
		if key == nil {
			break
		}
		//	 Skip nils
		var val []byte
		err = item.Value(func(v []byte) {
			val = make([]byte, len(v))
			copy(val, v)
		})

		if err != nil {
			break
		}

		if val == nil {
			continue
		}

		if err = cb(key, val); err != nil {
			break
		}
	}

	return err
}
