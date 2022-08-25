package consul

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestLeader_PeeringSync_Lifecycle_ClientDeletion(t *testing.T) {
	t.Run("without-tls", func(t *testing.T) {
		testLeader_PeeringSync_Lifecycle_ClientDeletion(t, false)
	})
	t.Run("with-tls", func(t *testing.T) {
		testLeader_PeeringSync_Lifecycle_ClientDeletion(t, true)
	})
}
func testLeader_PeeringSync_Lifecycle_ClientDeletion(t *testing.T, enableTLS bool) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, acceptor := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "acceptor"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		if enableTLS {
			c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
			c.TLSConfig.GRPC.CertFile = "../../test/hostname/Bob.crt"
			c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Bob.key"
		}
	})
	testrpc.WaitForLeader(t, acceptor.RPC, "dc1")

	// Create a peering by generating a token
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err := grpc.DialContext(ctx, acceptor.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(acceptor.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	acceptorClient := pbpeering.NewPeeringServiceClient(conn)

	req := pbpeering.GenerateTokenRequest{
		PeerName: "my-peer-dialer",
	}
	resp, err := acceptorClient.GenerateToken(ctx, &req)
	require.NoError(t, err)

	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)

	var token structs.PeeringToken
	require.NoError(t, json.Unmarshal(tokenJSON, &token))

	// S1 should not have a stream tracked for dc2 because acceptor generated a token for baz, and therefore needs to wait to be dialed.
	time.Sleep(1 * time.Second)
	_, found := acceptor.peerStreamServer.StreamStatus(token.PeerID)
	require.False(t, found)

	// Bring up dialer and establish a peering with acceptor's token so that it attempts to dial.
	_, dialer := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "dialer"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
		if enableTLS {
			c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
			c.TLSConfig.GRPC.CertFile = "../../test/hostname/Betty.crt"
			c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Betty.key"
		}
	})
	testrpc.WaitForLeader(t, dialer.RPC, "dc2")

	// Create a peering at dialer by establishing a peering with acceptor's token
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err = grpc.DialContext(ctx, dialer.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(dialer.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	dialerClient := pbpeering.NewPeeringServiceClient(conn)

	establishReq := pbpeering.EstablishRequest{
		PeerName:     "my-peer-acceptor",
		PeeringToken: resp.PeeringToken,
	}
	_, err = dialerClient.Establish(ctx, &establishReq)
	require.NoError(t, err)

	p, err := dialerClient.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: "my-peer-acceptor"})
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		status, found := dialer.peerStreamServer.StreamStatus(p.Peering.ID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	retry.Run(t, func(r *retry.R) {
		status, found := acceptor.peerStreamServer.StreamStatus(p.Peering.PeerID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	// Delete the peering to trigger the termination sequence.
	deleted := &pbpeering.Peering{
		ID:        p.Peering.ID,
		Name:      "my-peer-acceptor",
		DeletedAt: structs.TimeToProto(time.Now()),
	}
	require.NoError(t, dialer.fsm.State().PeeringWrite(2000, &pbpeering.PeeringWriteRequest{Peering: deleted}))
	dialer.logger.Trace("deleted peering for my-peer-acceptor")

	retry.Run(t, func(r *retry.R) {
		_, found := dialer.peerStreamServer.StreamStatus(p.Peering.ID)
		require.False(r, found)
	})

	// acceptor should have also marked the peering as terminated.
	retry.Run(t, func(r *retry.R) {
		_, peering, err := acceptor.fsm.State().PeeringRead(nil, state.Query{
			Value: "my-peer-dialer",
		})
		require.NoError(r, err)
		require.Equal(r, pbpeering.PeeringState_TERMINATED, peering.State)
	})
}

func TestLeader_PeeringSync_Lifecycle_ServerDeletion(t *testing.T) {
	t.Run("without-tls", func(t *testing.T) {
		testLeader_PeeringSync_Lifecycle_AcceptorDeletion(t, false)
	})
	t.Run("with-tls", func(t *testing.T) {
		testLeader_PeeringSync_Lifecycle_AcceptorDeletion(t, true)
	})
}

func testLeader_PeeringSync_Lifecycle_AcceptorDeletion(t *testing.T, enableTLS bool) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, acceptor := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "acceptor"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		if enableTLS {
			c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
			c.TLSConfig.GRPC.CertFile = "../../test/hostname/Bob.crt"
			c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Bob.key"
		}
	})
	testrpc.WaitForLeader(t, acceptor.RPC, "dc1")

	// Define a peering by generating a token for dialer
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err := grpc.DialContext(ctx, acceptor.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(acceptor.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	req := pbpeering.GenerateTokenRequest{
		PeerName: "my-peer-dialer",
	}
	resp, err := peeringClient.GenerateToken(ctx, &req)
	require.NoError(t, err)

	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)

	var token structs.PeeringToken
	require.NoError(t, json.Unmarshal(tokenJSON, &token))

	// Bring up dialer and establish a peering with acceptor's token so that it attempts to dial.
	_, dialer := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "dialer"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
		if enableTLS {
			c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
			c.TLSConfig.GRPC.CertFile = "../../test/hostname/Betty.crt"
			c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Betty.key"
		}
	})
	testrpc.WaitForLeader(t, dialer.RPC, "dc2")

	// Create a peering at dialer by establishing a peering with acceptor's token
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err = grpc.DialContext(ctx, dialer.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(dialer.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	dialerClient := pbpeering.NewPeeringServiceClient(conn)

	establishReq := pbpeering.EstablishRequest{
		PeerName:     "my-peer-acceptor",
		PeeringToken: resp.PeeringToken,
	}
	_, err = dialerClient.Establish(ctx, &establishReq)
	require.NoError(t, err)

	p, err := dialerClient.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: "my-peer-acceptor"})
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		status, found := dialer.peerStreamServer.StreamStatus(p.Peering.ID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	retry.Run(t, func(r *retry.R) {
		status, found := acceptor.peerStreamServer.StreamStatus(p.Peering.PeerID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	// Delete the peering from the server peer to trigger the termination sequence.
	deleted := &pbpeering.Peering{
		ID:        p.Peering.PeerID,
		Name:      "my-peer-dialer",
		DeletedAt: structs.TimeToProto(time.Now()),
	}

	require.NoError(t, acceptor.fsm.State().PeeringWrite(2000, &pbpeering.PeeringWriteRequest{Peering: deleted}))
	acceptor.logger.Trace("deleted peering for my-peer-dialer")

	retry.Run(t, func(r *retry.R) {
		_, found := acceptor.peerStreamServer.StreamStatus(p.Peering.PeerID)
		require.False(r, found)
	})

	// dialer should have received the termination message and updated the peering state.
	retry.Run(t, func(r *retry.R) {
		_, peering, err := dialer.fsm.State().PeeringRead(nil, state.Query{
			Value: "my-peer-acceptor",
		})
		require.NoError(r, err)
		require.Equal(r, pbpeering.PeeringState_TERMINATED, peering.State)
	})
}

func TestLeader_PeeringSync_FailsForTLSError(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("server-name-validation", func(t *testing.T) {
		testLeader_PeeringSync_failsForTLSError(t, func(token *structs.PeeringToken) {
			token.ServerName = "wrong.name"
		}, `transport: authentication handshake failed: x509: certificate is valid for server.dc1.consul, bob.server.dc1.consul, not wrong.name`)
	})
	t.Run("bad-ca-roots", func(t *testing.T) {
		wrongRoot, err := ioutil.ReadFile("../../test/client_certs/rootca.crt")
		require.NoError(t, err)

		testLeader_PeeringSync_failsForTLSError(t, func(token *structs.PeeringToken) {
			token.CA = []string{string(wrongRoot)}
		}, `transport: authentication handshake failed: x509: certificate signed by unknown authority`)
	})
}

func testLeader_PeeringSync_failsForTLSError(t *testing.T, tokenMutateFn func(token *structs.PeeringToken), expectErr string) {
	require.NotNil(t, tokenMutateFn)

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "bob"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"

		c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
		c.TLSConfig.GRPC.CertFile = "../../test/hostname/Bob.crt"
		c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Bob.key"
	})
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create a peering by generating a token
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err := grpc.DialContext(ctx, s1.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	req := pbpeering.GenerateTokenRequest{
		PeerName: "my-peer-s2",
	}
	resp, err := peeringClient.GenerateToken(ctx, &req)
	require.NoError(t, err)

	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)

	var token structs.PeeringToken
	require.NoError(t, json.Unmarshal(tokenJSON, &token))

	// Mutate token for test case
	tokenMutateFn(&token)

	// S1 should not have a stream tracked for dc2 because s1 generated a token
	// for baz, and therefore needs to wait to be dialed.
	time.Sleep(1 * time.Second)
	_, found := s1.peerStreamServer.StreamStatus(token.PeerID)
	require.False(t, found)

	// Bring up s2 and establish a peering with s1's token so that it attempts to dial.
	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "betty"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"

		c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
		c.TLSConfig.GRPC.CertFile = "../../test/hostname/Betty.crt"
		c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Betty.key"
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Create a peering at s2 by establishing a peering with s1's token
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err = grpc.DialContext(ctx, s2.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(s2.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	s2Client := pbpeering.NewPeeringServiceClient(conn)

	// Re-encode the mutated token and use it for the peering establishment.
	tokenJSON, err = json.Marshal(&token)
	require.NoError(t, err)
	tokenB64 := base64.StdEncoding.EncodeToString(tokenJSON)

	establishReq := pbpeering.EstablishRequest{
		PeerName:     "my-peer-s1",
		PeeringToken: tokenB64,
	}

	// Since the Establish RPC dials the remote cluster, it will yield the TLS error.
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	_, err = s2Client.Establish(ctx, &establishReq)
	require.Contains(t, err.Error(), expectErr)
}

func TestLeader_Peering_DeferredDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// TODO(peering): Configure with TLS
	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s1.dc1"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
	})
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	var (
		peerID      = "cc56f0b8-3885-4e78-8d7b-614a0c45712d"
		peerName    = "my-peer-s2"
		defaultMeta = acl.DefaultEnterpriseMeta()
		lastIdx     = uint64(0)
	)

	// Simulate a peering initiation event by writing a peering to the state store.
	lastIdx++
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   peerID,
			Name: peerName,
		},
	}))

	// Insert imported data: nodes, services, checks, trust bundle
	lastIdx = insertTestPeeringData(t, s1.fsm.State(), peerName, lastIdx)

	// Mark the peering for deletion to trigger the termination sequence.
	lastIdx++
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:        peerID,
			Name:      peerName,
			DeletedAt: structs.TimeToProto(time.Now()),
		},
	}))

	// Ensure imported data is gone:
	retry.Run(t, func(r *retry.R) {
		_, csn, err := s1.fsm.State().ServiceDump(nil, "", false, defaultMeta, peerName)
		require.NoError(r, err)
		require.Len(r, csn, 0)

		_, checks, err := s1.fsm.State().ChecksInState(nil, api.HealthAny, defaultMeta, peerName)
		require.NoError(r, err)
		require.Len(r, checks, 0)

		_, nodes, err := s1.fsm.State().NodeDump(nil, defaultMeta, peerName)
		require.NoError(r, err)
		require.Len(r, nodes, 0)

		_, tb, err := s1.fsm.State().PeeringTrustBundleRead(nil, state.Query{Value: peerName})
		require.NoError(r, err)
		require.Nil(r, tb)
	})

	// The leader routine should pick up the deletion and finish deleting the peering.
	retry.Run(t, func(r *retry.R) {
		_, peering, err := s1.fsm.State().PeeringRead(nil, state.Query{
			Value: peerName,
		})
		require.NoError(r, err)
		require.Nil(r, peering)
	})
}

// Test that the dialing peer attempts to reestablish connections when the accepting peer
// shuts down without sending a Terminated message.
//
// To test this, we start the two peer servers (accepting and dialing), set up peering, and then shut down
// the accepting peer. This terminates the connection without sending a Terminated message.
// We then restart the accepting peer and assert that the dialing peer reestablishes the connection.
func TestLeader_Peering_DialerReestablishesConnectionOnError(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Reserve a gRPC port so we can restart the accepting server with the same port.
	ports := freeport.GetN(t, 1)
	acceptingServerPort := ports[0]

	_, acceptingServer := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "acceptingServer.dc1"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		c.GRPCPort = acceptingServerPort
		c.PeeringEnabled = true
	})
	testrpc.WaitForLeader(t, acceptingServer.RPC, "dc1")

	// Create a peering by generating a token.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err := grpc.DialContext(ctx, acceptingServer.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(acceptingServer.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	acceptingClient := pbpeering.NewPeeringServiceClient(conn)
	req := pbpeering.GenerateTokenRequest{
		PeerName: "my-peer-dialing-server",
	}
	resp, err := acceptingClient.GenerateToken(ctx, &req)
	require.NoError(t, err)
	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)
	var token structs.PeeringToken
	require.NoError(t, json.Unmarshal(tokenJSON, &token))

	var (
		dialingServerPeerID = token.PeerID
	)

	// Bring up dialingServer and store acceptingServer's token so that it attempts to dial.
	_, dialingServer := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "dialing-server.dc2"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
		c.PeeringEnabled = true
	})
	testrpc.WaitForLeader(t, dialingServer.RPC, "dc2")

	// Create a peering at s2 by establishing a peering with s1's token
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err = grpc.DialContext(ctx, dialingServer.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(dialingServer.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	dialingClient := pbpeering.NewPeeringServiceClient(conn)

	establishReq := pbpeering.EstablishRequest{
		PeerName:     "my-peer-s1",
		PeeringToken: resp.PeeringToken,
	}
	_, err = dialingClient.Establish(ctx, &establishReq)
	require.NoError(t, err)

	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	p, err := dialingClient.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: "my-peer-s1"})
	require.NoError(t, err)

	// Wait for the stream to be connected.
	retry.Run(t, func(r *retry.R) {
		status, found := dialingServer.peerStreamServer.StreamStatus(p.Peering.ID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	// Wait until the dialing server has sent its roots over. This avoids a race condition where the accepting server
	// shuts down, but the dialing server is still sending messages to the stream. When this happens, an error is raised
	// which causes the stream to restart.
	// In this test, we want to test what happens when the stream is closed when there are _no_ messages being sent.
	retry.Run(t, func(r *retry.R) {
		_, bundle, err := acceptingServer.fsm.State().PeeringTrustBundleRead(nil, state.Query{Value: "my-peer-dialing-server"})
		require.NoError(r, err)
		require.NotNil(r, bundle)
	})

	// Capture the existing peering and associated secret so that they can be restored after the restart.
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	peering, err := acceptingClient.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: "my-peer-dialing-server"})
	require.NoError(t, err)
	require.NotNil(t, peering)

	secrets, err := acceptingServer.fsm.State().PeeringSecretsRead(nil, token.PeerID)
	require.NoError(t, err)
	require.NotNil(t, secrets)

	// Shutdown the accepting server.
	require.NoError(t, acceptingServer.Shutdown())

	// Have to manually shut down the gRPC server otherwise it stays bound to the port.
	acceptingServer.externalGRPCServer.Stop()

	// Restart the server by re-using the previous acceptor's data directory and node id.
	_, acceptingServerRestart := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "acceptingServer.dc1"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		c.GRPCPort = acceptingServerPort
		c.DataDir = acceptingServer.config.DataDir
		c.NodeID = acceptingServer.config.NodeID
	})

	testrpc.WaitForLeader(t, acceptingServerRestart.RPC, "dc1")

	// The dialing peer should eventually reconnect.
	retry.Run(t, func(r *retry.R) {
		connStreams := acceptingServerRestart.peerStreamServer.ConnectedStreams()
		require.Contains(r, connStreams, dialingServerPeerID)
	})
}

func insertTestPeeringData(t *testing.T, store *state.Store, peer string, lastIdx uint64) uint64 {
	lastIdx++
	require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
		TrustDomain: "952e6bd1-f4d6-47f7-83ff-84b31babaa17",
		PeerName:    peer,
		RootPEMs:    []string{"certificate bundle"},
	}))

	lastIdx++
	require.NoError(t, store.EnsureRegistration(lastIdx, &structs.RegisterRequest{
		Node:     "aaa",
		Address:  "10.0.0.1",
		PeerName: peer,
		Service: &structs.NodeService{
			Service:  "a-service",
			ID:       "a-service-1",
			Port:     8080,
			PeerName: peer,
		},
		Checks: structs.HealthChecks{
			{
				CheckID:     "a-service-1-check",
				ServiceName: "a-service",
				ServiceID:   "a-service-1",
				Node:        "aaa",
				PeerName:    peer,
			},
		},
	}))

	lastIdx++
	require.NoError(t, store.EnsureRegistration(lastIdx, &structs.RegisterRequest{
		Node:     "bbb",
		Address:  "10.0.0.2",
		PeerName: peer,
		Service: &structs.NodeService{
			Service:  "b-service",
			ID:       "b-service-1",
			Port:     8080,
			PeerName: peer,
		},
		Checks: structs.HealthChecks{
			{
				CheckID:     "b-service-1-check",
				ServiceName: "b-service",
				ServiceID:   "b-service-1",
				Node:        "bbb",
				PeerName:    peer,
			},
		},
	}))

	lastIdx++
	require.NoError(t, store.EnsureRegistration(lastIdx, &structs.RegisterRequest{
		Node:     "ccc",
		Address:  "10.0.0.3",
		PeerName: peer,
		Service: &structs.NodeService{
			Service:  "c-service",
			ID:       "c-service-1",
			Port:     8080,
			PeerName: peer,
		},
		Checks: structs.HealthChecks{
			{
				CheckID:     "c-service-1-check",
				ServiceName: "c-service",
				ServiceID:   "c-service-1",
				Node:        "ccc",
				PeerName:    peer,
			},
		},
	}))

	return lastIdx
}

// TODO(peering): once we move away from keeping state in stream tracker only on leaders, move this test to consul/server_test maybe
func TestLeader_Peering_ImportedExportedServicesCount(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s1.dc1"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		c.PeeringEnabled = true
	})
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	// Create a peering by generating a token
	conn, err := grpc.DialContext(ctx, s1.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	req := pbpeering.GenerateTokenRequest{
		PeerName: "my-peer-s2",
	}
	resp, err := peeringClient.GenerateToken(ctx, &req)
	require.NoError(t, err)

	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)

	var token structs.PeeringToken
	require.NoError(t, json.Unmarshal(tokenJSON, &token))

	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s2.dc2"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
		c.PeeringEnabled = true
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	conn, err = grpc.DialContext(ctx, s2.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(s2.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	s2Client := pbpeering.NewPeeringServiceClient(conn)

	establishReq := pbpeering.EstablishRequest{
		// Create a peering at s2 by establishing a peering with s1's token
		// Bring up s2 and store s1's token so that it attempts to dial.
		PeerName:     "my-peer-s1",
		PeeringToken: resp.PeeringToken,
	}
	_, err = s2Client.Establish(ctx, &establishReq)
	require.NoError(t, err)

	var lastIdx uint64

	// Add services to S1 to be synced to S2
	lastIdx++
	require.NoError(t, s1.FSM().State().EnsureRegistration(lastIdx, &structs.RegisterRequest{
		ID:      types.NodeID(generateUUID()),
		Node:    "aaa",
		Address: "10.0.0.1",
		Service: &structs.NodeService{
			Service: "a-service",
			ID:      "a-service-1",
			Port:    8080,
		},
		Checks: structs.HealthChecks{
			{
				CheckID:     "a-service-1-check",
				ServiceName: "a-service",
				ServiceID:   "a-service-1",
				Node:        "aaa",
			},
		},
	}))

	lastIdx++
	require.NoError(t, s1.FSM().State().EnsureRegistration(lastIdx, &structs.RegisterRequest{
		ID: types.NodeID(generateUUID()),

		Node:    "bbb",
		Address: "10.0.0.2",
		Service: &structs.NodeService{
			Service: "b-service",
			ID:      "b-service-1",
			Port:    8080,
		},
		Checks: structs.HealthChecks{
			{
				CheckID:     "b-service-1-check",
				ServiceName: "b-service",
				ServiceID:   "b-service-1",
				Node:        "bbb",
			},
		},
	}))

	lastIdx++
	require.NoError(t, s1.FSM().State().EnsureRegistration(lastIdx, &structs.RegisterRequest{
		ID: types.NodeID(generateUUID()),

		Node:    "ccc",
		Address: "10.0.0.3",
		Service: &structs.NodeService{
			Service: "c-service",
			ID:      "c-service-1",
			Port:    8080,
		},
		Checks: structs.HealthChecks{
			{
				CheckID:     "c-service-1-check",
				ServiceName: "c-service",
				ServiceID:   "c-service-1",
				Node:        "ccc",
			},
		},
	}))
	// Finished adding services

	type testCase struct {
		name                       string
		description                string
		exportedService            structs.ExportedServicesConfigEntry
		expectedImportedServsCount uint64
		expectedExportedServsCount uint64
	}

	testCases := []testCase{
		{
			name:        "wildcard",
			description: "for a wildcard exported services, we want to see all services synced",
			exportedService: structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: structs.WildcardSpecifier,
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "my-peer-s2",
							},
						},
					},
				},
			},
			expectedImportedServsCount: 4, // 3 services from above + the "consul" service
			expectedExportedServsCount: 4, // 3 services from above + the "consul" service
		},
		{
			name:        "no sync",
			description: "update the config entry to allow no service sync",
			exportedService: structs.ExportedServicesConfigEntry{
				Name: "default",
			},
			expectedImportedServsCount: 0, // we want to see this decremented from 4 --> 0
			expectedExportedServsCount: 0, // we want to see this decremented from 4 --> 0
		},
		{
			name:        "just a, b services",
			description: "export just two services",
			exportedService: structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "a-service",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "my-peer-s2",
							},
						},
					},
					{
						Name: "b-service",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "my-peer-s2",
							},
						},
					},
				},
			},
			expectedImportedServsCount: 2,
			expectedExportedServsCount: 2,
		},
		{
			name:        "unexport b service",
			description: "by unexporting b we want to see the count decrement eventually",
			exportedService: structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "a-service",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "my-peer-s2",
							},
						},
					},
				},
			},
			expectedImportedServsCount: 1,
			expectedExportedServsCount: 1,
		},
		{
			name:        "export c service",
			description: "now export the c service and expect the count to increment",
			exportedService: structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "a-service",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "my-peer-s2",
							},
						},
					},
					{
						Name: "c-service",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "my-peer-s2",
							},
						},
					},
				},
			},
			expectedImportedServsCount: 2,
			expectedExportedServsCount: 2,
		},
	}

	conn2, err := grpc.DialContext(ctx, s2.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(s2.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn2.Close()

	peeringClient2 := pbpeering.NewPeeringServiceClient(conn2)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lastIdx++
			require.NoError(t, s1.fsm.State().EnsureConfigEntry(lastIdx, &tc.exportedService))

			// Check that imported services count on S2 are what we expect
			retry.Run(t, func(r *retry.R) {
				// on Read
				resp, err := peeringClient2.PeeringRead(context.Background(), &pbpeering.PeeringReadRequest{Name: "my-peer-s1"})
				require.NoError(r, err)
				require.NotNil(r, resp.Peering)
				require.Equal(r, tc.expectedImportedServsCount, resp.Peering.ImportedServiceCount)

				// on List
				resp2, err2 := peeringClient2.PeeringList(context.Background(), &pbpeering.PeeringListRequest{})
				require.NoError(r, err2)
				require.NotEmpty(r, resp2.Peerings)
				require.Equal(r, tc.expectedExportedServsCount, resp2.Peerings[0].ImportedServiceCount)
			})

			// Check that exported services count on S1 are what we expect
			retry.Run(t, func(r *retry.R) {
				// on Read
				resp, err := peeringClient.PeeringRead(context.Background(), &pbpeering.PeeringReadRequest{Name: "my-peer-s2"})
				require.NoError(r, err)
				require.NotNil(r, resp.Peering)
				require.Equal(r, tc.expectedImportedServsCount, resp.Peering.ExportedServiceCount)

				// on List
				resp2, err2 := peeringClient.PeeringList(context.Background(), &pbpeering.PeeringListRequest{})
				require.NoError(r, err2)
				require.NotEmpty(r, resp2.Peerings)
				require.Equal(r, tc.expectedExportedServsCount, resp2.Peerings[0].ExportedServiceCount)
			})
		})
	}
}

// TODO(peering): once we move away from keeping state in stream tracker only on leaders, move this test to consul/server_test maybe
func TestLeader_PeeringMetrics_emitPeeringMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	var (
		s2PeerID1          = generateUUID()
		s2PeerID2          = generateUUID()
		testContextTimeout = 60 * time.Second
		lastIdx            = uint64(0)
	)

	// TODO(peering): Configure with TLS
	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s1.dc1"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
	})
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create a peering by generating a token
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	t.Cleanup(cancel)

	conn, err := grpc.DialContext(ctx, s1.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	req := pbpeering.GenerateTokenRequest{
		PeerName: "my-peer-s2",
	}
	resp, err := peeringClient.GenerateToken(ctx, &req)
	require.NoError(t, err)

	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)

	var token structs.PeeringToken
	require.NoError(t, json.Unmarshal(tokenJSON, &token))

	// Bring up s2 and store s1's token so that it attempts to dial.
	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s2.dc2"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Simulate exporting services in the tracker
	{
		// Simulate a peering initiation event by writing a peering with data from a peering token.
		// Eventually the leader in dc2 should dial and connect to the leader in dc1.
		p := &pbpeering.Peering{
			ID:                  s2PeerID1,
			Name:                "my-peer-s1",
			PeerID:              token.PeerID,
			PeerCAPems:          token.CA,
			PeerServerName:      token.ServerName,
			PeerServerAddresses: token.ServerAddresses,
		}
		require.True(t, p.ShouldDial())
		lastIdx++
		require.NoError(t, s2.fsm.State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: p}))

		p2 := &pbpeering.Peering{
			ID:                  s2PeerID2,
			Name:                "my-peer-s3",
			PeerID:              token.PeerID, // doesn't much matter what these values are
			PeerCAPems:          token.CA,
			PeerServerName:      token.ServerName,
			PeerServerAddresses: token.ServerAddresses,
		}
		require.True(t, p2.ShouldDial())
		lastIdx++
		require.NoError(t, s2.fsm.State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: p2}))

		// connect the stream
		mst1, err := s2.peeringServer.Tracker.Connected(s2PeerID1)
		require.NoError(t, err)

		// mimic tracking exported services
		mst1.TrackExportedService(structs.ServiceName{Name: "a-service"})
		mst1.TrackExportedService(structs.ServiceName{Name: "b-service"})
		mst1.TrackExportedService(structs.ServiceName{Name: "c-service"})

		// connect the stream
		mst2, err := s2.peeringServer.Tracker.Connected(s2PeerID2)
		require.NoError(t, err)

		// mimic tracking exported services
		mst2.TrackExportedService(structs.ServiceName{Name: "d-service"})
		mst2.TrackExportedService(structs.ServiceName{Name: "e-service"})
	}

	// set up a metrics sink
	sink := metrics.NewInmemSink(testContextTimeout, testContextTimeout)
	cfg := metrics.DefaultConfig("us-west")
	cfg.EnableHostname = false
	met, err := metrics.New(cfg, sink)
	require.NoError(t, err)

	errM := s2.emitPeeringMetricsOnce(s2.logger, met)
	require.NoError(t, errM)

	retry.Run(t, func(r *retry.R) {
		intervals := sink.Data()
		require.Len(r, intervals, 1)
		intv := intervals[0]

		// the keys for a Gauge value look like: {serviceName}.{prefix}.{key_name};{label=value};...
		keyMetric1 := fmt.Sprintf("us-west.consul.peering.exported_services;peer_name=my-peer-s1;peer_id=%s", s2PeerID1)
		metric1, ok := intv.Gauges[keyMetric1]
		require.True(r, ok, fmt.Sprintf("did not find the key %q", keyMetric1))

		require.Equal(r, float32(3), metric1.Value) // for a, b, c services

		keyMetric2 := fmt.Sprintf("us-west.consul.peering.exported_services;peer_name=my-peer-s3;peer_id=%s", s2PeerID2)
		metric2, ok := intv.Gauges[keyMetric2]
		require.True(r, ok, fmt.Sprintf("did not find the key %q", keyMetric2))

		require.Equal(r, float32(2), metric2.Value) // for d, e services
	})
}

// Test that the leader doesn't start its peering deletion routing when
// peering is disabled.
func TestLeader_Peering_NoDeletionWhenPeeringDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s1.dc1"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		c.PeeringEnabled = false
	})
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	var (
		peerID   = "cc56f0b8-3885-4e78-8d7b-614a0c45712d"
		peerName = "my-peer-s2"
		lastIdx  = uint64(0)
	)

	// Simulate a peering initiation event by writing a peering to the state store.
	lastIdx++
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   peerID,
			Name: peerName,
		},
	}))

	// Mark the peering for deletion to trigger the termination sequence.
	lastIdx++
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:        peerID,
			Name:      peerName,
			DeletedAt: structs.TimeToProto(time.Now()),
		},
	}))

	// The leader routine shouldn't be running so the peering should never get deleted.
	require.Never(t, func() bool {
		_, peering, err := s1.fsm.State().PeeringRead(nil, state.Query{
			Value: peerName,
		})
		if err != nil {
			t.Logf("unexpected err: %s", err)
			return true
		}
		if peering == nil {
			return true
		}
		return false
	}, 7*time.Second, 1*time.Second, "peering should not have been deleted")
}

// Test that the leader doesn't start its peering establishment routine
// when peering is disabled.
func TestLeader_Peering_NoEstablishmentWhenPeeringDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s1.dc1"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		c.PeeringEnabled = false
	})
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	var (
		peerID   = "cc56f0b8-3885-4e78-8d7b-614a0c45712d"
		peerName = "my-peer-s2"
		lastIdx  = uint64(0)
	)

	// Simulate a peering initiation event by writing a peering to the state store.
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:                  peerID,
			Name:                peerName,
			PeerServerAddresses: []string{"1.2.3.4"},
		},
	}))

	require.Never(t, func() bool {
		_, found := s1.peerStreamTracker.StreamStatus(peerID)
		return found
	}, 7*time.Second, 1*time.Second, "peering should not have been established")
}

// Test peeringRetryTimeout when the errors are FailedPrecondition errors because these
// errors have a different backoff.
func TestLeader_Peering_peeringRetryTimeout_failedPreconditionErrors(t *testing.T) {
	cases := []struct {
		failedAttempts uint
		expDuration    time.Duration
	}{
		// Constant time backoff.
		{0, 8 * time.Millisecond},
		{1, 8 * time.Millisecond},
		{2, 8 * time.Millisecond},
		{3, 8 * time.Millisecond},
		{4, 8 * time.Millisecond},
		{5, 8 * time.Millisecond},
		// Then exponential.
		{6, 16 * time.Millisecond},
		{7, 32 * time.Millisecond},
		{13, 2048 * time.Millisecond},
		{14, 4096 * time.Millisecond},
		{15, 8192 * time.Millisecond},
		// Max.
		{16, 8192 * time.Millisecond},
		{17, 8192 * time.Millisecond},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("failed attempts %d", c.failedAttempts), func(t *testing.T) {
			err := grpcstatus.Error(codes.FailedPrecondition, "msg")
			require.Equal(t, c.expDuration, peeringRetryTimeout(c.failedAttempts, err))
		})
	}
}

// Test peeringRetryTimeout with non-FailedPrecondition errors because these errors have a different
// backoff from FailedPrecondition errors.
func TestLeader_Peering_peeringRetryTimeout_regularErrors(t *testing.T) {
	cases := []struct {
		failedAttempts uint
		expDuration    time.Duration
	}{
		// Exponential.
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		// Until max.
		{8, 256 * time.Second},
		{9, 256 * time.Second},
		{10, 256 * time.Second},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("failed attempts %d", c.failedAttempts), func(t *testing.T) {
			err := errors.New("error")
			require.Equal(t, c.expDuration, peeringRetryTimeout(c.failedAttempts, err))
		})
	}
}

// This test exercises all the functionality of retryLoopBackoffPeering.
func TestLeader_Peering_retryLoopBackoffPeering(t *testing.T) {
	ctx := context.Background()
	logger := hclog.NewNullLogger()

	// loopCount counts how many times we executed loopFn.
	loopCount := 0
	// loopTimes holds the times at which each loopFn was executed. We use this to test the timeout functionality.
	var loopTimes []time.Time
	// loopFn will run 5 times and do something different on each loop.
	loopFn := func() error {
		loopCount++
		loopTimes = append(loopTimes, time.Now())
		if loopCount == 1 {
			return fmt.Errorf("error 1")
		}
		if loopCount == 2 {
			return fmt.Errorf("error 2")
		}
		if loopCount == 3 {
			// On the 3rd loop, return success which ends the loop.
			return nil
		}
		return nil
	}
	// allErrors collects all the errors passed into errFn.
	var allErrors []error
	errFn := func(e error) {
		allErrors = append(allErrors, e)
	}
	retryTimeFn := func(_ uint, _ error) time.Duration {
		return 1 * time.Millisecond
	}

	retryLoopBackoffPeering(ctx, logger, loopFn, errFn, retryTimeFn)

	// Ensure loopFn ran the number of expected times.
	require.Equal(t, 3, loopCount)
	// Ensure errFn ran as expected.
	require.Equal(t, []error{
		fmt.Errorf("error 1"),
		fmt.Errorf("error 2"),
	}, allErrors)

	// Test retryTimeFn by comparing the difference between when each loopFn ran.
	for i := range loopTimes {
		if i == 0 {
			// Can't compare first time.
			continue
		}
		require.True(t, loopTimes[i].Sub(loopTimes[i-1]) >= 1*time.Millisecond,
			"time between indices %d and %d was > 1ms", i, i-1)
	}
}

// Test that if the context is cancelled the loop exits.
func TestLeader_Peering_retryLoopBackoffPeering_cancelContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := hclog.NewNullLogger()

	// loopCount counts how many times we executed loopFn.
	loopCount := 0
	loopFn := func() error {
		loopCount++
		return fmt.Errorf("error %d", loopCount)
	}
	// allErrors collects all the errors passed into errFn.
	var allErrors []error
	errFn := func(e error) {
		allErrors = append(allErrors, e)
	}
	// Set the retry time to a huge number.
	retryTimeFn := func(_ uint, _ error) time.Duration {
		return 1 * time.Millisecond
	}

	// Cancel the context before the loop runs. It should run once and then exit.
	cancel()
	retryLoopBackoffPeering(ctx, logger, loopFn, errFn, retryTimeFn)

	// Ensure loopFn ran the number of expected times.
	require.Equal(t, 1, loopCount)
	// Ensure errFn ran as expected.
	require.Equal(t, []error{
		fmt.Errorf("error 1"),
	}, allErrors)
}

func Test_isFailedPreconditionErr(t *testing.T) {
	st := grpcstatus.New(codes.FailedPrecondition, "cannot establish a peering stream on a follower node")
	err := st.Err()
	assert.True(t, isFailedPreconditionErr(err))

	// test that wrapped errors are checked correctly
	werr := fmt.Errorf("wrapped: %w", err)
	assert.True(t, isFailedPreconditionErr(werr))
}

func Test_Leader_PeeringSync_ServerAddressUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// We want 1s retries for this test
	orig := maxRetryBackoff
	maxRetryBackoff = 1
	t.Cleanup(func() { maxRetryBackoff = orig })

	_, acceptor := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "acceptor"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
	})
	testrpc.WaitForLeader(t, acceptor.RPC, "dc1")

	// Create a peering by generating a token
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err := grpc.DialContext(ctx, acceptor.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(acceptor.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	acceptorClient := pbpeering.NewPeeringServiceClient(conn)

	req := pbpeering.GenerateTokenRequest{
		PeerName: "my-peer-dialer",
	}
	resp, err := acceptorClient.GenerateToken(ctx, &req)
	require.NoError(t, err)

	// Bring up dialer and establish a peering with acceptor's token so that it attempts to dial.
	_, dialer := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "dialer"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
	})
	testrpc.WaitForLeader(t, dialer.RPC, "dc2")

	// Create a peering at dialer by establishing a peering with acceptor's token
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, err = grpc.DialContext(ctx, dialer.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(dialer.config.RPCAddr.String())),
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	dialerClient := pbpeering.NewPeeringServiceClient(conn)

	establishReq := pbpeering.EstablishRequest{
		PeerName:     "my-peer-acceptor",
		PeeringToken: resp.PeeringToken,
	}
	_, err = dialerClient.Establish(ctx, &establishReq)
	require.NoError(t, err)

	p, err := dialerClient.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: "my-peer-acceptor"})
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		status, found := dialer.peerStreamServer.StreamStatus(p.Peering.ID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	testutil.RunStep(t, "calling establish with active connection does not overwrite server addresses", func(t *testing.T) {
		// generate a new token from the acceptor
		req := pbpeering.GenerateTokenRequest{
			PeerName: "my-peer-dialer",
		}
		resp, err := acceptorClient.GenerateToken(ctx, &req)
		require.NoError(t, err)

		token, err := acceptor.peeringBackend.DecodeToken([]byte(resp.PeeringToken))
		require.NoError(t, err)

		// we will update the token with bad addresses to assert it doesn't clobber existing ones
		token.ServerAddresses = []string{"1.2.3.4:1234"}

		badToken, err := acceptor.peeringBackend.EncodeToken(token)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)

		// try establishing
		establishReq := pbpeering.EstablishRequest{
			PeerName:     "my-peer-acceptor",
			PeeringToken: string(badToken),
		}
		_, err = dialerClient.Establish(ctx, &establishReq)
		require.NoError(t, err)

		p, err := dialerClient.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: "my-peer-acceptor"})
		require.NoError(t, err)
		require.NotContains(t, p.Peering.PeerServerAddresses, "1.2.3.4:1234")
	})

	testutil.RunStep(t, "updated server addresses are picked up by the leader", func(t *testing.T) {
		// force close the acceptor's gRPC server so the dialier retries with a new address.
		acceptor.externalGRPCServer.Stop()

		clone := proto.Clone(p.Peering)
		updated := clone.(*pbpeering.Peering)
		// start with a bad address so we can assert for a specific error
		updated.PeerServerAddresses = append([]string{
			"bad",
		}, p.Peering.PeerServerAddresses...)

		require.NoError(t, dialer.fsm.State().PeeringWrite(2000, &pbpeering.PeeringWriteRequest{Peering: updated}))

		retry.Run(t, func(r *retry.R) {
			status, found := dialer.peerStreamServer.StreamStatus(p.Peering.ID)
			require.True(r, found)
			// We assert for this error to be set which would indicate that we iterated
			// through a bad address.
			require.Contains(r, status.LastSendErrorMessage, "transport: Error while dialing dial tcp: address bad: missing port in address")
			require.False(r, status.Connected)
		})
	})
}
