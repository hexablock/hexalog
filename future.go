package hexalog

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

var (
	errTimedOut = errors.New("timed out")
)

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

	// Time the entry was set to be applied to the fsm.
	dispatched time.Time

	// Time the entry was applied to the fsm.  This is set when a call to applied is made.
	completed time.Time
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
	entry.completed = time.Now()

	if err != nil {
		entry.err <- err
	} else {
		entry.done <- data
	}
}

// dispatch sets the dispatched time to the current time
func (entry *FutureEntry) dispatch() {
	entry.dispatched = time.Now()
}

// Wait blocks until the entry has been applied to the FSM or the timeout has been reached
// It returns data from the fsm.Apply response or an error
func (entry *FutureEntry) Wait(timeout time.Duration) (resp interface{}, err error) {
	select {
	case resp = <-entry.done:
		// resp will be th response from the application fsm.  If it returns a nil a struct{}{}
		// is written instead
	case err = <-entry.err:
		// error returned from application fsm.
	case <-time.After(timeout):
		// The caller defined timeout reached
		err = errTimedOut
	}

	return
}

// Runtime returns the time taken from when the entry was set to be applied to the FSM
// until it was actually applied to the FSM
func (entry *FutureEntry) Runtime() time.Duration {
	return entry.completed.Sub(entry.dispatched)
}
