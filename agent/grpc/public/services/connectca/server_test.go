package connectca

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
)

func testStateStore(t *testing.T) *state.Store {
	t.Helper()

	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	return state.NewStateStoreWithEventPublisher(gc)
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
