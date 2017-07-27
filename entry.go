package hexalog

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
	"time"
)

// Clone clones an entry
func (entry *Entry) Clone() *Entry {
	return &Entry{
		Key:       entry.Key,
		Previous:  entry.Previous,
		Timestamp: entry.Timestamp,
		Height:    entry.Height,
		Data:      entry.Data,
	}
}

// MarshalJSON is a custom JSON marshaler for Entry
func (entry Entry) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"Key":       entry.Key,
		"Previous":  hex.EncodeToString(entry.Previous),
		"Timestamp": entry.Timestamp,
		"Height":    entry.Height,
		"Data":      entry.Data,
	}
	return json.Marshal(m)
}

// Hash computes the hash of the entry using the hash function
func (entry *Entry) Hash(hashFunc hash.Hash) []byte {
	hashFunc.Write(entry.Previous)
	binary.Write(hashFunc, binary.BigEndian, entry.Timestamp)
	binary.Write(hashFunc, binary.BigEndian, entry.Height)
	hashFunc.Write(entry.Key)
	hashFunc.Write(entry.Data)

	sh := hashFunc.Sum(nil)
	return sh[:]
}

// FutureEntry is an entry that been committed to the stable store but has not yet been
// applied to the FSM.
type FutureEntry struct {
	// This is the hash id of the entry that is supplied upon instantiation
	id []byte
	// Entry to be applied
	Entry *Entry
	// Channel used to signal applying entry failed
	err chan error
	// Channel used to signal entry was applied.  It contains the data from that returned
	// from the application fsm i.e. data returned from FSM.Apply
	done chan interface{}
}

// NewFutureEntry instantiates a new FutureEntry with an Entry.  It is an entry that is yet
// to be applied to the log. It can be used to wait for the entry to be applied.  It takes
// the hash id of the entry and entry as parameters.
func NewFutureEntry(id []byte, entry *Entry) *FutureEntry {
	return &FutureEntry{
		id:    id,
		Entry: entry,
		err:   make(chan error, 1),
		done:  make(chan interface{}, 1),
	}
}

// ID returns the id provided upon initiation.  It is used to verify the entry by computing
// the hash of the entry and comparing against this id.
func (entry *FutureEntry) ID() []byte {
	return entry.id
}

// MarshalJSON is a custom marshaller for a FutureEntry
func (entry FutureEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"ID":    hex.EncodeToString(entry.id),
		"Entry": *entry.Entry,
	})
}

// applied is used to signal the future that the entry has been applied.  It takes data
// and an error as an argument to indicate whether it succeeded or failed.  The data is
// only available if the error is nil
func (entry *FutureEntry) applied(data interface{}, err error) {
	if err != nil {
		entry.err <- err
	} else {
		entry.done <- data
	}
}

// Wait blocks until the entry has been applied to the FSM or the timeout has been reached
// It returns data from the fsm.Apply response or an error
func (entry *FutureEntry) Wait(timeout time.Duration) (resp interface{}, err error) {
	select {
	case resp = <-entry.done:
		// resp will be th response from the application fsm.  If it returns a nil a struct{}{}
		// is written instead
	case err = <-entry.err:
	case <-time.After(timeout):
		err = errTimedOut
	}

	return
}
