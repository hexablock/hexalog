package hexalog

import (
	"bytes"
	"errors"

	"github.com/hexablock/hexalog/store"
	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

var (
	errTimedOut = errors.New("timed out")
)

// FSM is the application finite-state-machine.  This is implemented by the application
// using the library. It should return an interface or an error.  The return value is only
// checked for an error type internally.
type FSM interface {
	Apply(entryID []byte, entry *hexatype.Entry) interface{}
}

// StableStore is the interface used to store the FSM state.  It contains information about
// each key and the hash of the last applied entry.  This is used when a node is restarted
// to ensure it is caught up.
type StableStore interface {
	Open() error
	Get(key []byte) ([]byte, error)
	Set(key []byte, value []byte) error
	Iter(cb func([]byte, []byte) error) error
	Close() error
}

// EchoFSM implements the FSM interface that simply echos the entry to stdout.
type EchoFSM struct{}

// Apply simply logs the Entry to stdout
func (fsm *EchoFSM) Apply(entryID []byte, entry *hexatype.Entry) interface{} {
	log.Printf("[INFO] Applied FSM key=%s id=%x height=%d", entry.Key, entryID, entry.Height)
	return map[string]string{"status": "ok"}
}

// fsm is the internal FSM handling operations that need to be performed before and/or
// after calling the application FSM along with queueing and updating the FSM state making
// sure all entries are applied in a consistent manner.
type fsm struct {
	// Application FSM
	f FSM
	// Channel for entries to be applied to the FSM
	applyCh chan *FutureEntry
	// Stable store to track which entries have been applied to the fsm.
	ss StableStore
	// log store to compare against
	ls *LogStore
	// hash function to use
	hasher hexatype.Hasher
}

// newFsm initializes a new fsm with the given application FSM, stable store and hash
// function.  It opens the store and starts the applying in a separate go routine.
func newFsm(f FSM, ss StableStore, logstore *LogStore, hasher hexatype.Hasher) (*fsm, error) {
	fsm := &fsm{
		f:       f,
		applyCh: make(chan *FutureEntry),
		ss:      ss,
		ls:      logstore,
		hasher:  hasher,
	}

	if err := ss.Open(); err != nil {
		return nil, err
	}

	go fsm.startApply()

	return fsm, nil
}

// apply queues the FutureEntry to the FSM.
func (fsm *fsm) apply(entry *FutureEntry) {
	// Start the future timer
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
		e2 := fsm.ss.Set(entry.Key, fentry.ID())
		// Signal future that we applied the entry supplying the app fsm response or any errors
		// encountered
		e := mergeErrors(e1, e2)
		fentry.applied(data, e)
		//log.Printf("[INFO] Applied key=%s height=%d runtime=%v error='%v'", entry.Key, entry.Height, fentry.Runtime(), e)
	}

}

// check checks entries in the logstore against what is committed to the stable store and
// submits entries to be applied to the fsm where it left off
func (fsm *fsm) check() error {
	log.Printf("[INFO] Validating FSM against hexalog")
	return fsm.ls.index.Iter(func(key []byte, ki store.KeylogIndex) error {
		// Last local entry for key
		last := ki.Last()
		seek, err := fsm.ss.Get(key)
		if err == nil {
			// We are ok. Nothing to do
			if bytes.Compare(seek, last) == 0 {
				return nil
			}
		}

		ki.Iter(seek, func(id []byte) error {
			ent, er := fsm.ls.entries.Get(id)
			if er != nil {
				log.Printf("[ERROR] Failed get entry key=%s error='%v'", key, er)
				return nil
			}

			//log.Printf("[DEBUG] Replay key=%s id=%x", ent.Key, id)
			fe := NewFutureEntry(id, ent)
			fsm.apply(fe)

			return nil
		})

		return nil
	})
}
