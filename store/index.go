package store

import (
	"sync"

	"github.com/hexablock/hexatype"
)

// KeylogIndex is the the index interface for a keylog
type KeylogIndex interface {
	LocationID() []byte
	Append(id, prev []byte) error
	Rollback() int
	Last() []byte
	Iter(seek []byte, cb func(id []byte) error) error
	Count() int
}

// InMemKeylogIndex implements an in memory KeylogIndex.  It simply wraps the
// hexatype.KeylogIndex with a mutex
type InMemKeylogIndex struct {
	mu  sync.RWMutex
	idx *hexatype.KeylogIndex
}

// NewInMemKeylogIndex instantiates a new in-memory KeylogIndex
func NewInMemKeylogIndex(key, locID []byte) *InMemKeylogIndex {
	return &InMemKeylogIndex{idx: hexatype.NewKeylogIndex(key, locID)}
}

func (idx *InMemKeylogIndex) LocationID() []byte {
	return idx.idx.Location
}

// Append appends the id to the index checking the previous hash.
func (idx *InMemKeylogIndex) Append(id, prev []byte) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.idx.Append(id, prev)
}

func (idx *InMemKeylogIndex) Rollback() int {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.idx.Rollback()
}

func (idx *InMemKeylogIndex) Last() []byte {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Last()
}

func (idx *InMemKeylogIndex) Iter(seek []byte, cb func(id []byte) error) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Iter(seek, cb)
}

func (idx *InMemKeylogIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Count()
}
