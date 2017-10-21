package hexalog

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
		PeerSet:       make([]*Participant, len(o.PeerSet)),
		RetryInterval: o.RetryInterval,
		Retries:       o.Retries,
		LTime:         o.LTime,
	}
	copy(opts.PeerSet, o.PeerSet)

	return opts
}

// SourcePeer returns the peer for the source index
func (o *RequestOptions) SourcePeer() *Participant {
	return o.PeerSet[o.SourceIndex]
}
