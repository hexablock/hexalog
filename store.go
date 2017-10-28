package hexalog

const (
	// In-memory store name
	storeNameInmem = "in-memory"
)

// IndexStore implements a datastore for the log indexes.  It contains all keys
// on a node with their associated keylog.  The interface must be thread-safe
type IndexStore interface {
	// Name of the store
	Name() string

	// Create a new KeylogIndex and add it to the store.
	NewKey(key []byte) (KeylogIndex, error)

	// Get a KeylogIndex from the store
	GetKey(key []byte) (KeylogIndex, error)

	// Create and/or get a KeylogIndex setting the marker if it is created.
	MarkKey(key []byte, marker []byte) (KeylogIndex, error)

	// Remove key if exists or return an error
	RemoveKey(key []byte) error

	// Iterate over each keylog index
	Iter(cb func(key []byte, kli KeylogIndex) error) error

	// Total number of keys in the index
	Count() int64

	// Close the index store
	Close() error
}

// EntryStore implements a datastore for log entries.
type EntryStore interface {
	// Name of store
	Name() string

	// Get an entry by id
	Get(id []byte) (*Entry, error)

	// Set an entry by the id
	Set(id []byte, entry *Entry) error

	// Delete an entry by id
	Delete(id []byte) error

	// Returns the number of entries in the store
	Count() int64

	// Close the store
	Close() error
}
