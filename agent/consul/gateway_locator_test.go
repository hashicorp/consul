package consul

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
)

func TestGatewayLocator(t *testing.T) {
	state, err := state.NewStateStore(nil)
	require.NoError(t, err)

	dc1 := &structs.FederationState{
		Datacenter: "dc1",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway1", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc1", "gateway2", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}
	dc2 := &structs.FederationState{
		Datacenter: "dc2",
		MeshGateways: []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc2", "gateway1", "5.6.7.8", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc2", "gateway2", "8.7.6.5", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		UpdatedAt: time.Now().UTC(),
	}

	// Insert data for the dcs
	require.NoError(t, state.FederationStateSet(1, dc1))
	require.NoError(t, state.FederationStateSet(2, dc2))

	t.Run("primary", func(t *testing.T) {
		logger := testutil.Logger(t)
		tsd := &testServerDelegate{State: state}
		g := NewGatewayLocator(
			logger,
			tsd,
			"dc1",
			"dc1",
		)

		idx, err := g.runOnce(0)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, tsd.Calls, 1)
		require.Equal(t, []string{
			"1.2.3.4:5555",
			"4.3.2.1:9999",
		}, g.listGateways(false))
		require.Equal(t, []string{
			"1.2.3.4:5555",
			"4.3.2.1:9999",
		}, g.listGateways(true))
	})

	t.Run("secondary", func(t *testing.T) {
		logger := testutil.Logger(t)
		tsd := &testServerDelegate{State: state}
		g := NewGatewayLocator(
			logger,
			tsd,
			"dc2",
			"dc1",
		)

		idx, err := g.runOnce(0)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, tsd.Calls, 1)
		require.Equal(t, []string{
			"5.6.7.8:5555",
			"8.7.6.5:9999",
		}, g.listGateways(false))
		require.Equal(t, []string{
			"1.2.3.4:5555",
			"4.3.2.1:9999",
		}, g.listGateways(true))
	})
}

type testServerDelegate struct {
	State *state.Store

	FallbackAddrs []string
	Calls         []uint64
}

// This is just enough to exercise the logic.
func (d *testServerDelegate) blockingQuery(
	queryOpts structs.QueryOptionsCompat,
	queryMeta structs.QueryMetaCompat,
	fn queryFn,
) error {
	minQueryIndex := queryOpts.GetMinQueryIndex()

	d.Calls = append(d.Calls, minQueryIndex)
	// if minQueryIndex == 0 {
	// 	goto RUN_QUERY
	// }

	// RUN_QUERY:

	var ws memdb.WatchSet
	// if minQueryIndex > 0 {
	// 	ws = memdb.NewWatchSet()
	// }

	err := fn(ws, d.State)
	if err == nil && queryMeta.GetIndex() < 1 {
		queryMeta.SetIndex(1)
	}

	return err
}

func newFakeStateStore() (*state.Store, error) {
	return state.NewStateStore(nil)
}

func (d *testServerDelegate) PrimaryGatewayFallbackAddresses() []string {
	return d.FallbackAddrs
}
