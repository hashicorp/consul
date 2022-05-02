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

// TestAPI_Peering_GenerateToken_Read_Initiate tests the following use case:
// a server creates a peering token, reads the token, then another server calls initiate peering
func TestAPI_Peering_GenerateToken_Read_Initiate(t *testing.T) {
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
	t.Run("Generate a token happy path", func(t *testing.T) {
		resp, qq, err := peerings.GenerateToken(ctx, p1, options)
		token1 = resp.PeeringToken

		require.NoError(t, err)
		require.NotEmpty(t, qq)
		require.NotEmpty(t, resp)
	})

	t.Run("Read token generated on \"server\"", func(t *testing.T) {
		resp, qq, err := peerings.Read(ctx, "peer1", nil)

		// basic ok checking
		require.NoError(t, err)
		require.NotEmpty(t, qq)
		require.NotEmpty(t, resp)

		// token specific assertions on the "server"
		require.Equal(t, "peer1", resp.Name)
		require.Equal(t, "default", resp.Partition)
		require.Equal(t, INITIAL, resp.State)

	})

	t.Run("Initiate peering", func(t *testing.T) {
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

		respi, wm, err := c2.Peerings().Initiate(ctx, i, options)

		// basic checks
		require.NoError(t, err)
		require.NotEmpty(t, wm)

		// at first the token will be undefined
		require.Equal(t, UNDEFINED, PeeringState(respi.Status))

		// wait for the peering backend to finish the peering connection
		time.Sleep(2 * time.Second)

		respr, qq, err := c2.Peerings().Read(ctx, "peer1", nil)

		// basic ok checking
		require.NoError(t, err)
		require.NotEmpty(t, qq)

		// require that the peering state is not undefined
		require.Equal(t, INITIAL, respr.State)

		// TODO(peering) -- let's go all the way and test in code either here or somewhere else that PeeringState does move to Active
		// require.Equal(t, PeeringState_ACTIVE, respr.State)
	})

}
