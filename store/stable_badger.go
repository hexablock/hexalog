package store

import (
	"github.com/dgraph-io/badger"
	"github.com/hexablock/hexatype"
)

// BadgerStableStore implements a stable storage to persist FSM state.
type BadgerStableStore struct {
	*baseBadger
}

// NewBadgerStableStore instantiates a new Badger key-value store backed stable store.
func NewBadgerStableStore(dataDir string) *BadgerStableStore {
	return &BadgerStableStore{
		baseBadger: newBaseBadger(dataDir),
	}
}

// Get retrieves a key either from memory or from the persistent store
func (store *BadgerStableStore) Get(key []byte) ([]byte, error) {
	val, _, err := store.get(key)
	if err != nil {
		return nil, err
	} else if val == nil {
		return nil, hexatype.ErrKeyNotFound
	}

	return val, nil
}

// Set writes the key value to the in memory map.
func (store *BadgerStableStore) Set(key, val []byte) error {
	return store.kv.Set(key, val, 0)
}

// Iter iterates of each key and associated entry
func (store *BadgerStableStore) Iter(cb func([]byte, []byte) error) error {
	opt := new(badger.IteratorOptions)
	*opt = badger.DefaultIteratorOptions

	it := store.kv.NewIterator(*opt)

	var err error
	for it.Rewind(); it.Valid(); it.Next() {
		item := it.Item()
		key := item.Key()
		if key == nil {
			break
		}
		//	 Skip nils
		var val []byte
		err = item.Value(func(v []byte) error {
			val = make([]byte, len(v))
			copy(val, v)
			return nil
		})

		if err != nil {
			break
		}

		if val == nil {
			continue
		}

		if err = cb(key, val); err != nil {
			break
		}
	}

	return err
}
