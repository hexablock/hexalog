package hexalog

import (
	"hash"
	"time"

	"github.com/hexablock/log"
)

// LogStore is the whole log containing all keys.  It manages serialization of
// operations and all validation and checks required therein.
type LogStore struct {
	// KeylogIndex datastore interface
	index IndexStore

	// Entry datastore interface
	entries EntryStore

	// Hash function to use when calculating id's
	hasher   func() hash.Hash
	hashSize int
}

// NewLogStore initializes a new in-memory log store
func NewLogStore(entries EntryStore, index IndexStore, hasher func() hash.Hash) *LogStore {
	ls := &LogStore{
		hasher:   hasher,
		entries:  entries,
		index:    index,
		hashSize: hasher().Size(),
	}
	log.Printf("[INFO] Hexalog store type='entry' name='%s'", entries.Name())
	log.Printf("[INFO] Hexalog store type='index' name='%s'", index.Name())
	return ls
}

// NewKey creates a new keylog for a key.  It returns an error if the key already exists
func (hlog *LogStore) NewKey(key []byte) (keylog *Keylog, err error) {
	idx, err := hlog.index.NewKey(key)
	if err == nil {
		return NewKeylog(hlog.entries, idx, hlog.hasher), nil
	}

	return nil, err
}

// GetKey returns the log for the given key
func (hlog *LogStore) GetKey(key []byte) (keylog *Keylog, err error) {
	idx, err := hlog.index.GetKey(key)
	if err == nil {
		return NewKeylog(hlog.entries, idx, hlog.hasher), nil
	}

	return nil, err
}

// RemoveKey marks a key log to be removed.  It is actually removed during compaction
func (hlog *LogStore) RemoveKey(key []byte) error {
	keylog, err := hlog.GetKey(key)
	if err != nil {
		return err
	}
	defer keylog.Close()

	if err = hlog.index.RemoveKey(key); err == nil {
		err = keylog.Iter(nil, func(id []byte, entry *Entry) error {
			//
			// TODO: Schedule removal rather than doing inline
			//
			if er := hlog.entries.Delete(id); er != nil {
				log.Printf("[ERROR] Failed to remove entry: %v key=%s id=%x",
					er, key, id)
			}

			return nil
		})
	}

	return err
}

// GetEntry gets an entry by key and id of the entry
func (hlog *LogStore) GetEntry(key, id []byte) (*Entry, error) {
	return hlog.entries.Get(id)
}

// LastEntry gets the last entry for a key form the log
func (hlog *LogStore) LastEntry(key []byte) *Entry {
	idx, err := hlog.index.GetKey(key)
	if err != nil {
		return nil
	}
	defer idx.Close()

	lh := idx.Last()
	if lh == nil {
		return nil
	}

	entry, _ := hlog.entries.Get(lh)
	return entry
}

// NewEntry sets the previous hash and height for a new Entry and returns it
func (hlog *LogStore) NewEntry(key []byte) *Entry {
	var (
		prev   []byte
		height uint32
	)

	idx, err := hlog.index.GetKey(key)
	if err == nil {
		prev = idx.Last()
		height = uint32(idx.Count() + 1)
		defer idx.Close()
	}
	// First entry
	if prev == nil {
		prev = make([]byte, hlog.hashSize)
		height = 1
	}

	return &Entry{
		Key:       key,
		Previous:  prev,
		Height:    height,
		Timestamp: uint64(time.Now().UnixNano()),
	}
}

// RollbackEntry rolls the log back to the given entry.  The entry must be the last entry in
// the log
func (hlog *LogStore) RollbackEntry(entry *Entry) error {
	keylog, err := hlog.GetKey(entry.Key)
	if err != nil {
		return err
	}
	defer keylog.Close()

	c, err := keylog.RollbackEntry(entry)
	// If we have no entries in the log after the rollback, remove the Keylog completely
	if c == 0 {
		if er := hlog.RemoveKey(entry.Key); er != nil {
			log.Printf("[ERROR] Logstore failed to remove empty key key=%s error='%v'", entry.Key, er)
		}
	}

	// Return rollback error
	return err
}

// IterKeySeed iterates of KeySeed objects
func (hlog *LogStore) IterKeySeed(f func(*KeySeed) error) error {
	err := hlog.index.Iter(func(key []byte, kli KeylogIndex) error {
		idx := kli.Index()

		marker := idx.Marker
		if marker == nil {
			marker = idx.Last()
		}

		seed := &KeySeed{
			Key:    key,
			Marker: marker,
			Height: kli.Height(),
			LTime:  idx.LTime,
		}

		return f(seed)
	})
	return err
}

// AppendEntry appends an entry to a KeyLog.  If the key does not exist it returns an error
func (hlog *LogStore) AppendEntry(entry *Entry) error {
	keylog, err := hlog.GetKey(entry.Key)
	if err == nil {
		defer keylog.Close()
		return keylog.AppendEntry(entry)
	}
	return err
}

// Stats returns a Stats object with the store related stats only
func (hlog *LogStore) Stats() *Stats {
	return &Stats{
		Keys:    hlog.index.Count(),
		Entries: hlog.entries.Count(),
	}
}
