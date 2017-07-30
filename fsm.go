package hexalog

import (
	"errors"

	"github.com/hexablock/log"
)

var (
	errTimedOut = errors.New("timed out")
)

// FSM is the application finite-state-machine.  This is implemented by the application
// using the library. It should return an interface or an error.  The return value is only
// checked for an error type internally.
type FSM interface {
	Apply(entryID []byte, entry *Entry) interface{}
}

// StableStore is the interface used to store the FSM state.  It contains information about
// the current fsm state.
type StableStore interface {
	Open() error
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Close() error
}

// EchoFSM implements the FSM interface that simply echos the entry to stdout.
type EchoFSM struct{}

// Apply simply logs the Entry to stdout
func (fsm *EchoFSM) Apply(entryID []byte, entry *Entry) interface{} {
	log.Printf("[INFO] Applied FSM key=%s id=%x height=%d", entry.Key, entryID, entry.Height)
	return map[string]string{"status": "ok"}
}

// fsm is the internal FSM handles operations that need to be performed before and/or
// after calling the application FSM along with queueing and updating the FSM state.
type fsm struct {
	// Application FSM
	f FSM
	// Channel for entries to be applied to the FSM
	applyCh chan *FutureEntry
	// Stable store to track which entries have been applied to the fsm.
	ss StableStore

	hasher Hasher
}

// newFsm initializes a new fsm with the given stable store and hash function.  It opens
// the store and starts the applying in a separate go routine.
func newFsm(f FSM, ss StableStore, hasher Hasher) (*fsm, error) {
	fsm := &fsm{
		f:       f,
		applyCh: make(chan *FutureEntry),
		ss:      ss,
		hasher:  hasher,
	}

	err := ss.Open()
	if err == nil {
		//
		// TODO: THIS MAY NEED TO HAPPEN ELSEWHERE
		// - Check stable store for last applied position
		// - Apply all remaining entries
		//
		go fsm.startApply()
	}

	return fsm, err
}

// apply queues the FutureEntry to the FSM.
func (fsm *fsm) apply(entry *FutureEntry) {
	entry.dispatch()
	fsm.applyCh <- entry
}

func (fsm *fsm) startApply() {

	for fentry := range fsm.applyCh {
		var (
			e1    error
			data  interface{} = struct{}{}
			entry             = fentry.Entry
		)

		// Apply entry to application FSM
		if resp := fsm.f.Apply(fentry.ID(), entry); resp != nil {
			// Check if the response is an error otherwise make the fsm data available
			if e, ok := resp.(error); ok {
				e1 = e
			} else {
				// Set the app fsm response
				data = resp
			}
		}

		// Commit the last fsm applied entry to stable store
		e2 := fsm.ss.Set(entry.Key, entry.Hash(fsm.hasher.New()))

		// Signal future that we applied the entry supplying the app fsm response or any errors
		// encountered
		fentry.applied(data, mergeErrors(e1, e2))
		log.Printf("[INFO] Applied key=%s height=%d runtime=%v", entry.Key, entry.Height,
			fentry.Runtime())
	}

}
