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

func TestPeeringBackend_DoesNotForwardToDifferentDC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerDC(t, "dc1")
	_, s2 := testServerDC(t, "dc2")

	joinWAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// make a grpc client to dial s2 directly
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, s2.config.RPCAddr.String(),
		gogrpc.WithContextDialer(newServerDialer(s2.config.RPCAddr.String())),
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	// GenerateToken request should fail against dc1, because we are dialing dc2. The GenerateToken request should never be forwarded across datacenters.
	req := pbpeering.GenerateTokenRequest{
		PeerName:   "peer1-usw1",
		Datacenter: "dc1",
	}
	_, err = peeringClient.GenerateToken(ctx, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requests to generate peering tokens cannot be forwarded to remote datacenters")
}

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
			Datacenter: "dc1",
			PeerName:   "foo",
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
