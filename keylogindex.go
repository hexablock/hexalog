package hexalog

import (
	"sync"
)

// KeylogIndex is the index interface for a keylog.
type KeylogIndex interface {
	// Key for the keylog index
	Key() []byte
	// SetMarker sets the marker
	SetMarker(id []byte) (bool, error)
	// returns the marker
	Marker() []byte

	// Append id checking previous is equal to prev.  Ltime is lamport time
	// attached to the entry
	Append(id, prev []byte, ltime uint64) error

	// Remove last entry and set the ltime.  The ltime here should be that of
	// the previous entry
	Rollback(ltime uint64) (int, bool)

	// Last entry id
	Last() []byte
	// Contains the entry id or not
	Contains(id []byte) bool
	// Iterate each entry id issuing the callback for each bailing on error
	Iter(seek []byte, cb func(id []byte) error) error
	// Number of entries
	Count() int
	// Height of the keylog
	Height() uint32
	// Index object used as readonly
	Index() UnsafeKeylogIndex
	// Flush to disk and remove from mem
	Close() error
}

// SafeKeylogIndex implements an in-memory KeylogIndex interface.  It simply wraps the
// KeylogIndex with a mutex for safety.
type SafeKeylogIndex struct {
	mu sync.RWMutex
	// in-mem index
	idx *UnsafeKeylogIndex
}

// NewSafeKeylogIndex instantiates a new in-memory KeylogIndex safe for concurrent
// access.
func NewSafeKeylogIndex(key, marker []byte) *SafeKeylogIndex {
	idx := &SafeKeylogIndex{idx: NewUnsafeKeylogIndex(key)}
	idx.idx.Marker = marker
	return idx
}

// Key returns the key for the index
func (idx *SafeKeylogIndex) Key() []byte {
	return idx.idx.Key
}

// Marker returns the marker value
func (idx *SafeKeylogIndex) Marker() []byte {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Marker
}

// SetMarker sets the marker for the index.  It returns true if the marker is not part of
// the index and was set.  This never returns an error as it is in-memory
func (idx *SafeKeylogIndex) SetMarker(marker []byte) (bool, error) {
	idx.mu.Lock()
	ok := idx.idx.SetMarker(marker)
	idx.mu.Unlock()
	return ok, nil
}

// Append appends the id to the index checking the previous hash.
func (idx *SafeKeylogIndex) Append(id, prev []byte, ltime uint64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.idx.Append(id, prev, ltime)
}

// Rollback safely removes the last entry id
func (idx *SafeKeylogIndex) Rollback(ltime uint64) (int, bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.idx.Rollback(ltime)
}

// Last safely returns the last entry id
func (idx *SafeKeylogIndex) Last() []byte {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Last()
}

// Contains safely returns if the entry id is in the index
func (idx *SafeKeylogIndex) Contains(id []byte) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Contains(id)
}

// Iter iterates over each entry id in the index
func (idx *SafeKeylogIndex) Iter(seek []byte, cb func(id []byte) error) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Iter(seek, cb)
}

// Count returns the number of entries in the index
func (idx *SafeKeylogIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Count()
}

// Height returns the height of the log in a thread safe way
func (idx *SafeKeylogIndex) Height() uint32 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Height
}

// Index returns the KeylogIndex index struct.  It is meant be used as readonly
// point in time.
func (idx *SafeKeylogIndex) Index() UnsafeKeylogIndex {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return *idx.idx
}

// Close is a noop
func (idx *SafeKeylogIndex) Close() error {
	return nil
}

func isZeroBytes(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
