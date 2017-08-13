package store

import "github.com/hexablock/hexatype"

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
	// Return the number of entries in the store
	Count() int
}

// IndexStore implements a datastore for the log indexes.
type IndexStore interface {
	// Create a new KeylogIndex and add it to the store.
	NewKey(key, locationID []byte) (KeylogIndex, error)
	// Get a KeylogIndex from the store
	GetKey(key []byte) (KeylogIndex, error)
	// Remove key if exists or return an error
	RemoveKey(key []byte) error
	// Iterate over each key and associated location id
	Iter(cb func(key string, locID []byte))
}

// KeylogIndex is the index interface for a keylog.
type KeylogIndex interface {
	// Returns location id for the key
	LocationID() []byte
	// Append id checking previous is equal to prev
	Append(id, prev []byte) error
	// Remove last entry
	Rollback() int
	// Last entry id
	Last() []byte
	// Iterate each entry id issuing the callback for each bailing on error
	Iter(seek []byte, cb func(id []byte) error) error
	// Number of entries
	Count() int
	// Index object used as readonly
	Index() hexatype.KeylogIndex
}
