package hexalog

import (
	"encoding/hex"
	"encoding/json"
)

// MarshalJSON is used to marshal id to hex string
func (part Participant) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string
		Host     string
		Priority int32
		Index    int32
	}{
		ID:       hex.EncodeToString(part.ID),
		Host:     part.Host,
		Priority: part.Priority,
		Index:    part.Index,
	})
}
