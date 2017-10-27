package hexalog

import (
	"crypto/sha1"
	"hash"
	"time"
)

// Config holds the configuration for the log.  This is used to initialize the log.
type Config struct {
	Hostname           string
	HealBufSize        int           // Buffer size for heal requests
	BroadcastBufSize   int           // proposal and commit broadcast buffer
	BallotReapInterval time.Duration // interval at which old ballots are cleaned up
	TTL                time.Duration // ttl for each ballot
	Votes              int           // minimum votes required
	Hasher             func() hash.Hash
	hashSize           int
}

// DefaultConfig returns a sane set of default configurations.  The default hash function
// used is SHA1
func DefaultConfig(hostname string) *Config {
	return &Config{
		Hostname:           hostname,
		BroadcastBufSize:   32,
		HealBufSize:        32,
		BallotReapInterval: 30 * time.Second,
		TTL:                3 * time.Second,
		Votes:              3,        // Minimum votes
		Hasher:             sha1.New, // hash function
	}

}
