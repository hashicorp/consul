package api

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/serf/testutil/retry"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func peerExistsInPeerListings(peer *Peering, peerings []*Peering) bool {
	for _, aPeer := range peerings {
		isEqual := (peer.PeerID == aPeer.PeerID) &&
			(reflect.DeepEqual(peer.PeerCAPems, aPeer.PeerCAPems)) &&
			(peer.PeerServerName == aPeer.PeerServerName) &&
			(peer.Partition == aPeer.Partition) &&
			(peer.Name == aPeer.Name) &&
			(reflect.DeepEqual(peer.PeerServerAddresses, aPeer.PeerServerAddresses)) &&
			(peer.State == aPeer.State) &&
			(peer.CreateIndex == aPeer.CreateIndex) &&
			(peer.ModifyIndex) == aPeer.ModifyIndex

		if isEqual {
			return true
		}
	}

	return false
}

func TestAPI_Peering_Read(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithCA(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)
	ctx := context.Background()
	peerings := c.Peerings()

	t.Run("call Read with no name", func(t *testing.T) {
		resp, qm, err := peerings.Read(ctx, PeeringRequest{}, nil)

		// basic checks
		require.EqualError(t, err, "peering name cannot be empty")
		require.Empty(t, qm)
		require.Empty(t, resp)
	})

	t.Run("read token that does not exist on server", func(t *testing.T) {
		resp, qm, err := peerings.Read(ctx, PeeringRequest{Name: "peer1"}, nil)

		// basic checks
		require.NotNil(t, err) // 404
		require.Empty(t, qm)
		require.Empty(t, resp)
	})

}

// TestAPI_Peering_List
func TestAPI_Peering_List(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithCA(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)
	ctx := context.Background()
	peerings := c.Peerings()

	t.Run("call List when no peers should exist", func(t *testing.T) {
		resp, qm, err := peerings.List(ctx, PeeringListRequest{}, nil)

		// basic checks
		require.NoError(t, err)
		require.NotEmpty(t, qm)

		require.Empty(t, resp) // no peerings so this should be empty
	})

	t.Run("call List when peers are present", func(t *testing.T) {
		// Generate a token happy path
		resp, wm, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer1"}, nil)

		require.NoError(t, err)
		require.NotEmpty(t, wm)
		require.NotEmpty(t, resp)

		peering, qm, err2 := peerings.Read(ctx, PeeringRequest{Name: "peer1"}, nil)

		// basic ok checking
		require.NoError(t, err2)
		require.NotEmpty(t, qm)
		require.NotEmpty(t, peering)

		peeringsList, qm, err := peerings.List(ctx, PeeringListRequest{}, nil)

		// basic checks
		require.NoError(t, err)
		require.NotEmpty(t, qm)

		require.True(t, peerExistsInPeerListings(peering, peeringsList), "expected to find peering in list response")

		// modify peering to some non existent peering
		peering.Name = "not_peer1"

		require.False(t, peerExistsInPeerListings(peering, peeringsList), "did not expect to find peering in list response")
	})

	t.Run("call List with dc1", func(t *testing.T) {
		resp, wm, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer2", Datacenter: "dc1"}, nil)

		require.NoError(t, err)
		require.NotEmpty(t, wm)
		require.NotEmpty(t, resp)

		peering, qm, err2 := peerings.Read(ctx, PeeringRequest{Name: "peer2"}, nil)

		// basic ok checking
		require.NoError(t, err2)
		require.NotEmpty(t, qm)
		require.NotEmpty(t, peering)

		peeringsList, qm, err := peerings.List(ctx, PeeringListRequest{}, nil)

		// basic checks
		require.NoError(t, err)
		require.NotEmpty(t, qm)

		require.True(t, peerExistsInPeerListings(peering, peeringsList), "expected to find peering in list response")
		require.Equal(t, 2, len(peeringsList))
	})
}

func TestAPI_Peering_GenerateToken(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithCA(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)
	ctx := context.Background()
	peerings := c.Peerings()

	t.Run("cannot have GenerateToken forward DC requests", func(t *testing.T) {
		// Try to generate a token in dc2
		resp, wm, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer2", Datacenter: "dc2"}, nil)

		require.Error(t, err)
		require.Empty(t, wm)
		require.Empty(t, resp)
	})
}

// TODO(peering): cover the following test cases: bad/ malformed input, peering with wrong token,
// peering with the wrong PeerName

// TestAPI_Peering_GenerateToken_Read_Initiate_Delete tests the following use case:
// a server creates a peering token, reads the token, then another server calls initiate peering
// finally, we delete the token on the first server
func TestAPI_Peering_GenerateToken_Read_Initiate_Delete(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithCA(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)
	options := &WriteOptions{Datacenter: "dc1"}
	ctx := context.Background()
	peerings := c.Peerings()

	p1 := PeeringGenerateTokenRequest{
		PeerName: "peer1",
	}
	var token1 string
	// Generate a token happy path
	resp, wm, err := peerings.GenerateToken(ctx, p1, options)
	token1 = resp.PeeringToken

	require.NoError(t, err)
	require.NotEmpty(t, wm)
	require.NotEmpty(t, resp)

	// Read token generated on server
	resp2, qm, err2 := peerings.Read(ctx, PeeringRequest{Name: "peer1"}, nil)

	// basic ok checking
	require.NoError(t, err2)
	require.NotEmpty(t, qm)
	require.NotEmpty(t, resp2)

	// token specific assertions on the "server"
	require.Equal(t, "peer1", resp2.Name)
	require.Equal(t, INITIAL, resp2.State)

	// Initiate peering

	// make a "client" server in second DC for peering
	c2, s2 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Datacenter = "dc2"
	})
	defer s2.Stop()

	i := PeeringInitiateRequest{
		Datacenter:   c2.config.Datacenter,
		PeerName:     "peer1",
		PeeringToken: token1,
	}

	respi, wm3, err3 := c2.Peerings().Initiate(ctx, i, options)

	// basic checks
	require.NoError(t, err3)
	require.NotEmpty(t, wm3)

	// at first the token will be undefined
	require.Equal(t, UNDEFINED, PeeringState(respi.Status))

	// wait for the peering backend to finish the peering connection
	time.Sleep(2 * time.Second)

	retry.Run(t, func(r *retry.R) {
		respr, qm2, err4 := c2.Peerings().Read(ctx, PeeringRequest{Name: "peer1"}, nil)

		// basic ok checking
		require.NoError(t, err4)
		require.NotEmpty(t, qm2)

		// require that the peering state is not undefined
		require.Equal(t, INITIAL, respr.State)

		// TODO(peering) -- let's go all the way and test in code either here or somewhere else that PeeringState does move to Active
		// require.Equal(t, PeeringState_ACTIVE, respr.State)
	})

	// Delete the token on server 1
	p := PeeringRequest{
		Name: "peer1",
	}
	resp4, qm3, err5 := peerings.Delete(ctx, p, nil)

	require.NoError(t, err5)
	require.NotEmpty(t, qm3)

	// {} is returned on success for now
	require.Empty(t, resp4)

	// Read to see if the token is "gone"
	resp5, qm4, err6 := peerings.Read(ctx, PeeringRequest{Name: "peer1"}, nil)

	// basic checks
	require.NotNil(t, err6)
	require.Empty(t, qm4)
	require.Empty(t, resp5)
}
