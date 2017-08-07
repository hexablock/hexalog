package store

import (
	"log"
	"sort"
	"sync"
	"time"

	"github.com/hexablock/hexatype"
)

// KeylogStore implements a storage interface to store logs for a given key
type KeylogStore interface {
	LocationID() []byte
	//Height() uint32
	Iter(seek []byte, cb func(entry *hexatype.Entry) error) error
	GetEntry(id []byte) (*hexatype.Entry, error)
	LastEntry() *hexatype.Entry
	AppendEntry(entry *hexatype.Entry) error
	RollbackEntry(entry *hexatype.Entry) (int, error)
}

// InMemLogStore is the whole log containing all keys.  It manages serialization of
// operations and all validation and checks required therein.
type InMemLogStore struct {
	// Actual key log store.
	mu sync.RWMutex
	m  map[string]KeylogStore
	// Hash to use when calculating id's
	hasher hexatype.Hasher
}

// NewInMemLogStore initializes a new in-memory log store
func NewInMemLogStore(hasher hexatype.Hasher) *InMemLogStore {
	return &InMemLogStore{
		m:      make(map[string]KeylogStore),
		hasher: hasher,
	}
}

// Iter iterates over all keys in the store.  It acquires a read-lock and should be used
// keeping that in mind
func (hlog *InMemLogStore) Iter(cb func(string, []byte)) {
	hlog.mu.RLock()
	defer hlog.mu.RUnlock()

	keys := hlog.sortedKeys()
	for _, k := range keys {

		kl := hlog.m[k]
		cb(k, kl.LocationID())
	}

}

func (hlog *InMemLogStore) sortedKeys() []string {
	keys := make([]string, len(hlog.m))
	var i int
	for k := range hlog.m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

// GetEntry gets an entry by key and id of the entry
func (hlog *InMemLogStore) GetEntry(key, id []byte) (*hexatype.Entry, error) {
	k := string(key)

	hlog.mu.RLock()
	defer hlog.mu.RUnlock()

	if kl, ok := hlog.m[k]; ok {
		return kl.GetEntry(id)
	}

	return nil, hexatype.ErrEntryNotFound
}

// LastEntry gets the last entry for a key form the log
func (hlog *InMemLogStore) LastEntry(key []byte) *hexatype.Entry {
	k := string(key)

	hlog.mu.RLock()
	if kl, ok := hlog.m[k]; ok {
		defer hlog.mu.RUnlock()
		return kl.LastEntry()
	}
	hlog.mu.RUnlock()
	return nil
}

// NewKey creates a new keylog for a key with the locationID.  It returns an error if the
// key already exists
func (hlog *InMemLogStore) NewKey(key, locationID []byte) (keylog KeylogStore, err error) {
	k := string(key)

	hlog.mu.Lock()
	if _, ok := hlog.m[k]; !ok {
		keylog = NewInMemKeylogStore(key, locationID, hlog.hasher)
		hlog.m[k] = keylog
	} else {
		err = hexatype.ErrKeyExists
	}
	hlog.mu.Unlock()

	return
}

// NewEntry sets the previous hash and height for a new Entry and returns it
func (hlog *InMemLogStore) NewEntry(key []byte) *hexatype.Entry {

	var (
		h             = hlog.hasher.New()
		prev          = make([]byte, h.Size())
		height uint32 = 1
		k             = string(key)
	)

	// Try to get the last entry hash id for the key
	hlog.mu.Lock()
	if kl, ok := hlog.m[k]; ok {
		// Get prev hash from last entry
		if last := kl.LastEntry(); last != nil {
			prev = last.Hash(h)
			height = last.Height + 1
		}

	}
	hlog.mu.Unlock()

	return &hexatype.Entry{
		Key:       key,
		Previous:  prev,
		Height:    height,
		Timestamp: uint64(time.Now().UnixNano()),
	}
}

// RollbackEntry rolls the log back to the given entry.  The entry must be the last entry in
// the log
func (hlog *InMemLogStore) RollbackEntry(entry *hexatype.Entry) error {
	key := string(entry.Key)

	hlog.mu.RLock()
	keylog, ok := hlog.m[key]
	if !ok {
		hlog.mu.RUnlock()
		return hexatype.ErrKeyNotFound
	}
	hlog.mu.RUnlock()

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

// GetKey returns the log for the given key
func (hlog *InMemLogStore) GetKey(key []byte) (keylog KeylogStore, err error) {
	var (
		k  = string(key)
		ok bool
	)

	hlog.mu.RLock()
	defer hlog.mu.RUnlock()

	if keylog, ok = hlog.m[k]; !ok {
		err = hexatype.ErrKeyNotFound
	}

	return
}

// RemoveKey marks a key log to be removed.  It is actually removed during compaction
func (hlog *InMemLogStore) RemoveKey(key []byte) error {
	k := string(key)

	hlog.mu.Lock()
	if _, ok := hlog.m[k]; ok {
		delete(hlog.m, k)
	}
	hlog.mu.Unlock()

	return hexatype.ErrKeyNotFound
}

// AppendEntry appends an entry to a KeyLog.  If the key does not exist it returns an error
func (hlog *InMemLogStore) AppendEntry(entry *hexatype.Entry) error {
	k := string(entry.Key)

	hlog.mu.Lock()
	klog, ok := hlog.m[k]
	if !ok {
		return hexatype.ErrKeyNotFound
	}

	if err := klog.AppendEntry(entry); err != nil {
		hlog.mu.Unlock()
		return err
	}

	hlog.m[k] = klog
	hlog.mu.Unlock()

	return nil
}
