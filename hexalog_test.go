package hexalog

import (
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"

	chord "github.com/hexablock/go-chord"
	"github.com/hexablock/hexaring"
	"github.com/hexablock/log"
)

var (
	testOpts = &RequestOptions{
		PeerSet: []*hexaring.Location{
			{ID: []byte("1"),
				Vnode: &chord.Vnode{Id: []byte("1"), Host: "127.0.0.1:43211"}},
			{ID: []byte("2"),
				Vnode: &chord.Vnode{Id: []byte("2"), Host: "127.0.0.1:43212"}},
			{ID: []byte("3"),
				Vnode: &chord.Vnode{Id: []byte("3"), Host: "127.0.0.1:43213"}},
		},
	}
	testOpts1 = &RequestOptions{
		PeerSet: []*hexaring.Location{
			{ID: []byte("1"),
				Vnode: &chord.Vnode{Id: []byte("1"), Host: "127.0.0.1:53211"}},
			{ID: []byte("2"),
				Vnode: &chord.Vnode{Id: []byte("2"), Host: "127.0.0.1:53212"}},
			{ID: []byte("3"),
				Vnode: &chord.Vnode{Id: []byte("3"), Host: "127.0.0.1:53213"}},
		},
	}
	testOpts2 = &RequestOptions{
		PeerSet: []*hexaring.Location{
			{
				ID:    []byte("1"),
				Vnode: &chord.Vnode{Id: []byte("1"), Host: "127.0.0.1:8997"},
			},
			{
				ID:    []byte("2"),
				Vnode: &chord.Vnode{Id: []byte("2"), Host: "127.0.0.1:9997"},
			},
			{
				ID:    []byte("3"),
				Vnode: &chord.Vnode{Id: []byte("3"), Host: "127.0.0.1:10997"},
			},
		},
	}
)

type testServer struct {
	conf *Config

	s  *grpc.Server
	ln net.Listener

	ss StableStore
	es EntryStore
	is IndexStore
	ls *LogStore

	fsm *EchoFSM

	hlog *Hexalog

	datadir string
}

func (server *testServer) start() {
	go server.s.Serve(server.ln)
}

func (server *testServer) cleanup() {
	os.RemoveAll(server.datadir)
}

func (server *testServer) stop() {
	server.hlog.trans.Shutdown()
	server.s.Stop()
	server.ln.Close()

	server.es.Close()
	server.cleanup()
}

func initConf(addr string) *Config {
	conf := DefaultConfig(addr)
	conf.BallotReapInterval = 5 * time.Second
	return conf
}

func (server *testServer) initStorage() {
	server.ss = &InMemStableStore{}
	server.es = NewInMemEntryStore()
	server.is = NewInMemIndexStore()
	server.ls = NewLogStore(server.es, server.is, server.conf.Hasher)
	server.fsm = &EchoFSM{}
}

func initTestServer(addr string) *testServer {
	ts := &testServer{}
	ts.ln, _ = net.Listen("tcp", addr)
	ts.s = grpc.NewServer()

	ts.conf = initConf(addr)
	ts.datadir, _ = ioutil.TempDir("/tmp", "hexalog-")

	// Set to low value to allow reaper testing
	trans := NewNetTransport(500*time.Millisecond, 3*time.Second)
	RegisterHexalogRPCServer(ts.s, trans)

	ts.initStorage()

	ts.hlog, _ = NewHexalog(ts.conf, ts.fsm, ts.ls, ts.ss, trans)

	ts.start()

	return ts
}

func TestMain(m *testing.M) {
	log.SetLevel("DEBUG")
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	os.Exit(m.Run())
}

func TestHexalog(t *testing.T) {
	ts1 := initTestServer("127.0.0.1:8997")
	ts2 := initTestServer("127.0.0.1:9997")
	ts3 := initTestServer("127.0.0.1:10997")
	<-time.After(1 * time.Second)

	testkey := []byte("hexalog-key")
	testdata := []byte("hexalog-data")
	ent1 := ts1.hlog.New(testkey)
	ent1.Data = testdata
	i1 := ent1.Hash(ts1.conf.Hasher.New())

	ballot, err := ts2.hlog.Propose(ent1, testOpts2)
	if err != nil {
		t.Fatal(err)
	}

	if err = ballot.Wait(); err != nil {
		t.Fatal(err)
	}
	fut := ballot.Future()
	if _, err = fut.Wait(400 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// Wait for all other nodes
	<-time.After(400 * time.Millisecond)

	if _, err = ts1.es.Get(i1); err != nil {
		t.Fatal(err)
	}
	if _, err = ts2.es.Get(i1); err != nil {
		t.Fatal(err)
	}
	if _, err = ts3.es.Get(i1); err != nil {
		t.Fatal(err)
	}

	leader, err := ts2.hlog.Leader(testkey, testOpts2.PeerSet)
	if err != nil {
		t.Fatal(err)
	}

	if ok, _ := leader.IsConsistent(); !ok {
		t.Fatal("should be consistent")
	}

	t.Logf("%+v", ts2.hlog.Stats())

	ts1.hlog.Shutdown()
	ts2.hlog.Shutdown()
	ts3.hlog.Shutdown()

	ts1.stop()
	ts2.stop()
	ts3.stop()

}
