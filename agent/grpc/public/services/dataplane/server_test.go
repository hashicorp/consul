package dataplane

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func testStateStore(t *testing.T) *state.Store {
	t.Helper()

	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	return state.NewStateStore(gc)
}

func testClient(t *testing.T, server *Server) pbdataplane.DataplaneServiceClient {
	t.Helper()

	addr := RunTestServer(t, server)

	conn, err := grpc.DialContext(context.Background(), addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return pbdataplane.NewDataplaneServiceClient(conn)
}

func RunTestServer(t *testing.T, server *Server) net.Addr {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	server.Register(grpcServer)

	go grpcServer.Serve(lis)
	t.Cleanup(grpcServer.Stop)

	return lis.Addr()
}
