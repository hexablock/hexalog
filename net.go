package hexalog

import (
	"fmt"
	"io"

	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

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
	pool map[string][]*rpcOutConn

	maxConnIdle  time.Duration
	reapInterval time.Duration

	shutdown int32
}

// NewNetTransport initializes a NetTransport with an outbound connection pool.
func NewNetTransport(reapInterval, maxConnIdle time.Duration) *NetTransport {
	trans := &NetTransport{
		pool:         make(map[string][]*rpcOutConn),
		maxConnIdle:  maxConnIdle,
		reapInterval: reapInterval,
	}

	go trans.reapOld()

	return trans
}

// ProposeEntry makes a Propose request on a remote host
func (trans *NetTransport) ProposeEntry(host string, entry *Entry, opts *RequestOptions) error {
	conn, err := trans.getConn(host)
	if err != nil {
		return err
	}

	req := &RPCRequest{Entry: entry, Options: opts}
	_, err = conn.client.ProposeRPC(context.Background(), req)
	trans.returnConn(conn)

	return err
}

// CommitEntry makes a Commit request on a remote host
func (trans *NetTransport) CommitEntry(host string, entry *Entry, opts *RequestOptions) error {
	conn, err := trans.getConn(host)
	if err != nil {
		return err
	}

	req := &RPCRequest{Entry: entry, Options: opts}
	_, err = conn.client.CommitRPC(context.Background(), req)
	trans.returnConn(conn)

	return err
}

// GetEntry makes a Get request to retrieve an Entry from a remote host.
func (trans *NetTransport) GetEntry(host string, key, id []byte, opts *RequestOptions) (*Entry, error) {
	conn, err := trans.getConn(host)
	if err != nil {
		return nil, err
	}

	req := &RPCRequest{ID: id, Entry: &Entry{Key: key}, Options: opts}
	resp, err := conn.client.GetRPC(context.Background(), req)
	trans.returnConn(conn)

	if err != nil {
		return nil, err
	}

	return resp.Entry, nil
}

func (trans *NetTransport) getConn(host string) (*rpcOutConn, error) {
	if atomic.LoadInt32(&trans.shutdown) == 1 {
		return nil, fmt.Errorf("transport is shutdown")
	}

	// Check if we have a conn cached
	var out *rpcOutConn
	trans.mu.Lock()
	list, ok := trans.pool[host]
	if ok && len(list) > 0 {
		out = list[len(list)-1]
		list = list[:len(list)-1]
		trans.pool[host] = list
	}
	trans.mu.Unlock()
	// Make a new connection
	if out == nil {
		conn, err := grpc.Dial(host, grpc.WithInsecure())
		if err == nil {
			return &rpcOutConn{
				host:   host,
				client: NewHexalogRPCClient(conn),
				conn:   conn,
				used:   time.Now(),
			}, nil
		}
		return nil, err
	}

	return out, nil
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
	list, _ := trans.pool[o.host]
	trans.pool[o.host] = append(list, o)
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
		max := len(conns)
		for i := 0; i < max; i++ {
			if time.Since(conns[i].used) > trans.maxConnIdle {
				conns[i].conn.Close()
				conns[i], conns[max-1] = conns[max-1], nil
				max--
				i--
			}
		}
		// Trim any idle conns
		trans.pool[host] = conns[:max]
	}

	trans.mu.Unlock()
}

// Register registers the log with the transport to serve RPC requests to the log
func (trans *NetTransport) Register(hlog *Hexalog) {
	trans.hlog = hlog
}

// ProposeRPC serves a Propose request.  The underlying ballot from the local log is ignored
func (trans *NetTransport) ProposeRPC(ctx context.Context, req *RPCRequest) (*RPCResponse, error) {
	resp := &RPCResponse{}
	_, err := trans.hlog.Propose(req.Entry, req.Options)
	return resp, err
}

// CommitRPC serves a Commit request.  The underlying ballot from the local log is ignored
func (trans *NetTransport) CommitRPC(ctx context.Context, req *RPCRequest) (*RPCResponse, error) {
	resp := &RPCResponse{}
	_, err := trans.hlog.Commit(req.Entry, req.Options)
	return resp, err
}

// GetRPC serves a Commit request.  The underlying ballot from the local log is ignored
func (trans *NetTransport) GetRPC(ctx context.Context, req *RPCRequest) (*RPCResponse, error) {
	var (
		resp = &RPCResponse{}
		ok   bool
		err  error
	)

	if resp.Entry, ok = trans.hlog.store.GetEntry(req.Entry.Key, req.ID); !ok {
		err = errEntryNotFound
	}

	return resp, err
}

// TransferKeylog transfers a key to the given host
func (trans *NetTransport) TransferKeylog(host string, key []byte) error {
	keylog, err := trans.hlog.store.GetKey(key)
	if err != nil {
		return err
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

	// Send request preamble with the last entry and the key location id
	preamble := &RPCRequest{
		Entry: keylog.LastEntry(),
		ID:    keylog.LocationID(),
	}
	if err = stream.Send(preamble); err != nil {
		return err
	}

	// Recieve preamble response with last entry
	if preamble, err = stream.Recv(); err != nil {
		return err
	}

	// Get the seek position based on last entry sent from remove
	h := trans.hlog.conf.Hasher.New()
	var seek []byte
	if preamble.Entry != nil {
		seek = preamble.Entry.Hash(h)
	}
	// Iterate based on seek position
	err = keylog.Iter(seek, func(entry *Entry) error {
		h.Reset()
		// Send entry
		req := &RPCRequest{ID: entry.Hash(h), Entry: entry}
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
		return errKeyInvalid
	}

	// Get the last entry for the key and assemble a new payload.
	preamble := &RPCRequest{
		Entry: trans.hlog.store.LastEntry(req.Entry.Key),
	}

	// Send last entry for the key back to requester
	if err = stream.Send(preamble); err != nil {
		return err
	}

	// Check for existence of the key locally
	if _, err = trans.hlog.store.GetKey(req.Entry.Key); err != nil {
		if err = trans.hlog.store.NewKey(req.Entry.Key, req.ID); err != nil {
			return err
		}
	}

	// Future to record the last entry applied
	var fentry *FutureEntry

	// Start receiving entries from the remote host
	for {
		var msg *RPCRequest
		if msg, err = stream.Recv(); err != nil {
			if err == io.EOF {
				//
				// TODO: may need to wait for entry to be applied
				//
				err = nil
			}
			break
		}

		log.Printf("[DEBUG] Take over id=%x key=%s height=%d prev=%x", msg.ID, msg.Entry.Key, msg.Entry.Height, msg.Entry.Previous)

		if err = trans.hlog.store.AppendEntry(msg.Entry); err != nil {
			break
		}

		fentry = NewFutureEntry(msg.ID, msg.Entry)
		trans.hlog.fsm.apply(fentry)

	}

	return err
}
