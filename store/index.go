package store

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

// NewKey creates a new KeylogIndex and adds it to the store.  It returns an error if it
// already exists
func (store *InMemIndexStore) NewKey(key, locID []byte) (KeylogIndex, error) {
	k := string(key)

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.m[k]; ok {
		return nil, hexatype.ErrKeyExists
	}

	kli := NewInMemKeylogIndex(key, locID)
	store.m[k] = kli

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
	}

	return hexatype.ErrKeyNotFound
}

// Iter iterates over each key in lexographical order issuing the callback with the key
// and location id.
func (store *InMemIndexStore) Iter(cb func(string, []byte) error) error {
	store.mu.RLock()
	defer store.mu.RUnlock()

	keys := store.sortedKeys()
	for _, k := range keys {
		kl := store.m[k]
		if err := cb(k, kl.LocationID()); err != nil {
			return err
		}
	}

	return nil
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

// InMemKeylogIndex implements an in-memory KeylogIndex interface.  It simply wraps the
// hexatype.KeylogIndex with a mutex
type InMemKeylogIndex struct {
	mu  sync.RWMutex
	idx *hexatype.KeylogIndex
}

// NewInMemKeylogIndex instantiates a new in-memory KeylogIndex
func NewInMemKeylogIndex(key, locID []byte) *InMemKeylogIndex {
	return &InMemKeylogIndex{idx: hexatype.NewKeylogIndex(key, locID)}
}

// LocationID return the location id for this keylog index.  This does not require a Lock
// as it's only written on initialization.
func (idx *InMemKeylogIndex) LocationID() []byte {
	return idx.idx.Location
}

// Append appends the id to the index checking the previous hash.
func (idx *InMemKeylogIndex) Append(id, prev []byte) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.idx.Append(id, prev)
}

// Rollback safely removes the last entry id
func (idx *InMemKeylogIndex) Rollback() int {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.idx.Rollback()
}

// Last safely returns the last entry id
func (idx *InMemKeylogIndex) Last() []byte {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Last()
}

// Iter iterates over each entry id in the index
func (idx *InMemKeylogIndex) Iter(seek []byte, cb func(id []byte) error) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Iter(seek, cb)
}

// Count returns the number of entries in the index
func (idx *InMemKeylogIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Count()
}

// Index returns the KeylogIndex index struct.  It is meant be used as readonly
func (idx *InMemKeylogIndex) Index() hexatype.KeylogIndex {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return *idx.idx
}
