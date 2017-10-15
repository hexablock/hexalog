package hexalog

import (
	"sort"
	"sync"

	"github.com/hexablock/hexatype"
)

// InMemIndexStore implements an in-memory KeylogIndex store interface
type InMemIndexStore struct {
	mu sync.RWMutex
	m  map[string]KeylogIndex
}

// NewInMemIndexStore initializes an in-memory store for KeylogIndexes.
func NewInMemIndexStore() *InMemIndexStore {
	return &InMemIndexStore{m: make(map[string]KeylogIndex)}
}

// Name returns the name of the index store
func (store *InMemIndexStore) Name() string {
	return storeNameInmem
}

// NewKey creates a new KeylogIndex and adds it to the store.  It returns an error if it
// already exists
func (store *InMemIndexStore) NewKey(key []byte) (KeylogIndex, error) {
	k := string(key)

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.m[k]; ok {
		return nil, hexatype.ErrKeyExists
	}

	kli := NewSafeKeylogIndex(key, nil)
	store.m[k] = kli

	return kli, nil
}

// MarkKey sets the marker on the key key.  If the key does not exist a new one is created.
// It returns the KeylogIndex or an error.
func (store *InMemIndexStore) MarkKey(key, marker []byte) (KeylogIndex, error) {
	k := string(key)

	store.mu.RLock()
	if v, ok := store.m[k]; ok {
		store.mu.RUnlock()
		_, err := v.SetMarker(marker)
		return v, err
	}
	store.mu.RUnlock()

	kli := NewSafeKeylogIndex(key, marker)

	store.mu.Lock()
	store.m[k] = kli
	store.mu.Unlock()
	return kli, nil
}

// GetKey returns a KeylogIndex from the store or an error if not found
func (store *InMemIndexStore) GetKey(key []byte) (KeylogIndex, error) {
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
		return nil
	}

	return hexatype.ErrKeyNotFound
}

// Iter iterates over each key and index
func (store *InMemIndexStore) Iter(cb func([]byte, KeylogIndex) error) error {
	store.mu.RLock()
	defer store.mu.RUnlock()

	keys := store.sortedKeys()

	for _, k := range keys {
		if err := cb([]byte(k), store.m[k]); err != nil {
			return err
		}
	}
	return nil
}

// Count returns the total number of keys in the index
func (store *InMemIndexStore) Count() int64 {
	store.mu.RLock()
	defer store.mu.RUnlock()

	return int64(len(store.m))
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

// Close is a noop
func (store *InMemIndexStore) Close() error {
	return nil
}
