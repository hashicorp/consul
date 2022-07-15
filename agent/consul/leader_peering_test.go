package consul

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/freeport"
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

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "bob"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		if enableTLS {
			c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
			c.TLSConfig.GRPC.CertFile = "../../test/hostname/Bob.crt"
			c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Bob.key"
		}
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

	// S1 should not have a stream tracked for dc2 because s1 generated a token for baz, and therefore needs to wait to be dialed.
	time.Sleep(1 * time.Second)
	_, found := s1.peerStreamServer.StreamStatus(token.PeerID)
	require.False(t, found)

	var (
		s2PeerID = "cc56f0b8-3885-4e78-8d7b-614a0c45712d"
	)

	// Bring up s2 and store s1's token so that it attempts to dial.
	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "betty"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
		if enableTLS {
			c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
			c.TLSConfig.GRPC.CertFile = "../../test/hostname/Betty.crt"
			c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Betty.key"
		}
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Simulate a peering initiation event by writing a peering with data from a peering token.
	// Eventually the leader in dc2 should dial and connect to the leader in dc1.
	p := &pbpeering.Peering{
		ID:                  s2PeerID,
		Name:                "my-peer-s1",
		PeerID:              token.PeerID,
		PeerCAPems:          token.CA,
		PeerServerName:      token.ServerName,
		PeerServerAddresses: token.ServerAddresses,
	}
	require.True(t, p.ShouldDial())

	// We maintain a pointer to the peering on the write so that we can get the ID without needing to re-query the state store.
	require.NoError(t, s2.fsm.State().PeeringWrite(1000, p))

	retry.Run(t, func(r *retry.R) {
		status, found := s2.peerStreamServer.StreamStatus(p.ID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	// Delete the peering to trigger the termination sequence.
	deleted := &pbpeering.Peering{
		ID:        s2PeerID,
		Name:      "my-peer-s1",
		DeletedAt: structs.TimeToProto(time.Now()),
	}
	require.NoError(t, s2.fsm.State().PeeringWrite(2000, deleted))
	s2.logger.Trace("deleted peering for my-peer-s1")

	retry.Run(t, func(r *retry.R) {
		_, found := s2.peerStreamServer.StreamStatus(p.ID)
		require.False(r, found)
	})

	// s1 should have also marked the peering as terminated.
	retry.Run(t, func(r *retry.R) {
		_, peering, err := s1.fsm.State().PeeringRead(nil, state.Query{
			Value: "my-peer-s2",
		})
		require.NoError(r, err)
		require.Equal(r, pbpeering.PeeringState_TERMINATED, peering.State)
	})
}

func TestLeader_PeeringSync_Lifecycle_ServerDeletion(t *testing.T) {
	t.Run("without-tls", func(t *testing.T) {
		testLeader_PeeringSync_Lifecycle_ServerDeletion(t, false)
	})
	t.Run("with-tls", func(t *testing.T) {
		testLeader_PeeringSync_Lifecycle_ServerDeletion(t, true)
	})
}
func testLeader_PeeringSync_Lifecycle_ServerDeletion(t *testing.T, enableTLS bool) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "bob"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		if enableTLS {
			c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
			c.TLSConfig.GRPC.CertFile = "../../test/hostname/Bob.crt"
			c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Bob.key"
		}
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

	var (
		s1PeerID = token.PeerID
		s2PeerID = "cc56f0b8-3885-4e78-8d7b-614a0c45712d"
	)

	// Bring up s2 and store s1's token so that it attempts to dial.
	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "betty"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
		if enableTLS {
			c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
			c.TLSConfig.GRPC.CertFile = "../../test/hostname/Betty.crt"
			c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Betty.key"
		}
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Simulate a peering initiation event by writing a peering with data from a peering token.
	// Eventually the leader in dc2 should dial and connect to the leader in dc1.
	p := &pbpeering.Peering{
		ID:                  s2PeerID,
		Name:                "my-peer-s1",
		PeerID:              token.PeerID,
		PeerCAPems:          token.CA,
		PeerServerName:      token.ServerName,
		PeerServerAddresses: token.ServerAddresses,
	}
	require.True(t, p.ShouldDial())

	// We maintain a pointer to the peering on the write so that we can get the ID without needing to re-query the state store.
	require.NoError(t, s2.fsm.State().PeeringWrite(1000, p))

	retry.Run(t, func(r *retry.R) {
		status, found := s2.peerStreamServer.StreamStatus(p.ID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	// Delete the peering from the server peer to trigger the termination sequence.
	deleted := &pbpeering.Peering{
		ID:        s1PeerID,
		Name:      "my-peer-s2",
		DeletedAt: structs.TimeToProto(time.Now()),
	}
	require.NoError(t, s1.fsm.State().PeeringWrite(2000, deleted))
	s2.logger.Trace("deleted peering for my-peer-s1")

	retry.Run(t, func(r *retry.R) {
		_, found := s1.peerStreamServer.StreamStatus(p.PeerID)
		require.False(r, found)
	})

	// s2 should have received the termination message and updated the peering state.
	retry.Run(t, func(r *retry.R) {
		_, peering, err := s2.fsm.State().PeeringRead(nil, state.Query{
			Value: "my-peer-s1",
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
		testLeader_PeeringSync_failsForTLSError(t, func(p *pbpeering.Peering) {
			p.PeerServerName = "wrong.name"
		}, `transport: authentication handshake failed: x509: certificate is valid for server.dc1.consul, bob.server.dc1.consul, not wrong.name`)
	})
	t.Run("bad-ca-roots", func(t *testing.T) {
		wrongRoot, err := ioutil.ReadFile("../../test/client_certs/rootca.crt")
		require.NoError(t, err)

		testLeader_PeeringSync_failsForTLSError(t, func(p *pbpeering.Peering) {
			p.PeerCAPems = []string{string(wrongRoot)}
		}, `transport: authentication handshake failed: x509: certificate signed by unknown authority`)
	})
}

func testLeader_PeeringSync_failsForTLSError(t *testing.T, peerMutateFn func(p *pbpeering.Peering), expectErr string) {
	require.NotNil(t, peerMutateFn)

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

	// S1 should not have a stream tracked for dc2 because s1 generated a token
	// for baz, and therefore needs to wait to be dialed.
	time.Sleep(1 * time.Second)
	_, found := s1.peerStreamServer.StreamStatus(token.PeerID)
	require.False(t, found)

	var (
		s2PeerID = "cc56f0b8-3885-4e78-8d7b-614a0c45712d"
	)

	// Bring up s2 and store s1's token so that it attempts to dial.
	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "betty"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"

		c.TLSConfig.GRPC.CAFile = "../../test/hostname/CertAuth.crt"
		c.TLSConfig.GRPC.CertFile = "../../test/hostname/Betty.crt"
		c.TLSConfig.GRPC.KeyFile = "../../test/hostname/Betty.key"
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Simulate a peering initiation event by writing a peering with data from a peering token.
	// Eventually the leader in dc2 should dial and connect to the leader in dc1.
	p := &pbpeering.Peering{
		ID:                  s2PeerID,
		Name:                "my-peer-s1",
		PeerID:              token.PeerID,
		PeerCAPems:          token.CA,
		PeerServerName:      token.ServerName,
		PeerServerAddresses: token.ServerAddresses,
	}
	peerMutateFn(p)
	require.True(t, p.ShouldDial())

	// We maintain a pointer to the peering on the write so that we can get the ID without needing to re-query the state store.
	require.NoError(t, s2.fsm.State().PeeringWrite(1000, p))

	retry.Run(t, func(r *retry.R) {
		status, found := s2.peerStreamTracker.StreamStatus(p.ID)
		require.True(r, found)
		require.False(r, status.Connected)
		require.Contains(r, status.LastSendErrorMessage, expectErr)
	})
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
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.Peering{
		ID:   peerID,
		Name: peerName,
	}))

	// Insert imported data: nodes, services, checks, trust bundle
	lastIdx = insertTestPeeringData(t, s1.fsm.State(), peerName, lastIdx)

	// Mark the peering for deletion to trigger the termination sequence.
	lastIdx++
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.Peering{
		ID:        peerID,
		Name:      peerName,
		DeletedAt: structs.TimeToProto(time.Now()),
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
// We then restart the accepting peer (we actually spin up a new server with the same config and port) and then
// assert that the dialing peer reestablishes the connection.
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

	peeringClient := pbpeering.NewPeeringServiceClient(conn)
	req := pbpeering.GenerateTokenRequest{
		PeerName: "my-peer-dialing-server",
	}
	resp, err := peeringClient.GenerateToken(ctx, &req)
	require.NoError(t, err)
	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)
	var token structs.PeeringToken
	require.NoError(t, json.Unmarshal(tokenJSON, &token))

	var (
		dialingServerPeerID   = token.PeerID
		acceptingServerPeerID = "cc56f0b8-3885-4e78-8d7b-614a0c45712d"
	)

	// Bring up dialingServer and store acceptingServer's token so that it attempts to dial.
	_, dialingServer := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "dialing-server.dc2"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
	})
	testrpc.WaitForLeader(t, dialingServer.RPC, "dc2")
	p := &pbpeering.Peering{
		ID:                  acceptingServerPeerID,
		Name:                "my-peer-accepting-server",
		PeerID:              token.PeerID,
		PeerCAPems:          token.CA,
		PeerServerName:      token.ServerName,
		PeerServerAddresses: token.ServerAddresses,
	}
	require.True(t, p.ShouldDial())
	require.NoError(t, dialingServer.fsm.State().PeeringWrite(1000, p))

	// Wait for the stream to be connected.
	retry.Run(t, func(r *retry.R) {
		status, found := dialingServer.peerStreamServer.StreamStatus(p.ID)
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

	// Shutdown the accepting server.
	require.NoError(t, acceptingServer.Shutdown())
	// Have to manually shut down the gRPC server otherwise it stays bound to the port.
	acceptingServer.externalGRPCServer.Stop()

	// Mimic the server restarting by starting a new server with the same config.
	_, acceptingServerRestart := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "acceptingServer.dc1"
		c.Datacenter = "dc1"
		c.TLSConfig.Domain = "consul"
		c.GRPCPort = acceptingServerPort
	})
	testrpc.WaitForLeader(t, acceptingServerRestart.RPC, "dc1")

	// Re-insert the peering state.
	require.NoError(t, acceptingServerRestart.fsm.State().PeeringWrite(2000, &pbpeering.Peering{
		ID:    dialingServerPeerID,
		Name:  "my-peer-dialing-server",
		State: pbpeering.PeeringState_PENDING,
	}))

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

// TODO(peering): once we move away from leader only request for PeeringList, move this test to consul/server_test maybe
func TestLeader_Peering_ImportedServicesCount(t *testing.T) {
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

	// Create a peering by generating a token
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	var (
		s2PeerID = "cc56f0b8-3885-4e78-8d7b-614a0c45712d"
		lastIdx  = uint64(0)
	)

	// Bring up s2 and store s1's token so that it attempts to dial.
	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s2.dc2"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Simulate a peering initiation event by writing a peering with data from a peering token.
	// Eventually the leader in dc2 should dial and connect to the leader in dc1.
	p := &pbpeering.Peering{
		ID:                  s2PeerID,
		Name:                "my-peer-s1",
		PeerID:              token.PeerID,
		PeerCAPems:          token.CA,
		PeerServerName:      token.ServerName,
		PeerServerAddresses: token.ServerAddresses,
	}
	require.True(t, p.ShouldDial())

	lastIdx++
	require.NoError(t, s2.fsm.State().PeeringWrite(lastIdx, p))

	/// add services to S1 to be synced to S2
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
	/// finished adding services

	type testCase struct {
		name                          string
		description                   string
		exportedService               structs.ExportedServicesConfigEntry
		expectedImportedServicesCount uint64
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
			expectedImportedServicesCount: 4, // 3 services from above + the "consul" service
		},
		{
			name:        "no sync",
			description: "update the config entry to allow no service sync",
			exportedService: structs.ExportedServicesConfigEntry{
				Name: "default",
			},
			expectedImportedServicesCount: 0, // we want to see this decremented from 4 --> 0
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
			expectedImportedServicesCount: 2,
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
			expectedImportedServicesCount: 1,
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
			expectedImportedServicesCount: 2,
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

			retry.Run(t, func(r *retry.R) {
				resp2, err := peeringClient2.PeeringList(ctx, &pbpeering.PeeringListRequest{})
				require.NoError(r, err)
				require.NotEmpty(r, resp2.Peerings)
				require.Equal(r, tc.expectedImportedServicesCount, resp2.Peerings[0].ImportedServiceCount)
			})
		})
	}
}
