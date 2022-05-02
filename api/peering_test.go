package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

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
	resp2, qm, err2 := peerings.Read(ctx, "peer1", nil)

	// basic ok checking
	require.NoError(t, err2)
	require.NotEmpty(t, qm)
	require.NotEmpty(t, resp2)

	// token specific assertions on the "server"
	require.Equal(t, "peer1", resp2.Name)

	// TODO(peering) -- split in OSS/ ENT test for "default" vs ""; or revisit PartitionOrEmpty vs PartitionOrDefault
	// require.Equal(t, "default", resp2.Partition)
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

	respr, qm2, err4 := c2.Peerings().Read(ctx, "peer1", nil)

	// basic ok checking
	require.NoError(t, err4)
	require.NotEmpty(t, qm2)

	// require that the peering state is not undefined
	require.Equal(t, INITIAL, respr.State)

	// TODO(peering) -- let's go all the way and test in code either here or somewhere else that PeeringState does move to Active
	// require.Equal(t, PeeringState_ACTIVE, respr.State)

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
	resp5, qm4, err6 := peerings.Read(ctx, "peer1", nil)

	// basic checks
	require.NotNil(t, err6)
	require.Empty(t, qm4)
	require.Empty(t, resp5)
}
