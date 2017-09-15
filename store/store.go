package store

import (
	"github.com/dgraph-io/badger"
	"github.com/hexablock/hexatype"
)

// Package store implements the storage interfaces needed for hexalog.  The structure is
// object based containing a keylog index and the entry itself.  Both are housed in
// seperate stores.  The overall storage structure is optimized for a key-value based
// storage system.
//
// The keylog index contains all hash id's of entries for operations on a key.  This in
// turn can be used to lookup and get the complete entry as needed.
//
// This file contains the required interfaces for storing log related data.

// EntryStore implements a datastore for log entries.
type EntryStore interface {
	Get(id []byte) (*hexatype.Entry, error)
	Set(id []byte, entry *hexatype.Entry) error
	Delete(id []byte) error
	Close() error
}

// IndexStore implements a datastore for the log indexes.  It contains all keys on a node
// with their associated keylog.  The interface must be thread-safe
type IndexStore interface {
	// Create a new KeylogIndex and add it to the store.
	NewKey(key []byte) (KeylogIndex, error)
	// Get a KeylogIndex from the store
	GetKey(key []byte) (KeylogIndex, error)
	// Create and/or get a KeylogIndex setting the marker if it is created.
	MarkKey(key []byte, marker []byte) (KeylogIndex, error)
	// Remove key if exists or return an error
	RemoveKey(key []byte) error
	// Iterate over each key
	Iter(cb func(key []byte, kli KeylogIndex) error) error
}

// KeylogIndex is the index interface for a keylog.
type KeylogIndex interface {
	// Key for the keylog index
	Key() []byte
	// SetMarker sets the marker
	SetMarker(id []byte) (bool, error)
	// returns the marker
	Marker() []byte
	// Append id checking previous is equal to prev
	Append(id, prev []byte) error
	// Remove last entr
	Rollback() (int, bool)
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
	Index() hexatype.KeylogIndex
}

type baseBadger struct {
	opt *badger.Options
	kv  *badger.KV
}

func newBaseBadger(dataDir string) *baseBadger {
	opt := new(badger.Options)
	*opt = badger.DefaultOptions
	opt.Dir = dataDir
	opt.ValueDir = dataDir
	// TODO: handle this a better way
	opt.SyncWrites = true

	return &baseBadger{
		opt: opt,
	}

}

func (store *baseBadger) Open() (err error) {
	store.kv, err = badger.NewKV(store.opt)
	return
}

func (store *baseBadger) Close() error {
	return store.kv.Close()
}

func (store *baseBadger) get(key []byte) ([]byte, uint64, error) {
	var item badger.KVItem
	err := store.kv.Get(key, &item)
	if err != nil {
		return nil, 0, err
	}

	var val []byte
	err = item.Value(func(v []byte) {
		if v != nil {
			val = make([]byte, len(v))
			copy(val, v)
		}
	})

	return val, item.Counter(), err
}
