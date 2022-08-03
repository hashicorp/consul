package consul

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gogrpc "google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestPeeringBackend_ForwardToLeader(t *testing.T) {
	t.Parallel()

	_, conf1 := testServerConfig(t)
	server1, err := newServer(t, conf1)
	require.NoError(t, err)

	_, conf2 := testServerConfig(t)
	conf2.Bootstrap = false
	server2, err := newServer(t, conf2)
	require.NoError(t, err)

	// Join a 2nd server (not the leader)
	testrpc.WaitForLeader(t, server1.RPC, "dc1")
	joinLAN(t, server2, server1)
	testrpc.WaitForLeader(t, server2.RPC, "dc1")

	// Make a write call to server2 and make sure it gets forwarded to server1
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	// Dial server2 directly
	conn, err := gogrpc.DialContext(ctx, server2.config.RPCAddr.String(),
		gogrpc.WithContextDialer(newServerDialer(server2.config.RPCAddr.String())),
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	testutil.RunStep(t, "forward a write", func(t *testing.T) {
		// Do the grpc Write call to server2
		req := pbpeering.GenerateTokenRequest{
			PeerName: "foo",
		}
		_, err := peeringClient.GenerateToken(ctx, &req)
		require.NoError(t, err)

		// TODO(peering) check that state store is updated on leader, indicating a forwarded request after state store
		// is implemented.
	})
}

func newServerDialer(serverAddr string) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", serverAddr)
		if err != nil {
			return nil, err
		}

		_, err = conn.Write([]byte{byte(pool.RPCGRPC)})
		if err != nil {
			conn.Close()
			return nil, err
		}

		return conn, nil
	}
}
