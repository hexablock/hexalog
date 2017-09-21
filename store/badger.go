package store

import "github.com/dgraph-io/badger"

type baseBadger struct {
	opt *badger.Options
	kv  *badger.KV
}

func newBaseBadger(dataDir string) *baseBadger {
	opt := new(badger.Options)
	*opt = badger.DefaultOptions
	opt.Dir = dataDir
	opt.ValueDir = dataDir
	// TODO: handle this a better way
	opt.SyncWrites = true

	return &baseBadger{
		opt: opt,
	}

}

func (store *baseBadger) Open() (err error) {
	store.kv, err = badger.NewKV(store.opt)
	return
}

func (store *baseBadger) Close() error {
	return store.kv.Close()
}

func (store *baseBadger) get(key []byte) ([]byte, uint64, error) {
	var item badger.KVItem
	err := store.kv.Get(key, &item)
	if err != nil {
		return nil, 0, err
	}

	var val []byte
	err = item.Value(func(v []byte) error {
		if v != nil {
			val = make([]byte, len(v))
			copy(val, v)
		}
		return nil
	})

	return val, item.Counter(), err
}
