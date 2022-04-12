package connectca

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
)

func testStateStore(t *testing.T, publisher state.EventPublisher) *state.Store {
	t.Helper()

	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	return state.NewStateStoreWithEventPublisher(gc, publisher)
}

type FakeFSM struct {
	lock      sync.Mutex
	store     *state.Store
	publisher *stream.EventPublisher
}

func newFakeFSM(t *testing.T, publisher *stream.EventPublisher) *FakeFSM {
	t.Helper()

	store := testStateStore(t, publisher)

	fsm := FakeFSM{store: store, publisher: publisher}

	// register handlers
	publisher.RegisterHandler(state.EventTopicCARoots, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return fsm.GetStore().CARootsSnapshot(req, buf)
	})

	return &fsm
}

func (f *FakeFSM) GetStore() *state.Store {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.store
}

func (f *FakeFSM) ReplaceStore(store *state.Store) {
	f.lock.Lock()
	defer f.lock.Unlock()
	oldStore := f.store
	f.store = store
	oldStore.Abandon()
	f.publisher.RefreshTopic(state.EventTopicCARoots)
}

func setupFSMAndPublisher(t *testing.T) (*FakeFSM, state.EventPublisher) {
	t.Helper()
	publisher := stream.NewEventPublisher(10 * time.Second)

	fsm := newFakeFSM(t, publisher)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go publisher.Run(ctx)

	return fsm, publisher
}

func testClient(t *testing.T, server *Server) pbconnectca.ConnectCAServiceClient {
	t.Helper()

	addr := runTestServer(t, server)

	conn, err := grpc.DialContext(context.Background(), addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return pbconnectca.NewConnectCAServiceClient(conn)
}

func runTestServer(t *testing.T, server *Server) net.Addr {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	server.Register(grpcServer)

	go grpcServer.Serve(lis)
	t.Cleanup(grpcServer.Stop)

	return lis.Addr()
}
