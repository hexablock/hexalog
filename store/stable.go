package store

import (
	"errors"
	"sync"

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

// Iter iterates over each key issuing the callback for each key and entry pair. The keys
// are not sorted.
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
