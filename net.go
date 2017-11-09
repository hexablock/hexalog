package hexalog

import (
	"io"

	"time"

	"golang.org/x/net/context"

	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

type KeySeedStream struct {
	o    *rpcOutConn
	pool *outPool
	HexalogRPC_SeedKeysRPCClient
}

// Recycle recycles the stream returning the conn back to the pool
func (stream *KeySeedStream) Recycle() {
	stream.pool.returnConn(stream.o)
}

// NetTransport is the network transport interface used to make remote calls.
type NetTransport struct {
	hlog *Hexalog
	pool *outPool
}

// NewNetTransport initializes a NetTransport with an outbound connection pool.
func NewNetTransport(reapInterval, maxConnIdle time.Duration) *NetTransport {
	trans := &NetTransport{
		pool: newOutPool(maxConnIdle, reapInterval),
	}

	go trans.pool.reapOld()

	return trans
}

// NewEntry requests a new Entry for a given key from the remote host.
func (trans *NetTransport) NewEntry(host string, key []byte, opts *RequestOptions) (*Entry, error) {
	conn, err := trans.pool.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &ReqResp{Entry: &Entry{Key: key}, Options: opts}
	resp, err := conn.client.NewRPC(context.Background(), req)
	trans.pool.returnConn(conn)
	if err != nil {
		return nil, hexatype.ParseGRPCError(err)
	}

	return resp.Entry, nil
}

// ProposeEntry makes a Propose request on a remote host
func (trans *NetTransport) ProposeEntry(ctx context.Context, host string, entry *Entry, opts *RequestOptions) (*ReqResp, error) {
	conn, err := trans.pool.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &ReqResp{Entry: entry, Options: opts}
	resp, err := conn.client.ProposeRPC(ctx, req)
	trans.pool.returnConn(conn)

	if err != nil {
		err = hexatype.ParseGRPCError(err)
	}

	return resp, err
}

// CommitEntry makes a Commit request on a remote host
func (trans *NetTransport) CommitEntry(ctx context.Context, host string, entry *Entry, opts *RequestOptions) error {
	conn, err := trans.pool.getConn(host)
	if err != nil {
		return err
	}

	req := &ReqResp{Entry: entry, Options: opts}
	if _, err = conn.client.CommitRPC(ctx, req); err != nil {
		err = hexatype.ParseGRPCError(err)
	}
	trans.pool.returnConn(conn)

	return err
}

// LastEntry retrieves the last entry for the given key from a remote host.
func (trans *NetTransport) LastEntry(host string, key []byte, opts *RequestOptions) (*Entry, error) {
	conn, err := trans.pool.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &ReqResp{Entry: &Entry{Key: key}, Options: opts}
	resp, err := conn.client.LastRPC(context.Background(), req)
	trans.pool.returnConn(conn)

	if err != nil {
		return nil, hexatype.ParseGRPCError(err)
	}

	return resp.Entry, nil
}

// GetEntry makes a Get request to retrieve an Entry from a remote host.
func (trans *NetTransport) GetEntry(host string, key, id []byte, opts *RequestOptions) (*Entry, error) {
	conn, err := trans.pool.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &ReqResp{ID: id, Entry: &Entry{Key: key}, Options: opts}
	resp, err := conn.client.GetRPC(context.Background(), req)
	trans.pool.returnConn(conn)

	if err != nil {
		return nil, hexatype.ParseGRPCError(err)
	}

	return resp.Entry, nil
}

// GetSeedKeys gets KeySeeds from a host.
func (trans *NetTransport) GetSeedKeys(host string) (*KeySeedStream, error) {
	conn, err := trans.pool.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &ReqResp{}
	stream, err := conn.client.SeedKeysRPC(context.Background(), req)
	if err != nil {
		trans.pool.returnConn(conn)
		return nil, hexatype.ParseGRPCError(err)
	}

	kss := &KeySeedStream{
		o:    conn,
		pool: trans.pool,
		HexalogRPC_SeedKeysRPCClient: stream,
	}

	return kss, nil
}

// Register registers the log with the transport to serve RPC requests to the log
func (trans *NetTransport) Register(hlog *Hexalog) {
	trans.hlog = hlog
}

// ProposeRPC serves a Propose request.  The underlying ballot from the local log is ignored
func (trans *NetTransport) ProposeRPC(ctx context.Context, req *ReqResp) (*ReqResp, error) {
	resp := &ReqResp{}

	ballot, err := trans.hlog.Propose(req.Entry, req.Options)
	if err != nil {
		return resp, err
	}

	opt := req.Options
	if !opt.WaitBallot {
		return resp, nil
	}

	if err = ballot.Wait(); err != nil {
		return resp, err
	}
	resp.BallotTime = ballot.Runtime().Nanoseconds()

	if opt.WaitApply {
		fut := ballot.Future()
		_, err = fut.Wait(time.Duration(opt.WaitApplyTimeout) * time.Millisecond)
		resp.ApplyTime = fut.Runtime().Nanoseconds()
	}

	return resp, err
}

// CommitRPC serves a Commit request.  The underlying ballot from the local log is ignored
func (trans *NetTransport) CommitRPC(ctx context.Context, req *ReqResp) (*ReqResp, error) {
	resp := &ReqResp{}
	_, err := trans.hlog.Commit(req.Entry, req.Options)
	return resp, err
}

// GetRPC serves a Commit request.  The underlying ballot from the local log is ignored
func (trans *NetTransport) GetRPC(ctx context.Context, req *ReqResp) (resp *ReqResp, err error) {
	resp = &ReqResp{}
	resp.Entry, err = trans.hlog.Get(req.Entry.Key, req.ID)
	return
}

// LastRPC serves a LastEntry request.  It returns the local last entry for a key.
func (trans *NetTransport) LastRPC(ctx context.Context, req *ReqResp) (resp *ReqResp, err error) {
	resp = &ReqResp{
		Entry: trans.hlog.store.LastEntry(req.Entry.Key),
	}
	return
}

// NewRPC serves a NewEntry request from the local log
func (trans *NetTransport) NewRPC(ctx context.Context, req *ReqResp) (*ReqResp, error) {
	return &ReqResp{
		Entry: trans.hlog.New(req.Entry.Key),
	}, nil
}

// PullKeylog fetches the key log from the given host starting at the entry.  If the
// previous hash of the entry is nil, then all entries for the key are fetched. It appends
// each entry directly to the log and submits to the FSM and returns a FutureEntry which
// is the last entry that was appended to the key log and/or an error.  The keylog must
// exist in order for the log to be sucessfully fetched.
func (trans *NetTransport) PullKeylog(host string, entry *Entry, opts *RequestOptions) (*FutureEntry, error) {
	conn, err := trans.pool.getConn(host)
	if err != nil {
		return nil, err
	}
	stream, err := conn.client.PullKeylogRPC(context.Background())
	if err != nil {
		return nil, err
	}

	// Send the entry we want to start at.  Only generate an id for the entry if the
	// previous hash is not nil.  Remote assumes all log entries if the request id is nil.
	req := &ReqResp{Entry: entry}
	if entry.Previous != nil {
		req.ID = entry.Hash(trans.hlog.conf.Hasher())
	}

	log.Printf("[DEBUG] Fetching host=%s key=%s height=%d prev=%x", host, entry.Key, entry.Height, entry.Previous)
	if err = stream.Send(req); err != nil {
		return nil, hexatype.ParseGRPCError(err)
	}

	// This is the last entry that has been applied to the FSM used to wait
	var fentry *FutureEntry

	// Start recieving entries
	for {
		var msg *ReqResp
		if msg, err = stream.Recv(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		//
		// TODO: validate
		//

		// Continue on if already have the entry.  This should be checked against the
		// index as the entry may exist but not been applied to the index.

		if fentry, err = trans.hlog.append(msg.ID, msg.Entry); err != nil {
			log.Printf("[ERROR] Fetch key entry key=%s height=%d error='%v'", entry.Key, entry.Height, err)
		}

	}

	return fentry, hexatype.ParseGRPCError(err)
}

// PullKeylogRPC streams log entries for key to the caller based on the request
func (trans *NetTransport) PullKeylogRPC(stream HexalogRPC_PullKeylogRPCServer) error {
	// Get request
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	ent := req.Entry
	// Make sure a key is specified in the request
	if ent == nil || ent.Key == nil || len(ent.Key) == 0 {
		log.Printf("[ERROR] Invalid entry/key: %#v", req)
		return hexatype.ErrKeyInvalid
	}

	// Get the key log
	keylog, err := trans.hlog.store.GetKey(ent.Key)
	if err != nil {
		return err
	}
	defer keylog.Close()

	// Get the requested seek position from the request id.  If it is nil assume
	// all log entries need to be sent
	var seek []byte
	if req.ID != nil {
		seek = req.ID
	}

	// Start sending entries starting at the id in the request
	err = keylog.Iter(seek, func(id []byte, entry *Entry) error {
		resp := &ReqResp{Entry: entry, ID: id}
		return stream.Send(resp)
	})

	return err
}

// PushKeylog pushes a key log directly from the local store to the remote
// host
func (trans *NetTransport) PushKeylog(host string, key []byte, opts *RequestOptions) error {
	// Get the keylog
	keylog, err := trans.hlog.store.GetKey(key)
	if err != nil {
		return err
	}
	defer keylog.Close()

	last := keylog.LastEntry()
	// Nothing to transfer
	if last == nil {
		return nil
	}

	// Get the connection
	conn, err := trans.pool.getConn(host)
	if err != nil {
		return err
	}
	// Get the stream from the connection
	stream, err := conn.client.PushKeylogRPC(context.Background())
	if err != nil {
		return err
	}

	// Send request preamble with the local last entry letting the remote know
	// what key we are requesting
	preamble := &ReqResp{Entry: last}
	if err = stream.Send(preamble); err != nil {
		return err
	}

	// Recieve preamble response with last entry of remote
	if preamble, err = stream.Recv(); err != nil {
		return hexatype.ParseGRPCError(err)
	}

	// Get the seek position based on last entry sent from remote
	var seek []byte
	if preamble.Entry != nil {
		seek = preamble.Entry.Hash(trans.hlog.conf.Hasher())
	}

	// Iterate based on seek position and send to remote
	err = keylog.Iter(seek, func(id []byte, entry *Entry) error {
		if err = stream.Send(&ReqResp{ID: id, Entry: entry}); err != nil {
			return err
		}
		return nil
	})

	// Return the close error only if there were not any previous errors
	if er := stream.CloseSend(); er != nil && err == nil {
		err = er
	}

	// TODO: check for if key needs to be removed

	return err
}

// PushKeylogRPC accepts a transfer request for a key initiated by a remote host.
// The key must first be initialized before a transfer is accepted.  It returns
// an ErrKeyNotFound if the key has not been locally initialized or any
// underlying error
func (trans *NetTransport) PushKeylogRPC(stream HexalogRPC_PushKeylogRPCServer) error {
	// Get request and check it.  The ID in this request is the location id of
	// the key.
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	if req.Entry == nil || req.Entry.Key == nil || len(req.Entry.Key) == 0 {
		return hexatype.ErrKeyInvalid
	}

	// Check for existence of the key locally
	var keylog *Keylog
	if keylog, err = trans.hlog.store.GetKey(req.Entry.Key); err != nil {
		return err
	}
	defer keylog.Close()

	// Send last entry we have for the key back to requester
	preamble := &ReqResp{
		Entry: keylog.LastEntry(),
	}
	if err = stream.Send(preamble); err != nil {
		return err
	}

	// Future to record the last entry applied
	var fentry *FutureEntry

	// Start receiving entries from the remote host
	for {
		var msg *ReqResp
		if msg, err = stream.Recv(); err != nil {
			if err == io.EOF {
				// TODO: may need to wait for entry to be applied
				err = nil
			}
			break
		}

		// Continue on if we already have the entry
		if _, er := keylog.GetEntry(msg.ID); er != nil {
			continue
		}

		// TODO: optimize to skip to the latest height.

		log.Printf("[DEBUG] Take over id=%x key=%s height=%d prev=%x",
			msg.ID, msg.Entry.Key, msg.Entry.Height, msg.Entry.Previous)

		// We append to the keylog rather than the log here as we have already
		// gotten the key and would be more efficient
		if er := keylog.AppendEntry(msg.Entry); er != nil {
			log.Printf("[ERROR] Failed to accept key entry transfer key=%s height=%d error='%v'",
				msg.Entry.Key, msg.Entry.Height, er)
			continue
		}

		fentry = NewFutureEntry(msg.ID, msg.Entry)
		trans.hlog.fsm.apply(fentry)

	}

	return err
}

// SeedKeysRPC builds KeySeeds using the last entry for a key and streams them
// to the requestor
func (trans *NetTransport) SeedKeysRPC(req *ReqResp, stream HexalogRPC_SeedKeysRPCServer) error {
	err := trans.hlog.store.IterKeySeed(func(seed *KeySeed) error {
		return stream.Send(seed)
	})

	return err
}

// Shutdown signals the transport to be shutdown and closes all outbound
// connections.
func (trans *NetTransport) Shutdown() {
	trans.pool.shutdown()
}
