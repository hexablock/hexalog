package store

// Package store implements the storage interfaces needed for hexalog.  The structure is
// object based containing a keylog index and the entry itself.  Both are housed in
// seperate stores.  The overall storage structure is optimized for a key-value based
// storage system.
//
// The keylog index contains all hash id's of entries for operations on a key.  This in
// turn can be used to lookup and get the complete entry as needed.
//
// The interface definitions are located in the logstore.go file
