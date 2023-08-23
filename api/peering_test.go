package api

import (
	"context"
	"encoding/base64"
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
			(peer.ModifyIndex == aPeer.ModifyIndex) &&
			(peer.ImportedServiceCount == aPeer.ImportedServiceCount) &&
			(peer.ExportedServiceCount == aPeer.ExportedServiceCount)

		if isEqual {
			return true
		}
	}

	return false
}

func TestAPI_Peering_ACLDeny(t *testing.T) {
	c1, s1 := makeClientWithConfig(t, nil, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.ACL.Tokens.InitialManagement = "root"
		serverConfig.ACL.Enabled = true
		serverConfig.ACL.DefaultPolicy = "deny"
		serverConfig.Ports.GRPC = 5300
		serverConfig.Ports.HTTPS = -1
	})
	defer s1.Stop()

	c2, s2 := makeClientWithConfig(t, nil, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.ACL.Tokens.InitialManagement = "root"
		serverConfig.ACL.Enabled = true
		serverConfig.ACL.DefaultPolicy = "deny"
		serverConfig.Ports.GRPC = 5301
		serverConfig.Ports.HTTPS = -1
		serverConfig.Datacenter = "dc2"
	})
	defer s2.Stop()

	var peeringToken string
	testutil.RunStep(t, "generate token", func(t *testing.T) {
		peerings := c1.Peerings()

		req := PeeringGenerateTokenRequest{PeerName: "peer1"}

		testutil.RunStep(t, "without ACL token", func(t *testing.T) {
			_, _, err := peerings.GenerateToken(context.Background(), req, &WriteOptions{Token: "anonymous"})
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, "Permission denied")
		})

		testutil.RunStep(t, "with ACL token", func(t *testing.T) {
			resp, wm, err := peerings.GenerateToken(context.Background(), req, &WriteOptions{Token: "root"})
			require.NoError(t, err)
			require.NotNil(t, wm)
			require.NotNil(t, resp)

			peeringToken = resp.PeeringToken
		})
	})

	testutil.RunStep(t, "establish peering", func(t *testing.T) {
		peerings := c2.Peerings()

		req := PeeringEstablishRequest{
			PeerName:     "peer2",
			PeeringToken: peeringToken,
		}
		testutil.RunStep(t, "without ACL token", func(t *testing.T) {
			_, _, err := peerings.Establish(context.Background(), req, &WriteOptions{Token: "anonymous"})
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, "Permission denied")
		})

		testutil.RunStep(t, "with ACL token", func(t *testing.T) {
			resp, wm, err := peerings.Establish(context.Background(), req, &WriteOptions{Token: "root"})
			require.NoError(t, err)
			require.NotNil(t, wm)
			require.NotNil(t, resp)
		})
	})

	testutil.RunStep(t, "read peering", func(t *testing.T) {
		peerings := c1.Peerings()

		testutil.RunStep(t, "without ACL token", func(t *testing.T) {
			_, _, err := peerings.Read(context.Background(), "peer1", &QueryOptions{Token: "anonymous"})
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, "Permission denied")
		})

		testutil.RunStep(t, "with ACL token", func(t *testing.T) {
			resp, qm, err := peerings.Read(context.Background(), "peer1", &QueryOptions{Token: "root"})
			require.NoError(t, err)
			require.NotNil(t, qm)
			require.NotNil(t, resp)
		})
	})

	testutil.RunStep(t, "list peerings", func(t *testing.T) {
		peerings := c1.Peerings()

		testutil.RunStep(t, "without ACL token", func(t *testing.T) {
			_, _, err := peerings.List(context.Background(), &QueryOptions{Token: "anonymous"})
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, "Permission denied")
		})

		testutil.RunStep(t, "with ACL token", func(t *testing.T) {
			resp, qm, err := peerings.List(context.Background(), &QueryOptions{Token: "root"})
			require.NoError(t, err)
			require.NotNil(t, qm)
			require.NotNil(t, resp)
			require.Len(t, resp, 1)
		})
	})

	testutil.RunStep(t, "delete peering", func(t *testing.T) {
		peerings := c1.Peerings()

		testutil.RunStep(t, "without ACL token", func(t *testing.T) {
			_, err := peerings.Delete(context.Background(), "peer1", &WriteOptions{Token: "anonymous"})
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, "Permission denied")
		})

		testutil.RunStep(t, "with ACL token", func(t *testing.T) {
			wm, err := peerings.Delete(context.Background(), "peer1", &WriteOptions{Token: "root"})
			require.NoError(t, err)
			require.NotNil(t, wm)
		})
	})
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
		// call List when no peers should exist
		resp, qm, err := peerings.List(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.Empty(t, resp) // no peerings so this should be empty
	})

	testutil.RunStep(t, "list with some peers", func(t *testing.T) {
		// call List when peers are present
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

func TestAPI_Peering_GenerateToken_ExternalAddresses(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t) // this is "dc1"
	defer s.Stop()
	s.WaitForSerfCheck(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	externalAddress := "32.1.2.3:8502"

	// Generate a token happy path
	p1 := PeeringGenerateTokenRequest{
		PeerName:                "peer1",
		Meta:                    map[string]string{"foo": "bar"},
		ServerExternalAddresses: []string{externalAddress},
	}
	resp, wm, err := c.Peerings().GenerateToken(ctx, p1, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotNil(t, resp)

	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)

	require.Contains(t, string(tokenJSON), externalAddress)
}

// TestAPI_Peering_GenerateToken_Read_Establish_Delete tests the following use case:
// a server creates a peering token, reads the token, then another server calls establish peering
// finally, we delete the token on the first server
func TestAPI_Peering_GenerateToken_Read_Establish_Delete(t *testing.T) {
	t.Parallel()

	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Datacenter = "dc1"
		conf.Ports.HTTPS = -1
	})
	defer s.Stop()
	s.WaitForSerfCheck(t)

	// make a "client" server in second DC for peering
	c2, s2 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Datacenter = "dc2"
	})
	defer s2.Stop()

	testNodeServiceCheckRegistrations(t, c2, "dc2")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		require.Equal(t, PeeringStatePending, resp.State)
		require.Equal(t, map[string]string{"foo": "bar"}, resp.Meta)
	})

	testutil.RunStep(t, "establish peering", func(t *testing.T) {
		i := PeeringEstablishRequest{
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
			require.Equal(r, PeeringStateEstablishing, resp.State)
			require.Equal(r, map[string]string{"foo": "bar"}, resp.Meta)
		})
	})

	testutil.RunStep(t, "look for active state of peering in dc2", func(t *testing.T) {
		// read and list the peer to make sure the status transitions to active
		retry.Run(t, func(r *retry.R) {
			peering, qm, err := c2.Peerings().Read(ctx, "peer1", nil)
			require.NoError(r, err)
			require.NotNil(r, qm)
			require.NotNil(r, peering)
			require.Equal(r, PeeringStateActive, peering.State)

			peerings, qm, err := c2.Peerings().List(ctx, nil)

			require.NoError(r, err)
			require.NotNil(r, qm)
			require.NotNil(r, peerings)
			require.Equal(r, PeeringStateActive, peerings[0].State)
		})
	})

	testutil.RunStep(t, "look for active state of peering in dc1", func(t *testing.T) {
		// read and list the peer to make sure the status transitions to active
		retry.Run(t, func(r *retry.R) {
			peering, qm, err := c.Peerings().Read(ctx, "peer1", nil)
			require.NoError(r, err)
			require.NotNil(r, qm)
			require.NotNil(r, peering)
			require.Equal(r, PeeringStateActive, peering.State)

			peerings, qm, err := c.Peerings().List(ctx, nil)

			require.NoError(r, err)
			require.NotNil(r, qm)
			require.NotNil(r, peerings)
			require.Equal(r, PeeringStateActive, peerings[0].State)
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
