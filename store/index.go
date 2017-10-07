package store

import (
	"sort"
	"sync"

	"github.com/hexablock/hexatype"
)

// InMemIndexStore implements an in-memory KeylogIndex store interface
type InMemIndexStore struct {
	mu sync.RWMutex
	m  map[string]hexatype.KeylogIndex
}

// NewInMemIndexStore initializes an in-memory store for KeylogIndexes.
func NewInMemIndexStore() *InMemIndexStore {
	return &InMemIndexStore{m: make(map[string]hexatype.KeylogIndex)}
}

// NewKey creates a new KeylogIndex and adds it to the store.  It returns an error if it
// already exists
func (store *InMemIndexStore) NewKey(key []byte) (hexatype.KeylogIndex, error) {
	k := string(key)

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.m[k]; ok {
		return nil, hexatype.ErrKeyExists
	}

	kli := hexatype.NewSafeKeylogIndex(key, nil)
	store.m[k] = kli

	return kli, nil
}

// MarkKey sets the marker on the key key.  If the key does not exist a new one is created.
// It returns the KeylogIndex or an error.
func (store *InMemIndexStore) MarkKey(key, marker []byte) (hexatype.KeylogIndex, error) {
	k := string(key)
	store.mu.RLock()
	if v, ok := store.m[k]; ok {
		store.mu.RUnlock()
		_, err := v.SetMarker(marker)
		return v, err
	}
	store.mu.RUnlock()

	kli := hexatype.NewSafeKeylogIndex(key, marker)

	store.mu.Lock()
	store.m[k] = kli
	store.mu.Unlock()
	return kli, nil
}

// GetKey returns a KeylogIndex from the store or an error if not found
func (store *InMemIndexStore) GetKey(key []byte) (hexatype.KeylogIndex, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	if v, ok := store.m[string(key)]; ok {
		return v, nil
	}

	return nil, hexatype.ErrKeyNotFound
}

// RemoveKey removes the given key's index from the store.  It does NOT remove the associated
// entry hash id's
func (store *InMemIndexStore) RemoveKey(key []byte) error {
	k := string(key)

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.m[k]; ok {
		delete(store.m, k)
	}

	return hexatype.ErrKeyNotFound
}

// Iter iterates over each key and index
func (store *InMemIndexStore) Iter(cb func([]byte, hexatype.KeylogIndex) error) error {
	store.mu.RLock()
	defer store.mu.RUnlock()

	for k, v := range store.m {
		if err := cb([]byte(k), v); err != nil {
			return err
		}
	}
	return nil
}

// KeyCount returns the total number of keys in the index
func (store *InMemIndexStore) KeyCount() int {
	store.mu.RLock()
	defer store.mu.RUnlock()

	return len(store.m)
}

func (store *InMemIndexStore) sortedKeys() []string {
	keys := make([]string, len(store.m))
	var i int
	for k := range store.m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}
