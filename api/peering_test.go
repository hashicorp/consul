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
		_, _, err := peerings.Read(ctx, "", nil)
		require.EqualError(t, err, "peering name cannot be empty")
	})

	t.Run("read peer that does not exist on server", func(t *testing.T) {
		resp, qm, err := peerings.Read(ctx, "peer1", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.Nil(t, resp)
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

	testutil.RunStep(t, "list with no peers", func(t *testing.T) {
		// "call List when no peers should exist"
		resp, qm, err := peerings.List(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.Empty(t, resp) // no peerings so this should be empty
	})

	testutil.RunStep(t, "list with some peers", func(t *testing.T) {
		// "call List when peers are present"
		resp1, wm, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer1"}, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotNil(t, resp1)

		resp2, wm, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer2"}, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotNil(t, resp2)

		peering1, qm, err := peerings.Read(ctx, "peer1", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotNil(t, peering1)

		peering2, qm, err := peerings.Read(ctx, "peer2", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotNil(t, peering2)

		peeringsList, qm, err := peerings.List(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)

		require.Len(t, peeringsList, 2)
		require.True(t, peerExistsInPeerListings(peering1, peeringsList), "expected to find peering in list response")
		require.True(t, peerExistsInPeerListings(peering2, peeringsList), "expected to find peering in list response")
	})
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
		_, _, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer2", Datacenter: "dc2"}, nil)
		require.Error(t, err)
	})
}

// TODO(peering): cover the following test cases: bad/ malformed input, peering with wrong token,
// peering with the wrong PeerName

// TestAPI_Peering_GenerateToken_Read_Establish_Delete tests the following use case:
// a server creates a peering token, reads the token, then another server calls establish peering
// finally, we delete the token on the first server
func TestAPI_Peering_GenerateToken_Read_Establish_Delete(t *testing.T) {
	t.Parallel()

	c, s := makeClientWithCA(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultCtxDuration)
	defer cancel()

	var token1 string
	testutil.RunStep(t, "generate token", func(t *testing.T) {
		// Generate a token happy path
		p1 := PeeringGenerateTokenRequest{
			PeerName: "peer1",
			Meta:     map[string]string{"foo": "bar"},
		}
		resp, wm, err := c.Peerings().GenerateToken(ctx, p1, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotNil(t, resp)

		token1 = resp.PeeringToken
	})

	testutil.RunStep(t, "verify token", func(t *testing.T) {
		// Read token generated on server
		resp, qm, err := c.Peerings().Read(ctx, "peer1", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotNil(t, resp)

		// token specific assertions on the "server"
		require.Equal(t, "peer1", resp.Name)
		require.Equal(t, PeeringStateInitial, resp.State)
		require.Equal(t, map[string]string{"foo": "bar"}, resp.Meta)
	})

	// make a "client" server in second DC for peering
	c2, s2 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Datacenter = "dc2"
	})
	defer s2.Stop()

	testutil.RunStep(t, "establish peering", func(t *testing.T) {
		i := PeeringEstablishRequest{
			Datacenter:   c2.config.Datacenter,
			PeerName:     "peer1",
			PeeringToken: token1,
			Meta:         map[string]string{"foo": "bar"},
		}

		_, wm, err := c2.Peerings().Establish(ctx, i, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)

		retry.Run(t, func(r *retry.R) {
			resp, qm, err := c2.Peerings().Read(ctx, "peer1", nil)
			require.NoError(r, err)
			require.NotNil(r, qm)

			// require that the peering state is not undefined
			require.Equal(r, PeeringStateInitial, resp.State)
			require.Equal(r, map[string]string{"foo": "bar"}, resp.Meta)

			// TODO(peering) -- let's go all the way and test in code either here or somewhere else that PeeringState does move to Active
		})
	})

	testutil.RunStep(t, "delete peering at source", func(t *testing.T) {
		// Delete the token on server 1
		wm, err := c.Peerings().Delete(ctx, "peer1", nil)
		require.NoError(t, err)
		require.NotNil(t, wm)

		// Read to see if the token is gone
		retry.Run(t, func(r *retry.R) {
			resp, qm, err := c.Peerings().Read(ctx, "peer1", nil)
			require.NoError(r, err)
			require.NotNil(r, qm)
			require.Nil(r, resp)
		})
	})
}
