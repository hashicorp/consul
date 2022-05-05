//go:build !consulent
// +build !consulent

package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_Peering_Read_ErrorHandling_Partitions(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithCA(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultCtxDuration)
	defer cancel()
	peerings := c.Peerings()

	respG, wm, err := peerings.GenerateToken(ctx, PeeringGenerateTokenRequest{PeerName: "peer1"}, nil)
	require.NoError(t, err)
	require.NotEmpty(t, wm)
	require.NotEmpty(t, respG)

	resp, qm, err := peerings.Read(ctx, PeeringReadRequest{Name: "peer1"}, nil)

	require.NoError(t, err)
	require.NotEmpty(t, qm)
	require.NotEmpty(t, resp)

	// token specific assertions on the "server"
	require.Equal(t, "peer1", resp.Name)
	require.Equal(t, "", resp.Partition)
	require.Equal(t, PeeringStateInitial, resp.State)
}
