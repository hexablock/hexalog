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
func (store *InMemIndexStore) NewKey(key []byte) (KeylogIndex, error) {
	k := string(key)

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.m[k]; ok {
		return nil, hexatype.ErrKeyExists
	}

	kli := NewInMemKeylogIndex(key, nil)
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

	kli := NewInMemKeylogIndex(key, marker)

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
	}

	return hexatype.ErrKeyNotFound
}

// Iter iterates over each key and index
func (store *InMemIndexStore) Iter(cb func([]byte, KeylogIndex) error) error {
	store.mu.RLock()
	defer store.mu.RUnlock()

	for k, v := range store.m {
		if err := cb([]byte(k), v); err != nil {
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
// hexatype.KeylogIndex with a mutex for safety.  This index interface is meant to be
// implemented based on the backend persistent store used.
type InMemKeylogIndex struct {
	mu sync.RWMutex
	// in-mem index
	idx *hexatype.KeylogIndex
}

// NewInMemKeylogIndex instantiates a new in-memory KeylogIndex
func NewInMemKeylogIndex(key, marker []byte) *InMemKeylogIndex {
	idx := &InMemKeylogIndex{idx: hexatype.NewKeylogIndex(key)}
	idx.idx.Marker = marker
	return idx
}

// Key returns the key for the index
func (idx *InMemKeylogIndex) Key() []byte {
	return idx.idx.Key
}

// Marker returns the marker value
func (idx *InMemKeylogIndex) Marker() []byte {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Marker
}

// SetMarker sets the marker for the index.  It returns true if the marker is not part of
// the index and was set.  This never returns an error as it is in-memory
func (idx *InMemKeylogIndex) SetMarker(marker []byte) (bool, error) {
	idx.mu.Lock()
	ok := idx.idx.SetMarker(marker)
	idx.mu.Unlock()
	return ok, nil
}

// Append appends the id to the index checking the previous hash.
func (idx *InMemKeylogIndex) Append(id, prev []byte) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.idx.Append(id, prev)
}

// Rollback safely removes the last entry id
func (idx *InMemKeylogIndex) Rollback() (int, bool) {
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

// Contains safely returns if the entry id is in the index
func (idx *InMemKeylogIndex) Contains(id []byte) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Contains(id)
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

// Height returns the height of the log in a thread safe way
func (idx *InMemKeylogIndex) Height() uint32 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Height
}

// Index returns the KeylogIndex index struct.  It is meant be used as readonly
func (idx *InMemKeylogIndex) Index() hexatype.KeylogIndex {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return *idx.idx
}
