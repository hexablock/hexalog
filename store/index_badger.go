package store

import (
	"log"
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/golang/protobuf/proto"
	"github.com/hexablock/hexatype"
)

// BadgerIndexStore implements a badger backed IndexStore interface
type BadgerIndexStore struct {
	*baseBadger
}

// NewBadgerIndexStore instantiates a badger based entry store
func NewBadgerIndexStore(dataDir string) *BadgerIndexStore {
	return &BadgerIndexStore{
		baseBadger: newBaseBadger(dataDir),
	}
}

// NewKey creates a new KeylogIndex.  It gets added to the store on the first write
func (store *BadgerIndexStore) NewKey(key []byte) (hexatype.KeylogIndex, error) {
	ok, err := store.kv.Exists(key)
	if err != nil {
		return nil, err
	} else if ok {
		return nil, hexatype.ErrKeyExists
	}

	ki := newBadgerKeylogIndex(store.baseBadger, key)
	err = ki.commit(true)

	return ki, err
}

// GetKey gets a KeylogIndex from the store
func (store *BadgerIndexStore) GetKey(key []byte) (hexatype.KeylogIndex, error) {
	val, c, err := store.get(key)
	if err != nil {
		return nil, err
	} else if val == nil {
		return nil, hexatype.ErrKeyNotFound
	}

	ki := newBadgerKeylogIndex(store.baseBadger, key)
	err = ki.load(val, c)
	return ki, err
}

// MarkKey sets the marker on the key key.  If the key does not exist a new one is created.
// It returns the KeylogIndex or an error.
func (store *BadgerIndexStore) MarkKey(key []byte, marker []byte) (hexatype.KeylogIndex, error) {
	ki, err := store.GetKey(key)
	if err == hexatype.ErrKeyNotFound {
		err = nil
		ki = newBadgerKeylogIndex(store.baseBadger, key)
	}

	if err == nil {
		_, err = ki.SetMarker(marker)
	}

	return ki, err
}

// Iter iterates over each key
func (store *BadgerIndexStore) Iter(cb func([]byte, hexatype.KeylogIndex) error) error {
	opt := new(badger.IteratorOptions)
	*opt = badger.DefaultIteratorOptions
	iter := store.kv.NewIterator(*opt)

	var err error
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		key := item.Key()
		if key == nil {
			break
		}
		var val []byte
		if err = item.Value(func(v []byte) error {
			val = make([]byte, len(v))
			copy(val, v)
			return nil
		}); err != nil {
			return err
		}

		index := newBadgerKeylogIndex(store.baseBadger, key)
		if err = index.load(val, item.Counter()); err == nil {
			err = cb(key, index)
		}

		if err != nil {
			break
		}
	}

	iter.Close()
	return err
}

// RemoveKey removes key if exists or return an error
func (store *BadgerIndexStore) RemoveKey(key []byte) error {
	ok, err := store.kv.Exists(key)
	if err != nil {
		return err
	} else if !ok {
		return hexatype.ErrKeyNotFound
	}

	return store.kv.Delete(key)
}

// BadgerKeylogIndex implements the KeylogIndex interface backed by badger
type BadgerKeylogIndex struct {
	*baseBadger
	key []byte

	mu  sync.RWMutex
	c   uint64                      // cas counter
	idx *hexatype.UnsafeKeylogIndex // in-memory index
}

func newBadgerKeylogIndex(bb *baseBadger, key []byte) *BadgerKeylogIndex {
	return &BadgerKeylogIndex{
		baseBadger: bb,
		key:        key,
		idx:        hexatype.NewUnsafeKeylogIndex(key),
	}
}

// Key returns the key of the index
func (idx *BadgerKeylogIndex) Key() []byte {
	return idx.key
}

// Marker returns the marker value
func (idx *BadgerKeylogIndex) Marker() []byte {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Marker
}

// Last safely returns the last entry id
func (idx *BadgerKeylogIndex) Last() []byte {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Last()
}

// Contains safely returns if the entry id is in the index
func (idx *BadgerKeylogIndex) Contains(id []byte) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Contains(id)
}

// Iter iterates over each entry id in the index
func (idx *BadgerKeylogIndex) Iter(seek []byte, cb func(id []byte) error) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Iter(seek, cb)
}

// Count returns the number of entries in the index
func (idx *BadgerKeylogIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Count()
}

// Height returns the height of the log in a thread safe way
func (idx *BadgerKeylogIndex) Height() uint32 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idx.Height
}

// Index returns the KeylogIndex instance.  This is mean to be used as a readonly snapshot
func (idx *BadgerKeylogIndex) Index() hexatype.UnsafeKeylogIndex {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return *idx.idx
}

// SetMarker sets the marker for the index
func (idx *BadgerKeylogIndex) SetMarker(marker []byte) (bool, error) {
	idx.mu.Lock()
	ok := idx.idx.SetMarker(marker)
	idx.mu.Unlock()

	err := idx.commit(true)
	return ok, err
}

// Append appends the id to the index checking the previous hash.
func (idx *BadgerKeylogIndex) Append(id, prev []byte) error {
	idx.mu.Lock()
	err := idx.idx.Append(id, prev)
	if err == nil {
		idx.mu.Unlock()
		return idx.commit(true)
	}

	idx.mu.Unlock()
	return err
}

// Rollback safely removes the last entry id
func (idx *BadgerKeylogIndex) Rollback() (int, bool) {
	idx.mu.Lock()
	n, ok := idx.idx.Rollback()
	idx.mu.Unlock()
	if ok {
		if err := idx.commit(true); err != nil {
			log.Printf("[ERROR] Failed to commit key=%s error='%v'", idx.key, err)
			return idx.Count(), false
		}
	}
	return n, ok
}

func (idx *BadgerKeylogIndex) commit(reload bool) error {
	// Reload from badger if requested.  This is used in case where calls within this function
	// fail and need to reload state from disk
	if reload {
		defer func() {
			if err := idx.reload(); err != nil {
				log.Printf("[ERROR] Failed to reload index key=%s error='%v'", idx.key, err)
			}
		}()
	}

	idx.mu.RLock()
	val, err := proto.Marshal(idx.idx)
	if err != nil {
		idx.mu.RUnlock()
		return err
	}

	err = idx.kv.CompareAndSet(idx.key, val, idx.c)
	idx.mu.RUnlock()
	return err
}

func (idx *BadgerKeylogIndex) load(val []byte, c uint64) error {
	var index hexatype.UnsafeKeylogIndex
	if err := proto.Unmarshal(val, &index); err != nil {
		return err
	}

	idx.mu.Lock()
	idx.c = c
	idx.idx = &index
	idx.mu.Unlock()
	return nil
}

func (idx *BadgerKeylogIndex) reload() error {
	var item badger.KVItem
	err := idx.kv.Get(idx.key, &item)
	if err != nil {
		return err
	}

	var val []byte
	err = item.Value(func(v []byte) error {
		val = make([]byte, len(v))
		copy(val, v)
		return nil
	})
	if err != nil {
		return err
	}

	return idx.load(val, item.Counter())
}
