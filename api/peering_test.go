package api

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

const DefaultCtxDuration = 15 * time.Second

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

func TestAPI_Peering_Read_ErrorHandling(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithCA(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultCtxDuration)
	defer cancel()
	peerings := c.Peerings()

	t.Run("call Read with no name", func(t *testing.T) {
		resp, qm, err := peerings.Read(ctx, PeeringReadRequest{}, nil)

		// basic checks
		require.EqualError(t, err, "peering name cannot be empty")
		require.Empty(t, qm)
		require.Empty(t, resp)
	})

	t.Run("read peer that does not exist on server", func(t *testing.T) {
		resp, qm, err := peerings.Read(ctx, PeeringReadRequest{Name: "peer1"}, nil)

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
	ctx, cancel := context.WithTimeout(context.Background(), DefaultCtxDuration)
	defer cancel()
	peerings := c.Peerings()

	// "call List when no peers should exist"
	resp, qm, err := peerings.List(ctx, PeeringListRequest{}, nil)

	// basic checks
	require.NoError(t, err)
	require.NotEmpty(t, qm)

	require.Empty(t, resp) // no peerings so this should be empty

	// "call List when peers are present"
	resp2, wm, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer1"}, nil)
	require.NoError(t, err)
	require.NotEmpty(t, wm)
	require.NotEmpty(t, resp2)

	resp3, wm, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer2"}, nil)
	require.NoError(t, err)
	require.NotEmpty(t, wm)
	require.NotEmpty(t, resp3)

	peering1, qm, err2 := peerings.Read(ctx, PeeringReadRequest{Name: "peer1"}, nil)
	require.NoError(t, err2)
	require.NotEmpty(t, qm)
	require.NotEmpty(t, peering1)
	peering2, qm, err2 := peerings.Read(ctx, PeeringReadRequest{Name: "peer2"}, nil)
	require.NoError(t, err2)
	require.NotEmpty(t, qm)
	require.NotEmpty(t, peering2)

	peeringsList, qm, err := peerings.List(ctx, PeeringListRequest{}, nil)
	require.NoError(t, err)
	require.NotEmpty(t, qm)

	require.Equal(t, 2, len(peeringsList))
	require.True(t, peerExistsInPeerListings(peering1, peeringsList), "expected to find peering in list response")
	require.True(t, peerExistsInPeerListings(peering2, peeringsList), "expected to find peering in list response")

}

func TestAPI_Peering_GenerateToken(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithCA(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultCtxDuration)
	defer cancel()
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
	ctx, cancel := context.WithTimeout(context.Background(), DefaultCtxDuration)
	defer cancel()
	peerings := c.Peerings()

	p1 := PeeringGenerateTokenRequest{
		PeerName: "peer1",
		Meta:     map[string]string{"foo": "bar"},
	}
	var token1 string
	// Generate a token happy path
	resp, wm, err := peerings.GenerateToken(ctx, p1, options)
	token1 = resp.PeeringToken

	require.NoError(t, err)
	require.NotEmpty(t, wm)
	require.NotEmpty(t, resp)

	// Read token generated on server
	resp2, qm, err2 := peerings.Read(ctx, PeeringReadRequest{Name: "peer1"}, nil)

	// basic ok checking
	require.NoError(t, err2)
	require.NotEmpty(t, qm)
	require.NotEmpty(t, resp2)

	// token specific assertions on the "server"
	require.Equal(t, "peer1", resp2.Name)
	require.Equal(t, PeeringStateInitial, resp2.State)
	require.Equal(t, map[string]string{"foo": "bar"}, resp2.Meta)

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
		Meta:         map[string]string{"foo": "bar"},
	}

	respi, wm3, err3 := c2.Peerings().Initiate(ctx, i, options)

	// basic checks
	require.NoError(t, err3)
	require.NotEmpty(t, wm3)

	// at first the token will be undefined
	require.Equal(t, PeeringStateUndefined, PeeringState(respi.Status))

	// wait for the peering backend to finish the peering connection
	time.Sleep(2 * time.Second)

	retry.Run(t, func(r *retry.R) {
		respr, qm2, err4 := c2.Peerings().Read(ctx, PeeringReadRequest{Name: "peer1"}, nil)

		// basic ok checking
		require.NoError(r, err4)
		require.NotEmpty(r, qm2)

		// require that the peering state is not undefined
		require.Equal(r, PeeringStateInitial, respr.State)
		require.Equal(r, map[string]string{"foo": "bar"}, respr.Meta)

		// TODO(peering) -- let's go all the way and test in code either here or somewhere else that PeeringState does move to Active
	})

	// Delete the token on server 1
	resp4, qm3, err5 := peerings.Delete(ctx, PeeringDeleteRequest{Name: "peer1"}, nil)

	require.NoError(t, err5)
	require.NotEmpty(t, qm3)

	// {} is returned on success for now
	require.Empty(t, resp4)

	// Read to see if the token is "gone"
	resp5, qm4, err6 := peerings.Read(ctx, PeeringReadRequest{Name: "peer1"}, nil)

	// basic checks
	require.NotNil(t, err6)
	require.Empty(t, qm4)
	require.Empty(t, resp5)
}
