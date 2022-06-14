package consul

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestLeader_PeeringSync_Lifecycle_ClientDeletion(t *testing.T) {
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
	_, found := s1.peeringService.StreamStatus(token.PeerID)
	require.False(t, found)

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
		status, found := s2.peeringService.StreamStatus(p.ID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	// Delete the peering to trigger the termination sequence.
	deleted := &pbpeering.Peering{
		Name:      "my-peer-s1",
		DeletedAt: structs.TimeToProto(time.Now()),
	}
	require.NoError(t, s2.fsm.State().PeeringWrite(2000, deleted))
	s2.logger.Trace("deleted peering for my-peer-s1")

	retry.Run(t, func(r *retry.R) {
		_, found := s2.peeringService.StreamStatus(p.ID)
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
		status, found := s2.peeringService.StreamStatus(p.ID)
		require.True(r, found)
		require.True(r, status.Connected)
	})

	// Delete the peering from the server peer to trigger the termination sequence.
	deleted := &pbpeering.Peering{
		Name:      "my-peer-s2",
		DeletedAt: structs.TimeToProto(time.Now()),
	}
	require.NoError(t, s1.fsm.State().PeeringWrite(2000, deleted))
	s2.logger.Trace("deleted peering for my-peer-s1")

	retry.Run(t, func(r *retry.R) {
		_, found := s1.peeringService.StreamStatus(p.PeerID)
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
		peerName    = "my-peer-s2"
		defaultMeta = acl.DefaultEnterpriseMeta()
		lastIdx     = uint64(0)
	)

	// Simulate a peering initiation event by writing a peering to the state store.
	lastIdx++
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.Peering{
		Name: peerName,
	}))

	// Insert imported data: nodes, services, checks, trust bundle
	lastIdx = insertTestPeeringData(t, s1.fsm.State(), peerName, lastIdx)

	// Mark the peering for deletion to trigger the termination sequence.
	lastIdx++
	require.NoError(t, s1.fsm.State().PeeringWrite(lastIdx, &pbpeering.Peering{
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
			{
				CheckID:  structs.SerfCheckID,
				Node:     "aaa",
				PeerName: peer,
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
			{
				CheckID:  structs.SerfCheckID,
				Node:     "bbb",
				PeerName: peer,
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
			{
				CheckID:  structs.SerfCheckID,
				Node:     "ccc",
				PeerName: peer,
			},
		},
	}))

	return lastIdx
}
