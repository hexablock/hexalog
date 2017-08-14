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

// ProposeEntry makes a Propose request on a remote host
func (trans *NetTransport) ProposeEntry(host string, entry *hexatype.Entry, opts *hexatype.RequestOptions) error {
	conn, err := trans.getConn(host)
	if err != nil {
		return err
	}

	req := &hexatype.ReqResp{Entry: entry, Options: opts}
	if _, err = conn.client.ProposeRPC(context.Background(), req); err != nil {
		err = hexatype.ParseGRPCError(err)
	}
	trans.returnConn(conn)

	return err
}

// CommitEntry makes a Commit request on a remote host
func (trans *NetTransport) CommitEntry(host string, entry *hexatype.Entry, opts *hexatype.RequestOptions) error {
	conn, err := trans.getConn(host)
	if err != nil {
		return err
	}

	req := &hexatype.ReqResp{Entry: entry, Options: opts}
	if _, err = conn.client.CommitRPC(context.Background(), req); err != nil {
		err = hexatype.ParseGRPCError(err)
	}
	trans.returnConn(conn)

	return err
}

func (trans *NetTransport) LastEntry(host string, key []byte, opts *hexatype.RequestOptions) (*hexatype.Entry, error) {
	conn, err := trans.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &hexatype.ReqResp{Entry: &hexatype.Entry{Key: key}, Options: opts}
	resp, err := conn.client.LastRPC(context.Background(), req)
	trans.returnConn(conn)

	if err != nil {
		return nil, hexatype.ParseGRPCError(err)
	}

	return resp.Entry, nil
}

// GetEntry makes a Get request to retrieve an Entry from a remote host.
func (trans *NetTransport) GetEntry(host string, key, id []byte, opts *hexatype.RequestOptions) (*hexatype.Entry, error) {
	conn, err := trans.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &hexatype.ReqResp{ID: id, Entry: &hexatype.Entry{Key: key}, Options: opts}
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

	// Update the last used time
	o.used = time.Now()

	// Push back into the pool
	trans.mu.Lock()
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
func (trans *NetTransport) ProposeRPC(ctx context.Context, req *hexatype.ReqResp) (*hexatype.ReqResp, error) {
	resp := &hexatype.ReqResp{}
	_, err := trans.hlog.Propose(req.Entry, req.Options)
	return resp, err
}

// CommitRPC serves a Commit request.  The underlying ballot from the local log is ignored
func (trans *NetTransport) CommitRPC(ctx context.Context, req *hexatype.ReqResp) (*hexatype.ReqResp, error) {
	resp := &hexatype.ReqResp{}
	_, err := trans.hlog.Commit(req.Entry, req.Options)
	return resp, err
}

// GetRPC serves a Commit request.  The underlying ballot from the local log is ignored
func (trans *NetTransport) GetRPC(ctx context.Context, req *hexatype.ReqResp) (*hexatype.ReqResp, error) {
	var (
		resp = &hexatype.ReqResp{}
		err  error
	)

	resp.Entry, err = trans.hlog.store.GetEntry(req.Entry.Key, req.ID)

	return resp, err
}

// LastRPC serves a LastEntry request.  It returns the local last entry for a key.
func (trans *NetTransport) LastRPC(ctx context.Context, req *hexatype.ReqResp) (*hexatype.ReqResp, error) {
	var resp = &hexatype.ReqResp{
		Entry: trans.hlog.store.LastEntry(req.Entry.Key),
	}
	return resp, nil
}

// FetchKeylog fetches the key log from the given host starting at the entry.  If the
// previous hash of the entry is nil, then all entries for the key are fetched.  It appends
// each entry directly to the log and submitting to the FSM and returns a FutureEntry
// which is the last entry that was appended to the key log and/or an error
func (trans *NetTransport) FetchKeylog(host string, entry *hexatype.Entry) (*FutureEntry, error) {
	conn, err := trans.getConn(host)
	if err != nil {
		return nil, err
	}
	stream, err := conn.client.FetchKeylogRPC(context.Background())
	if err != nil {
		return nil, err
	}

	// Send the entry we want to start at
	req := &hexatype.ReqResp{Entry: entry}

	// Only generate an id for the entry if the previous hash is not nil.  Remote assumes
	// all log entries if the request id is nil
	if entry.Previous != nil {
		req.ID = entry.Hash(trans.hlog.conf.Hasher.New())
	}

	log.Printf("[DEBUG] Fetching host=%s key=%s height=%d prev=%x", host, entry.Key, entry.Height, entry.Previous)
	if err = stream.Send(req); err != nil {
		return nil, err
	}

	// This is the last entry that has been applied to the FSM used to wait
	var fentry *FutureEntry

	// Start recieving entries
	for {
		var msg *hexatype.ReqResp
		if msg, err = stream.Recv(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		//
		// TODO: validate
		//

		// Continue on if already have the entry
		if _, err = trans.hlog.store.GetEntry(msg.Entry.Key, msg.ID); err == nil {
			continue
		}

		if fentry, err = trans.hlog.append(msg.ID, msg.Entry); err != nil {
			log.Printf("[ERROR] Fetch key entry key=%s height=%d error='%v'", entry.Key, entry.Height, err)
		}

	}

	return fentry, err
}

// FetchKeylogRPC streams log entries for key to the caller based on the request
func (trans *NetTransport) FetchKeylogRPC(stream HexalogRPC_FetchKeylogRPCServer) error {
	// Get request
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	// Make sure a key is specified in the request
	if req.Entry == nil || req.Entry.Key == nil || len(req.Entry.Key) == 0 {
		log.Printf("[ERROR] Invalid entry/key: %#v", req)
		return hexatype.ErrKeyInvalid
	}

	// Get the key log
	keylog, err := trans.hlog.store.GetKey(req.Entry.Key)
	if err != nil {
		return err
	}

	// Get the seek position from the request id.  If it is nil assume all log entries need
	// to be sent
	var seek []byte
	if req.ID != nil {
		seek = req.ID
	}

	// Start sending entries starting from the id in the request
	err = keylog.Iter(seek, func(id []byte, entry *hexatype.Entry) error {
		resp := &hexatype.ReqResp{Entry: entry, ID: id}
		return stream.Send(resp)
	})

	return err
}

// TransferKeylog transfers a key directly from the store to the given host
func (trans *NetTransport) TransferKeylog(host string, key []byte) error {
	// Get the keylog
	keylog, err := trans.hlog.store.GetKey(key)
	if err != nil {
		return err
	}

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

	// Send request preamble with the last entry and the key location id letting the remote
	// know what key we are requesting
	preamble := &hexatype.ReqResp{
		Entry: last,
		ID:    keylog.LocationID(),
	}
	if err = stream.Send(preamble); err != nil {
		return err
	}

	// Recieve preamble response with last entry of remote
	if preamble, err = stream.Recv(); err != nil {
		return err
	}

	// Get the seek position based on last entry sent from remote
	var seek []byte
	if preamble.Entry != nil {
		seek = preamble.Entry.Hash(trans.hlog.conf.Hasher.New())
	}
	// Iterate based on seek position
	err = keylog.Iter(seek, func(id []byte, entry *hexatype.Entry) error {
		// Send entry
		req := &hexatype.ReqResp{ID: id, Entry: entry}
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

// TransferKeylogRPC accepts a transfer request for a key initiated by a remote host
func (trans *NetTransport) TransferKeylogRPC(stream HexalogRPC_TransferKeylogRPCServer) error {
	// Get request and check it.  The ID in this request is the location id of the key.
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	if req.Entry == nil || req.Entry.Key == nil || len(req.Entry.Key) == 0 {
		log.Printf("[ERROR] Invalid entry/key: %#v", req)
		return hexatype.ErrKeyInvalid
	}

	// TODO:
	// mark receiving
	// defer unmark receiving

	// // Get the last entry for the key and assemble a new payload.
	// preamble := &hexatype.ReqResp{
	// 	Entry: trans.hlog.store.LastEntry(req.Entry.Key),
	// }
	//
	// // Send last entry for the key back to requester
	// if err = stream.Send(preamble); err != nil {
	// 	return err
	// }

	// Check for existence of the key locally
	var keylog *Keylog
	if keylog, err = trans.hlog.store.GetKey(req.Entry.Key); err != nil {
		if keylog, err = trans.hlog.store.NewKey(req.Entry.Key, req.ID); err != nil {
			return err
		}
	}

	// Send last entry for the key back to requester
	preamble := &hexatype.ReqResp{
		Entry: keylog.LastEntry(),
	}
	if err = stream.Send(preamble); err != nil {
		return err
	}

	// Future to record the last entry applied
	var fentry *FutureEntry

	// Start receiving entries from the remote host
	for {
		var msg *hexatype.ReqResp
		if msg, err = stream.Recv(); err != nil {
			if err == io.EOF {
				//
				// TODO: may need to wait for entry to be applied
				//
				err = nil
			}
			break
		}

		// Continue on if already have the entry
		if _, er := keylog.GetEntry(msg.ID); er != nil {
			continue
		}

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
		//for _, v := range arr {
		conn.conn.Close()
		//}
	}
	trans.pool = nil
	trans.mu.Unlock()
}
