package store

import (
	"github.com/golang/protobuf/proto"
	"github.com/hexablock/hexatype"
)

// BadgerEntryStore is an EntryStore using badger for persistence
type BadgerEntryStore struct {
	*baseBadger
}

// NewBadgerEntryStore instantiates a badger based entry store
func NewBadgerEntryStore(dataDir string) *BadgerEntryStore {
	return &BadgerEntryStore{
		baseBadger: newBaseBadger(dataDir),
	}
}

// Get gets an entry by id
func (store *BadgerEntryStore) Get(id []byte) (*hexatype.Entry, error) {
	val, _, err := store.get(id)
	if err != nil {
		return nil, err
	} else if val == nil {
		// The key is consider to exist only if the value is not nil.  This is due to how badger
		// performs deletes.  This needs to  verified.
		return nil, hexatype.ErrEntryNotFound
	}

	entry := &hexatype.Entry{}
	err = proto.Unmarshal(val, entry)
	return entry, err
}

// Set sets the entry to the given id.  The id should be an entry hash
func (store *BadgerEntryStore) Set(id []byte, entry *hexatype.Entry) error {
	val, err := proto.Marshal(entry)
	if err == nil {
		return store.kv.Set(id, val, 0)
	}
	return err
}

// Delete removes the entry with the given id from the store
func (store *BadgerEntryStore) Delete(id []byte) error {
	return store.kv.Delete(id)
}
