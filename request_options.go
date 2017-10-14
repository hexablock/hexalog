package hexalog

import "github.com/hexablock/hexaring"

// DefaultRequestOptions returns sane options for a request.  The PeerSet
// still must be set if needed.
func DefaultRequestOptions() *RequestOptions {
	return &RequestOptions{
		RetryInterval:    20, // msec
		Retries:          3,
		WaitApplyTimeout: 2000, // msec
	}
}

// CloneWithSourceIndex clones RequestOptions assigning it the provided idx as the SourceIndex
func (o *RequestOptions) CloneWithSourceIndex(idx int32) *RequestOptions {
	opts := &RequestOptions{
		SourceIndex:   idx,
		PeerSet:       make([]*hexaring.Location, len(o.PeerSet)),
		RetryInterval: o.RetryInterval,
		Retries:       o.Retries,
		LTime:         o.LTime,
	}
	copy(opts.PeerSet, o.PeerSet)

	return opts
}

// SourcePeer returns the peer for the source index
func (o *RequestOptions) SourcePeer() *hexaring.Location {
	return o.PeerSet[o.SourceIndex]
}

// LocationSet returns the peer locations as a LocationSet
func (o *RequestOptions) LocationSet() hexaring.LocationSet {
	return hexaring.LocationSet(o.PeerSet)
}
