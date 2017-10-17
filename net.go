package hexalog

import (
	"fmt"
	"io"

	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

type rpcOutConn struct {
	host   string
	conn   *grpc.ClientConn
	client HexalogRPCClient
	used   time.Time
}

// NetTransport is the network transport interface used to make remote calls.
type NetTransport struct {
	hlog *Hexalog

	mu   sync.RWMutex
	pool map[string]*rpcOutConn

	maxConnIdle  time.Duration
	reapInterval time.Duration

	shutdown int32
}

// NewNetTransport initializes a NetTransport with an outbound connection pool.
func NewNetTransport(reapInterval, maxConnIdle time.Duration) *NetTransport {
	trans := &NetTransport{
		pool:         make(map[string]*rpcOutConn),
		maxConnIdle:  maxConnIdle,
		reapInterval: reapInterval,
	}

	go trans.reapOld()

	return trans
}

// NewEntry requests a new Entry for a given key from the remote host.
func (trans *NetTransport) NewEntry(host string, key []byte, opts *RequestOptions) (*Entry, error) {
	conn, err := trans.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &ReqResp{Entry: &Entry{Key: key}, Options: opts}
	resp, err := conn.client.NewRPC(context.Background(), req)
	if err != nil {
		trans.returnConn(conn)
		return nil, hexatype.ParseGRPCError(err)
	}

	return resp.Entry, nil
}

// ProposeEntry makes a Propose request on a remote host
func (trans *NetTransport) ProposeEntry(ctx context.Context, host string, entry *Entry, opts *RequestOptions) error {
	conn, err := trans.getConn(host)
	if err != nil {
		return err
	}

	req := &ReqResp{Entry: entry, Options: opts}
	_, err = conn.client.ProposeRPC(ctx, req)
	trans.returnConn(conn)

	if err != nil {
		err = hexatype.ParseGRPCError(err)
	}

	return err
}

// CommitEntry makes a Commit request on a remote host
func (trans *NetTransport) CommitEntry(ctx context.Context, host string, entry *Entry, opts *RequestOptions) error {
	conn, err := trans.getConn(host)
	if err != nil {
		return err
	}

	req := &ReqResp{Entry: entry, Options: opts}
	if _, err = conn.client.CommitRPC(ctx, req); err != nil {
		err = hexatype.ParseGRPCError(err)
	}
	trans.returnConn(conn)

	return err
}

// LastEntry retrieves the last entry for the given key from a remote host.
func (trans *NetTransport) LastEntry(host string, key []byte, opts *RequestOptions) (*Entry, error) {
	conn, err := trans.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &ReqResp{Entry: &Entry{Key: key}, Options: opts}
	resp, err := conn.client.LastRPC(context.Background(), req)
	trans.returnConn(conn)

	if err != nil {
		return nil, hexatype.ParseGRPCError(err)
	}

	return resp.Entry, nil
}

// GetEntry makes a Get request to retrieve an Entry from a remote host.
func (trans *NetTransport) GetEntry(host string, key, id []byte, opts *RequestOptions) (*Entry, error) {
	conn, err := trans.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &ReqResp{ID: id, Entry: &Entry{Key: key}, Options: opts}
	resp, err := conn.client.GetRPC(context.Background(), req)
	trans.returnConn(conn)

	if err != nil {
		return nil, hexatype.ParseGRPCError(err)
	}

	return resp.Entry, nil
}

func (trans *NetTransport) getConn(host string) (*rpcOutConn, error) {
	if atomic.LoadInt32(&trans.shutdown) == 1 {
		return nil, fmt.Errorf("transport is shutdown")
	}

	trans.mu.RLock()
	if out, ok := trans.pool[host]; ok && out != nil {
		defer trans.mu.RUnlock()
		return out, nil
	}
	trans.mu.RUnlock()

	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	// Make a new connection
	trans.mu.Lock()
	out := &rpcOutConn{
		host:   host,
		client: NewHexalogRPCClient(conn),
		conn:   conn,
		used:   time.Now(),
	}
	trans.pool[host] = out
	trans.mu.Unlock()

	return out, err
}

func (trans *NetTransport) returnConn(o *rpcOutConn) {
	if atomic.LoadInt32(&trans.shutdown) == 1 {
		o.conn.Close()
		return
	}

	// Push back into the pool
	trans.mu.Lock()
	// Update the last used time
	o.used = time.Now()
	trans.pool[o.host] = o
	trans.mu.Unlock()
}

func (trans *NetTransport) reapOld() {
	for {
		if atomic.LoadInt32(&trans.shutdown) == 1 {
			return
		}
		time.Sleep(trans.reapInterval)
		trans.reapOnce()
	}
}

func (trans *NetTransport) reapOnce() {
	trans.mu.Lock()
	for host, conns := range trans.pool {
		if time.Since(conns.used) > trans.maxConnIdle {
			conns.conn.Close()
			delete(trans.pool, host)
		}
	}
	trans.mu.Unlock()
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

	if opt.WaitApply {

		fut := ballot.Future()
		_, err = fut.Wait(time.Duration(opt.WaitApplyTimeout) * time.Millisecond)
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
func (trans *NetTransport) GetRPC(ctx context.Context, req *ReqResp) (*ReqResp, error) {
	var (
		resp = &ReqResp{}
		err  error
	)

	resp.Entry, err = trans.hlog.store.GetEntry(req.Entry.Key, req.ID)

	return resp, err
}

// LastRPC serves a LastEntry request.  It returns the local last entry for a key.
func (trans *NetTransport) LastRPC(ctx context.Context, req *ReqResp) (*ReqResp, error) {
	var resp = &ReqResp{
		Entry: trans.hlog.store.LastEntry(req.Entry.Key),
	}
	return resp, nil
}

// NewRPC serves a NewEntry request from the local log
func (trans *NetTransport) NewRPC(ctx context.Context, req *ReqResp) (*ReqResp, error) {
	return &ReqResp{
		Entry: trans.hlog.New(req.Entry.Key),
	}, nil
}

// FetchKeylog fetches the key log from the given host starting at the entry.  If the
// previous hash of the entry is nil, then all entries for the key are fetched. It appends
// each entry directly to the log and submits to the FSM and returns a FutureEntry which
// is the last entry that was appended to the key log and/or an error.  The keylog must
// exist in order for the log to be sucessfully fetched.
func (trans *NetTransport) FetchKeylog(host string, entry *Entry, opts *RequestOptions) (*FutureEntry, error) {
	conn, err := trans.getConn(host)
	if err != nil {
		return nil, err
	}
	stream, err := conn.client.FetchKeylogRPC(context.Background())
	if err != nil {
		return nil, err
	}

	// Send the entry we want to start at.  Only generate an id for the entry if the
	// previous hash is not nil.  Remote assumes all log entries if the request id is nil.
	req := &ReqResp{Entry: entry}
	if entry.Previous != nil {
		req.ID = entry.Hash(trans.hlog.conf.Hasher.New())
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

		// if _, err = trans.hlog.store.GetEntry(msg.Entry.Key, msg.ID); err == nil {
		// 	continue
		// }

		if fentry, err = trans.hlog.append(msg.ID, msg.Entry); err != nil {
			log.Printf("[ERROR] Fetch key entry key=%s height=%d error='%v'", entry.Key, entry.Height, err)
		}

	}

	return fentry, hexatype.ParseGRPCError(err)
}

// FetchKeylogRPC streams log entries for key to the caller based on the request
func (trans *NetTransport) FetchKeylogRPC(stream HexalogRPC_FetchKeylogRPCServer) error {
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
	keylog, err := trans.hlog.store.GetKey(req.Entry.Key)
	if err != nil {
		return err
	}
	defer keylog.Close()

	// Get the seek position from the request id.  If it is nil assume all log entries need
	// to be sent
	var seek []byte
	if req.ID != nil {
		seek = req.ID
	}

	// Start sending entries starting from the id in the request
	err = keylog.Iter(seek, func(id []byte, entry *Entry) error {
		resp := &ReqResp{Entry: entry, ID: id}
		return stream.Send(resp)
	})

	return err
}

// TransferKeylog transfers a key directly from the store to the given host
func (trans *NetTransport) TransferKeylog(host string, key []byte, opts *RequestOptions) error {
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

	// Get the connection stream
	conn, err := trans.getConn(host)
	if err != nil {
		return err
	}
	stream, err := conn.client.TransferKeylogRPC(context.Background())
	if err != nil {
		return err
	}

	// Send request preamble with the local last entry letting the  remote know what key we
	// are requesting
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
		seek = preamble.Entry.Hash(trans.hlog.conf.Hasher.New())
	}
	// Iterate based on seek position
	err = keylog.Iter(seek, func(id []byte, entry *Entry) error {
		// Send entry
		req := &ReqResp{ID: id, Entry: entry}
		if err = stream.Send(req); err != nil {
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

// TransferKeylogRPC accepts a transfer request for a key initiated by a remote host.  The
// key must first be initialized before a transfer is accepted.  It returns an
// ErrKeyNotFound if the key has not been locally initialized or any underlying error
func (trans *NetTransport) TransferKeylogRPC(stream HexalogRPC_TransferKeylogRPCServer) error {
	// Get request and check it.  The ID in this request is the location id of the key.
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

		// We append to the keylog rather than the log here as we have already gotten the key
		// and would be more efficient
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

// Shutdown signals the transport to be shutdown and closes all outbound connections.
func (trans *NetTransport) Shutdown() {
	atomic.StoreInt32(&trans.shutdown, 1)

	trans.mu.Lock()
	for _, conn := range trans.pool {
		conn.conn.Close()
	}
	trans.pool = nil
	trans.mu.Unlock()
}
