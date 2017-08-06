package hexalog

import (
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"

	chord "github.com/hexablock/go-chord"
	"github.com/hexablock/hexaring"
	"github.com/hexablock/hexatype"
	"github.com/hexablock/log"
)

var (
	testOpts = &RequestOptions{
		PeerSet: []*hexaring.Location{
			&hexaring.Location{ID: []byte("1"),
				Vnode: &chord.Vnode{Id: []byte("1"), Host: "127.0.0.1:43211"}},
			&hexaring.Location{ID: []byte("2"),
				Vnode: &chord.Vnode{Id: []byte("2"), Host: "127.0.0.1:43212"}},
			&hexaring.Location{ID: []byte("3"),
				Vnode: &chord.Vnode{Id: []byte("3"), Host: "127.0.0.1:43213"}},
		},
	}
	testOpts1 = &RequestOptions{
		PeerSet: []*hexaring.Location{
			&hexaring.Location{ID: []byte("1"),
				Vnode: &chord.Vnode{Id: []byte("1"), Host: "127.0.0.1:53211"}},
			&hexaring.Location{ID: []byte("2"),
				Vnode: &chord.Vnode{Id: []byte("2"), Host: "127.0.0.1:53212"}},
			&hexaring.Location{ID: []byte("3"),
				Vnode: &chord.Vnode{Id: []byte("3"), Host: "127.0.0.1:53213"}},
		},
	}
)

type testServer struct {
	s    *grpc.Server
	ln   net.Listener
	hlog *Hexalog
}

func (server *testServer) stop() {
	server.hlog.trans.Shutdown()
	server.s.Stop()
	server.ln.Close()
}

type MockStableStore struct{}

func (ss *MockStableStore) Open() error                    { return nil }
func (ss *MockStableStore) Get(key []byte) ([]byte, error) { return nil, nil }
func (ss *MockStableStore) Set(key, value []byte) error    { return nil }
func (ss *MockStableStore) Close() error                   { return nil }

func initTestServer(addr string) *testServer {
	ln, _ := net.Listen("tcp", addr)
	server := grpc.NewServer()

	// Set to low value to allow reaper testing
	trans := NewNetTransport(500*time.Millisecond, 3*time.Second)
	RegisterHexalogRPCServer(server, trans)

	ss := &MockStableStore{}
	ls := NewInMemLogStore(&hexatype.SHA1Hasher{})
	hlog, _ := initHexalog(addr, ls, ss, trans)

	go server.Serve(ln)

	return &testServer{ln: ln, hlog: hlog, s: server}
}

func initHexalog(host string, ls LogStore, ss StableStore, trans Transport) (*Hexalog, error) {
	conf := DefaultConfig(host)
	conf.BallotReapInterval = 5 * time.Second

	return NewHexalog(conf, &EchoFSM{}, ls, ss, trans)
}

func TestMain(m *testing.M) {
	log.SetLevel("INFO")
	os.Exit(m.Run())
}

func TestHexalogShutdown(t *testing.T) {
	ts1 := initTestServer("127.0.0.1:8997")
	ts2 := initTestServer("127.0.0.1:9997")
	ts3 := initTestServer("127.0.0.1:10997")
	<-time.After(1 * time.Second)

	ts1.hlog.Shutdown()
	ts2.hlog.Shutdown()
	ts3.hlog.Shutdown()

	ts1.stop()
	ts2.stop()
	ts3.stop()

}
