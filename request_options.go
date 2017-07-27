package hexalog

import "github.com/hexablock/hexaring"

// CloneWithSourceIndex clones RequestOptions assigning it the provided idx as the SourceIndex
func (o *RequestOptions) CloneWithSourceIndex(idx int32) *RequestOptions {
	opts := &RequestOptions{
		SourceIndex: idx,
		PeerSet:     make([]*hexaring.Location, len(o.PeerSet)),
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
