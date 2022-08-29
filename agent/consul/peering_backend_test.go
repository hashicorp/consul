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
	"github.com/hashicorp/consul/proto/pbpeerstream"
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

func TestPeerStreamService_ForwardToLeader(t *testing.T) {
	t.Parallel()

	_, conf1 := testServerConfig(t)
	server1, err := newServer(t, conf1)
	require.NoError(t, err)

	_, conf2 := testServerConfig(t)
	conf2.Bootstrap = false
	server2, err := newServer(t, conf2)
	require.NoError(t, err)

	// server1 is leader, server2 follower
	testrpc.WaitForLeader(t, server1.RPC, "dc1")
	joinLAN(t, server2, server1)
	testrpc.WaitForLeader(t, server2.RPC, "dc1")

	peerId := testUUID()

	// Simulate a GenerateToken call on server1, which stores the establishment secret
	{
		require.NoError(t, server1.FSM().State().PeeringWrite(10, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name: "foo",
				ID:   peerId,
			},
			SecretsRequest: &pbpeering.SecretsWriteRequest{
				PeerID: peerId,
				Request: &pbpeering.SecretsWriteRequest_GenerateToken{
					GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
						EstablishmentSecret: "389bbcdf-1c31-47d6-ae96-f2a3f4c45f84",
					},
				},
			},
		}))
	}

	testutil.RunStep(t, "server2 forwards write to server1", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)

		// We will dial server2 which should forward to server1
		conn, err := gogrpc.DialContext(ctx, server2.config.RPCAddr.String(),
			gogrpc.WithContextDialer(newServerDialer(server2.config.RPCAddr.String())),
			gogrpc.WithInsecure(),
			gogrpc.WithBlock())
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close() })

		peerStreamClient := pbpeerstream.NewPeerStreamServiceClient(conn)
		req := &pbpeerstream.ExchangeSecretRequest{
			PeerID:              peerId,
			EstablishmentSecret: "389bbcdf-1c31-47d6-ae96-f2a3f4c45f84",
		}
		_, err = peerStreamClient.ExchangeSecret(ctx, req)
		require.NoError(t, err)
	})
}
