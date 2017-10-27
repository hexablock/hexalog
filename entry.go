package hexalog

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
)

// Clone clones an entry
func (entry *Entry) Clone() *Entry {
	return &Entry{
		Key:       entry.Key,
		Previous:  entry.Previous,
		Timestamp: entry.Timestamp,
		LTime:     entry.LTime,
		Height:    entry.Height,
		Data:      entry.Data,
	}
}

// MarshalJSON is a custom JSON marshaler for Entry
func (entry Entry) MarshalJSON() ([]byte, error) {
	m := struct {
		Key       []byte
		Previous  string
		Height    uint32
		Timestamp uint64
		LTime     uint64
		Data      []byte
	}{
		Key:       entry.Key,
		Previous:  hex.EncodeToString(entry.Previous),
		Timestamp: entry.Timestamp,
		LTime:     entry.LTime,
		Height:    entry.Height,
		Data:      entry.Data,
	}
	return json.Marshal(m)
}

// Hash computes the hash of the entry using the hash function
func (entry *Entry) Hash(hashFunc hash.Hash) []byte {
	hashFunc.Write(entry.Previous)
	binary.Write(hashFunc, binary.BigEndian, entry.Timestamp)
	binary.Write(hashFunc, binary.BigEndian, entry.LTime)
	binary.Write(hashFunc, binary.BigEndian, entry.Height)
	hashFunc.Write(entry.Key)
	hashFunc.Write(entry.Data)

	sh := hashFunc.Sum(nil)
	return sh[:]
}
