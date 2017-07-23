package hexalog

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEntry_Clone(t *testing.T) {
	e1 := &Entry{
		Key:       []byte("k"),
		Previous:  []byte("prev"),
		Timestamp: uint64(time.Now().UnixNano()),
		Height:    2,
		Data:      []byte("data"),
	}

	e2 := e1.Clone()

	if string(e1.Key) != string(e2.Key) {
		t.Fatal("key mismatch")
	}

	if string(e1.Data) != string(e2.Data) {
		t.Fatal("data mismatch")
	}

	if string(e1.Previous) != string(e2.Previous) {
		t.Fatal("previous mismatch")
	}

	if e1.Timestamp != e2.Timestamp {
		t.Fatal("time mismatch")
	}
	if e1.Height != e2.Height {
		t.Fatal("time mismatch")
	}

	if _, err := json.Marshal(e1); err != nil {
		t.Fatal(err)
	}
}
