// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/blockingquery"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestGatewayLocator(t *testing.T) {
	state := state.NewStateStore(nil)

	serverRoles := []string{"leader", "follower"}
	now := time.Now().UTC()

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

	t.Run("primary - no data", func(t *testing.T) {
		for _, role := range serverRoles {
			t.Run(role, func(t *testing.T) {
				isLeader := role == "leader"

				logger := testutil.Logger(t)
				tsd := &testServerDelegate{State: state, isLeader: isLeader}
				if !isLeader {
					tsd.lastContact = now
				}
				g := NewGatewayLocator(
					logger,
					tsd,
					"dc1",
					"dc1",
				)
				g.SetUseReplicationSignal(isLeader)

				t.Run("before first run", func(t *testing.T) {
					assert.False(t, g.DialPrimaryThroughLocalGateway()) // not important
					assert.Len(t, tsd.Calls, 0)
					assert.Equal(t, []string(nil), g.listGateways(false))
					assert.Equal(t, []string(nil), g.listGateways(true))
					assert.False(t, tsd.datacenterSupportsFederationStates())
				})

				idx, err := g.runOnce(0)
				require.NoError(t, err)
				assert.Equal(t, uint64(1), idx)

				t.Run("after first run", func(t *testing.T) {
					assert.False(t, g.DialPrimaryThroughLocalGateway()) // not important
					assert.Len(t, tsd.Calls, 1)
					assert.Equal(t, []string(nil), g.listGateways(false))
					assert.Equal(t, []string(nil), g.listGateways(true))
					assert.False(t, tsd.datacenterSupportsFederationStates()) // no results, so we don't flip the bit yet
				})
			})
		}
	})

	t.Run("secondary - no data", func(t *testing.T) {
		for _, role := range serverRoles {
			t.Run(role, func(t *testing.T) {
				isLeader := role == "leader"

				logger := testutil.Logger(t)
				tsd := &testServerDelegate{State: state, isLeader: isLeader}
				if !isLeader {
					tsd.lastContact = now
				}
				g := NewGatewayLocator(
					logger,
					tsd,
					"dc2",
					"dc1",
				)
				g.SetUseReplicationSignal(isLeader)

				t.Run("before first run", func(t *testing.T) {
					assert.True(t, g.DialPrimaryThroughLocalGateway()) // defaults to sure!
					assert.Len(t, tsd.Calls, 0)
					assert.Equal(t, []string(nil), g.listGateways(false))
					assert.Equal(t, []string(nil), g.listGateways(true))
					assert.False(t, tsd.datacenterSupportsFederationStates())
				})

				idx, err := g.runOnce(0)
				require.NoError(t, err)
				assert.Equal(t, uint64(1), idx)

				t.Run("after first run", func(t *testing.T) {
					assert.True(t, g.DialPrimaryThroughLocalGateway()) // defaults to sure!
					assert.Len(t, tsd.Calls, 1)
					assert.Equal(t, []string(nil), g.listGateways(false))
					assert.Equal(t, []string(nil), g.listGateways(true))
					assert.False(t, tsd.datacenterSupportsFederationStates()) // no results, so we don't flip the bit yet
				})
			})
		}
	})

	t.Run("secondary - just fallback", func(t *testing.T) {
		for _, role := range serverRoles {
			t.Run(role, func(t *testing.T) {
				isLeader := role == "leader"

				logger := testutil.Logger(t)
				tsd := &testServerDelegate{State: state, isLeader: isLeader}
				if !isLeader {
					tsd.lastContact = now
				}
				g := NewGatewayLocator(
					logger,
					tsd,
					"dc2",
					"dc1",
				)
				g.SetUseReplicationSignal(isLeader)
				g.RefreshPrimaryGatewayFallbackAddresses([]string{
					"7.7.7.7:7777",
					"8.8.8.8:8888",
				})

				t.Run("before first run", func(t *testing.T) {
					assert.True(t, g.DialPrimaryThroughLocalGateway()) // defaults to sure!
					assert.Len(t, tsd.Calls, 0)
					assert.Equal(t, []string(nil), g.listGateways(false))
					assert.Equal(t, []string(nil), g.listGateways(true)) // don't return any data until we initialize
					assert.False(t, tsd.datacenterSupportsFederationStates())
				})

				idx, err := g.runOnce(0)
				require.NoError(t, err)
				assert.Equal(t, uint64(1), idx)

				t.Run("after first run", func(t *testing.T) {
					assert.True(t, g.DialPrimaryThroughLocalGateway()) // defaults to sure!
					assert.Len(t, tsd.Calls, 1)
					assert.Equal(t, []string(nil), g.listGateways(false))
					assert.Equal(t, []string{
						"7.7.7.7:7777",
						"8.8.8.8:8888",
					}, g.listGateways(true))
					assert.False(t, tsd.datacenterSupportsFederationStates()) // no results, so we don't flip the bit yet
				})
			})
		}
	})

	// Insert data for the dcs
	require.NoError(t, state.FederationStateSet(1, dc1))
	require.NoError(t, state.FederationStateSet(2, dc2))

	t.Run("primary - with data", func(t *testing.T) {
		for _, role := range serverRoles {
			t.Run(role, func(t *testing.T) {
				isLeader := role == "leader"

				logger := testutil.Logger(t)
				tsd := &testServerDelegate{State: state, isLeader: isLeader}
				if !isLeader {
					tsd.lastContact = now
				}
				g := NewGatewayLocator(
					logger,
					tsd,
					"dc1",
					"dc1",
				)
				g.SetUseReplicationSignal(isLeader)

				t.Run("before first run", func(t *testing.T) {
					assert.False(t, g.DialPrimaryThroughLocalGateway()) // not important
					assert.Len(t, tsd.Calls, 0)
					assert.Equal(t, []string(nil), g.listGateways(false))
					assert.Equal(t, []string(nil), g.listGateways(true)) // don't return any data until we initialize
					assert.False(t, tsd.datacenterSupportsFederationStates())
				})

				idx, err := g.runOnce(0)
				require.NoError(t, err)
				assert.Equal(t, uint64(2), idx)

				t.Run("after first run", func(t *testing.T) {
					assert.False(t, g.DialPrimaryThroughLocalGateway()) // not important
					assert.Len(t, tsd.Calls, 1)
					assert.Equal(t, []string{
						"1.2.3.4:5555",
						"4.3.2.1:9999",
					}, g.listGateways(false))
					assert.Equal(t, []string{
						"1.2.3.4:5555",
						"4.3.2.1:9999",
					}, g.listGateways(true))
					assert.True(t, tsd.datacenterSupportsFederationStates()) // have results, so we flip the bit
				})
			})
		}
	})

	t.Run("secondary - with data", func(t *testing.T) {
		for _, role := range serverRoles {
			t.Run(role, func(t *testing.T) {
				isLeader := role == "leader"

				logger := testutil.Logger(t)
				tsd := &testServerDelegate{State: state, isLeader: isLeader}
				if !isLeader {
					tsd.lastContact = now
				}
				g := NewGatewayLocator(
					logger,
					tsd,
					"dc2",
					"dc1",
				)
				g.SetUseReplicationSignal(isLeader)

				t.Run("before first run", func(t *testing.T) {
					assert.True(t, g.DialPrimaryThroughLocalGateway()) // defaults to sure!
					assert.Len(t, tsd.Calls, 0)
					assert.Equal(t, []string(nil), g.listGateways(false))
					assert.Equal(t, []string(nil), g.listGateways(true)) // don't return any data until we initialize
					assert.False(t, tsd.datacenterSupportsFederationStates())
				})

				idx, err := g.runOnce(0)
				require.NoError(t, err)
				assert.Equal(t, uint64(2), idx)

				t.Run("after first run", func(t *testing.T) {
					assert.True(t, g.DialPrimaryThroughLocalGateway()) // defaults to sure!
					assert.Len(t, tsd.Calls, 1)
					assert.Equal(t, []string{
						"5.6.7.8:5555",
						"8.7.6.5:9999",
					}, g.listGateways(false))
					assert.Equal(t, []string{
						"5.6.7.8:5555",
						"8.7.6.5:9999",
					}, g.listGateways(true))
					assert.True(t, tsd.datacenterSupportsFederationStates()) // have results, so we flip the bit
				})

			})
		}
	})

	t.Run("secondary - with data and fallback - repl ok", func(t *testing.T) {
		// Only run for the leader.
		logger := testutil.Logger(t)
		tsd := &testServerDelegate{State: state, isLeader: true}
		g := NewGatewayLocator(
			logger,
			tsd,
			"dc2",
			"dc1",
		)
		g.SetUseReplicationSignal(true)

		g.RefreshPrimaryGatewayFallbackAddresses([]string{
			"7.7.7.7:7777",
			"8.8.8.8:8888",
		})

		g.SetLastFederationStateReplicationError(nil, true)

		t.Run("before first run", func(t *testing.T) {
			assert.True(t, g.DialPrimaryThroughLocalGateway()) // defaults to sure!
			assert.Len(t, tsd.Calls, 0)
			assert.Equal(t, []string(nil), g.listGateways(false))
			assert.Equal(t, []string(nil), g.listGateways(true)) // don't return any data until we initialize
			assert.False(t, tsd.datacenterSupportsFederationStates())
		})

		idx, err := g.runOnce(0)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), idx)

		t.Run("after first run", func(t *testing.T) {
			assert.True(t, g.DialPrimaryThroughLocalGateway())
			assert.Len(t, tsd.Calls, 1)
			assert.Equal(t, []string{
				"5.6.7.8:5555",
				"8.7.6.5:9999",
			}, g.listGateways(false))
			assert.Equal(t, []string{
				"5.6.7.8:5555",
				"8.7.6.5:9999",
			}, g.listGateways(true))
			assert.True(t, tsd.datacenterSupportsFederationStates()) // have results, so we flip the bit
		})
	})

	t.Run("secondary - with data and fallback - repl ok then failed 2 times", func(t *testing.T) {
		// Only run for the leader.
		logger := testutil.Logger(t)
		tsd := &testServerDelegate{State: state, isLeader: true}
		g := NewGatewayLocator(
			logger,
			tsd,
			"dc2",
			"dc1",
		)
		g.SetUseReplicationSignal(true)

		g.RefreshPrimaryGatewayFallbackAddresses([]string{
			"7.7.7.7:7777",
			"8.8.8.8:8888",
		})

		g.SetLastFederationStateReplicationError(nil, true)
		g.SetLastFederationStateReplicationError(errors.New("fake"), true)
		g.SetLastFederationStateReplicationError(errors.New("fake"), true)

		t.Run("before first run", func(t *testing.T) {
			assert.True(t, g.DialPrimaryThroughLocalGateway()) // defaults to sure!
			assert.Len(t, tsd.Calls, 0)
			assert.Equal(t, []string(nil), g.listGateways(false))
			assert.Equal(t, []string(nil), g.listGateways(true)) // don't return any data until we initialize
			assert.False(t, tsd.datacenterSupportsFederationStates())
		})

		idx, err := g.runOnce(0)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), idx)

		t.Run("after first run", func(t *testing.T) {
			assert.True(t, g.DialPrimaryThroughLocalGateway())
			assert.Len(t, tsd.Calls, 1)
			assert.Equal(t, []string{
				"5.6.7.8:5555",
				"8.7.6.5:9999",
			}, g.listGateways(false))
			assert.Equal(t, []string{
				"5.6.7.8:5555",
				"8.7.6.5:9999",
			}, g.listGateways(true))
			assert.True(t, tsd.datacenterSupportsFederationStates()) // have results, so we flip the bit
		})
	})

	t.Run("secondary - with data and fallback - repl ok then failed 3 times", func(t *testing.T) {
		// Only run for the leader.
		logger := testutil.Logger(t)
		tsd := &testServerDelegate{State: state, isLeader: true}
		g := NewGatewayLocator(
			logger,
			tsd,
			"dc2",
			"dc1",
		)
		g.SetUseReplicationSignal(true)

		g.RefreshPrimaryGatewayFallbackAddresses([]string{
			"7.7.7.7:7777",
			"8.8.8.8:8888",
		})

		g.SetLastFederationStateReplicationError(nil, true)
		g.SetLastFederationStateReplicationError(errors.New("fake"), true)
		g.SetLastFederationStateReplicationError(errors.New("fake"), true)
		g.SetLastFederationStateReplicationError(errors.New("fake"), true)

		t.Run("before first run", func(t *testing.T) {
			assert.False(t, g.DialPrimaryThroughLocalGateway()) // too many errors
			assert.Len(t, tsd.Calls, 0)
			assert.Equal(t, []string(nil), g.listGateways(false))
			assert.Equal(t, []string(nil), g.listGateways(true)) // don't return any data until we initialize
			assert.False(t, tsd.datacenterSupportsFederationStates())
		})

		idx, err := g.runOnce(0)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), idx)

		t.Run("after first run", func(t *testing.T) {
			assert.False(t, g.DialPrimaryThroughLocalGateway())
			assert.Len(t, tsd.Calls, 1)
			assert.Equal(t, []string{
				"5.6.7.8:5555",
				"8.7.6.5:9999",
			}, g.listGateways(false))
			assert.Equal(t, []string{
				"1.2.3.4:5555",
				"4.3.2.1:9999",
				"7.7.7.7:7777",
				"8.8.8.8:8888",
			}, g.listGateways(true))
			assert.True(t, tsd.datacenterSupportsFederationStates()) // have results, so we flip the bit
		})
	})

	t.Run("secondary - with data and fallback - repl ok then failed 3 times then ok again", func(t *testing.T) {
		// Only run for the leader.
		logger := testutil.Logger(t)
		tsd := &testServerDelegate{State: state, isLeader: true}
		g := NewGatewayLocator(
			logger,
			tsd,
			"dc2",
			"dc1",
		)
		g.SetUseReplicationSignal(true)

		g.RefreshPrimaryGatewayFallbackAddresses([]string{
			"7.7.7.7:7777",
			"8.8.8.8:8888",
		})

		g.SetLastFederationStateReplicationError(nil, true)
		g.SetLastFederationStateReplicationError(errors.New("fake"), true)
		g.SetLastFederationStateReplicationError(errors.New("fake"), true)
		g.SetLastFederationStateReplicationError(errors.New("fake"), true)
		g.SetLastFederationStateReplicationError(nil, true)

		t.Run("before first run", func(t *testing.T) {
			assert.True(t, g.DialPrimaryThroughLocalGateway()) // all better again
			assert.Len(t, tsd.Calls, 0)
			assert.Equal(t, []string(nil), g.listGateways(false))
			assert.Equal(t, []string(nil), g.listGateways(true)) // don't return any data until we initialize
			assert.False(t, tsd.datacenterSupportsFederationStates())
		})

		idx, err := g.runOnce(0)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), idx)

		t.Run("after first run", func(t *testing.T) {
			assert.True(t, g.DialPrimaryThroughLocalGateway()) // all better again
			assert.Len(t, tsd.Calls, 1)
			assert.Equal(t, []string{
				"5.6.7.8:5555",
				"8.7.6.5:9999",
			}, g.listGateways(false))
			assert.Equal(t, []string{
				"5.6.7.8:5555",
				"8.7.6.5:9999",
			}, g.listGateways(true))
			assert.True(t, tsd.datacenterSupportsFederationStates()) // have results, so we flip the bit
		})
	})
}

var _ serverDelegate = (*testServerDelegate)(nil)

type testServerDelegate struct {
	dcSupportsFederationStates int32 // atomically accessed, at start to prevent alignment issues

	State *state.Store

	Calls []uint64

	isLeader    bool
	lastContact time.Time
}

func (d *testServerDelegate) setDatacenterSupportsFederationStates() {
	atomic.StoreInt32(&d.dcSupportsFederationStates, 1)
}

func (d *testServerDelegate) datacenterSupportsFederationStates() bool {
	return atomic.LoadInt32(&d.dcSupportsFederationStates) != 0
}

// This is just enough to exercise the logic.
func (d *testServerDelegate) blockingQuery(
	queryOpts blockingquery.RequestOptions,
	queryMeta blockingquery.ResponseMeta,
	fn blockingquery.QueryFn,
) error {
	minQueryIndex := queryOpts.GetMinQueryIndex()

	d.Calls = append(d.Calls, minQueryIndex)

	var ws memdb.WatchSet

	err := fn(ws, d.State)
	if err == nil && queryMeta.GetIndex() < 1 {
		queryMeta.SetIndex(1)
	}

	return err
}

func (d *testServerDelegate) IsLeader() bool {
	return d.isLeader
}

func (d *testServerDelegate) LeaderLastContact() time.Time {
	return d.lastContact
}
