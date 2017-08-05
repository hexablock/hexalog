package hexalog

import (
	"crypto/sha1"
	"encoding/json"
	"hash"
)

// HashAlgorithm is the hashing algorithm a Hasher implements. e.g. SHA1 SHA256 SHA512 etc.
type HashAlgorithm string

// Hasher interface implements an interface to create a new instance of a hash function
type Hasher interface {
	New() hash.Hash
	Algorithm() HashAlgorithm
}

// SHA1Hasher implements the Hasher interface for SHA1
type SHA1Hasher struct{}

// New returns a new instance of a SHA1 hasher
func (hasher *SHA1Hasher) New() hash.Hash {
	return sha1.New()
}

// Algorithm returns the hashing algorithm the hasher implements
func (hasher *SHA1Hasher) Algorithm() HashAlgorithm {
	return HashAlgorithm("SHA1")
}

func (hasher SHA1Hasher) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"Algorithm": hasher.Algorithm(),
	})
}
