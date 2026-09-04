// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// failoverChainForTest compiles a "db" discovery chain that fails over to
// "fail", and returns the chain along with its resolver node.
func failoverChainForTest(t *testing.T) (*structs.CompiledDiscoveryChain, *structs.DiscoveryGraphNode) {
	t.Helper()

	set := configentry.NewDiscoveryChainSet()
	set.AddEntries(
		&structs.ProxyConfigEntry{
			Kind:   structs.ProxyDefaults,
			Name:   structs.ProxyConfigGlobal,
			Config: map[string]interface{}{"protocol": "http"},
		},
		&structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "db",
			Failover: map[string]structs.ServiceResolverFailover{
				"*": {Service: "fail"},
			},
		},
	)

	chain := discoverychain.TestCompileConfigEntries(
		t, "db", "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)

	var node *structs.DiscoveryGraphNode
	for _, n := range chain.Nodes {
		if n.Type == structs.DiscoveryGraphNodeTypeResolver && n.Resolver != nil && n.Resolver.Failover != nil {
			node = n
			break
		}
	}
	require.NotNil(t, node, "expected a resolver node carrying a failover definition")
	require.NotEmpty(t, node.Resolver.Failover.Targets, "expected the resolver node to have failover targets")

	return chain, node
}

// TestMapDiscoChainTargets_APIGatewayAggregateGuard covers the guard that stops
// Consul from handing Envoy an aggregate cluster whose member clusters have no
// EDS assignment. That shape crashes Envoy during worker startup rather than
// degrading (envoyproxy/envoy#35157), so when a member is not ready we render
// the base cluster as a plain EDS cluster over the primary target instead.
func TestMapDiscoChainTargets_APIGatewayAggregateGuard(t *testing.T) {
	chain, node := failoverChainForTest(t)

	primaryTargetID := node.Resolver.Target
	failoverTargetID := node.Resolver.Failover.Targets[0]
	require.NotEqual(t, primaryTargetID, failoverTargetID)

	uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("db", nil))

	// endpointsFor builds the WatchedUpstreamEndpoints map for the given target
	// IDs. makeLoadAssignmentEndpointGroup keys readiness off the presence of
	// the target ID, so an absent key is exactly the "endpoints not assembled
	// yet" state that the two-phase watch produces on cold start.
	endpointsFor := func(t *testing.T, targetIDs ...string) map[string]structs.CheckServiceNodes {
		out := map[string]structs.CheckServiceNodes{}
		for _, id := range targetIDs {
			out[id] = proxycfg.TestUpstreamNodes(t, "db")
		}
		return out
	}

	cases := map[string]struct {
		kind             structs.ServiceKind
		readyTargets     []string
		expectFailover   bool
		expectTargetIDs  []string
		expectClusterQty int
	}{
		"api gateway, all members ready: aggregate is preserved": {
			kind:             structs.ServiceKindAPIGateway,
			readyTargets:     []string{primaryTargetID, failoverTargetID},
			expectFailover:   true,
			expectTargetIDs:  []string{primaryTargetID, failoverTargetID},
			expectClusterQty: 2,
		},
		"api gateway, failover member unready: degrades to the primary": {
			kind:             structs.ServiceKindAPIGateway,
			readyTargets:     []string{primaryTargetID},
			expectFailover:   false,
			expectTargetIDs:  []string{primaryTargetID},
			expectClusterQty: 1,
		},
		"api gateway, no members ready: degrades to the primary": {
			kind:             structs.ServiceKindAPIGateway,
			readyTargets:     nil,
			expectFailover:   false,
			expectTargetIDs:  []string{primaryTargetID},
			expectClusterQty: 1,
		},
		// The crash is only reachable through a gateway cold start, and the
		// user-facing contract for sidecars is unchanged, so the guard is
		// deliberately scoped to API gateways.
		"connect proxy is out of scope: aggregate is preserved when unready": {
			kind:             structs.ServiceKindConnectProxy,
			readyTargets:     nil,
			expectFailover:   true,
			expectTargetIDs:  []string{primaryTargetID, failoverTargetID},
			expectClusterQty: 2,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			snap := testAggregateGuardSnapshot(t, tc.kind)

			upstreams, err := snap.ToConfigSnapshotUpstreams()
			require.NoError(t, err)
			upstreams.DiscoveryChain[uid] = chain
			upstreams.WatchedUpstreamEndpoints[uid] = endpointsFor(t, tc.readyTargets...)

			s := &ResourceGenerator{Logger: testutil.Logger(t)}

			mapped, err := s.mapDiscoChainTargets(snap, uid, chain, node, structs.UpstreamConfig{}, false, "")
			require.NoError(t, err)

			require.Equal(t, tc.expectFailover, mapped.failover)
			require.Equal(t, tc.expectFailover, mapped.isAggregateCluster(),
				"RDS retry-policy selection must follow the same decision as CDS")

			var gotTargetIDs []string
			for _, ti := range mapped.targets {
				gotTargetIDs = append(gotTargetIDs, ti.TargetID)
			}
			require.Equal(t, tc.expectTargetIDs, gotTargetIDs)

			// groupedTargets is what clusters.go and endpoints.go both consume,
			// so assert the shape Envoy actually receives.
			groups, err := mapped.groupedTargets()
			require.NoError(t, err)
			require.Len(t, groups, tc.expectClusterQty)
			for _, g := range groups {
				require.Len(t, g.Targets, 1,
					"endpoint generation rejects groups that do not hold exactly one target")
			}

			if !tc.expectFailover {
				require.Equal(t, mapped.baseClusterName, groups[0].ClusterName,
					"the degraded cluster must keep the base name so route destinations still resolve")
				require.Empty(t, mapped.failoverPolicy.Mode)
			}
		})
	}
}

func testAggregateGuardSnapshot(t *testing.T, kind structs.ServiceKind) *proxycfg.ConfigSnapshot {
	t.Helper()

	switch kind {
	case structs.ServiceKindAPIGateway:
		return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, nil, nil, nil, nil)
	case structs.ServiceKindConnectProxy:
		return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover", false, nil, nil)
	default:
		t.Fatalf("unsupported kind %q", kind)
		return nil
	}
}

// TestDegradeToSingleTarget_DroppedPrimary covers the case where the primary
// target is absent from the mapped targets, which happens when it is a peered
// target whose peering metadata has not resolved and it is therefore dropped
// while mapping. The guard must still clear failover: leaving the shape
// untouched would emit an aggregate over the remaining unready members, which
// is precisely the crash the guard exists to prevent.
func TestDegradeToSingleTarget_DroppedPrimary(t *testing.T) {
	const (
		primary   = "db.default.default.dc1"
		failoverA = "fail-a.default.default.dc1"
		failoverB = "fail-b.default.default.dc1"
	)

	cases := map[string]struct {
		targets      []targetInfo
		expectChosen string
		expectTarget []string
	}{
		"primary present: degrades onto the primary": {
			targets: []targetInfo{
				{TargetID: primary}, {TargetID: failoverA}, {TargetID: failoverB},
			},
			expectChosen: primary,
			expectTarget: []string{primary},
		},
		"primary present but not first: still degrades onto the primary": {
			targets: []targetInfo{
				{TargetID: failoverA}, {TargetID: primary},
			},
			expectChosen: primary,
			expectTarget: []string{primary},
		},
		"primary dropped: degrades onto the highest-priority remaining target": {
			targets: []targetInfo{
				{TargetID: failoverA}, {TargetID: failoverB},
			},
			expectChosen: failoverA,
			expectTarget: []string{failoverA},
		},
		"every target dropped: clears failover and leaves nothing to emit": {
			targets:      nil,
			expectChosen: "",
			expectTarget: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ft := discoChainTargets{
				baseClusterName: "db.default.dc1.internal.example.consul",
				targets:         tc.targets,
				failover:        true,
				failoverPolicy:  structs.ServiceResolverFailoverPolicy{Mode: "sequential"},
			}

			chosen := ft.degradeToSingleTarget(primary)
			require.Equal(t, tc.expectChosen, chosen)

			// Failover must be cleared unconditionally. This is the invariant
			// that keeps clusters.go from emitting an aggregate cluster.
			require.False(t, ft.failover)
			require.False(t, ft.isAggregateCluster())
			require.Empty(t, ft.failoverPolicy.Mode)

			var gotTargetIDs []string
			for _, ti := range ft.targets {
				gotTargetIDs = append(gotTargetIDs, ti.TargetID)
			}
			require.Equal(t, tc.expectTarget, gotTargetIDs)

			groups, err := ft.groupedTargets()
			require.NoError(t, err)

			if tc.expectChosen == "" {
				// One group holding zero targets: clusters.go emits no
				// aggregate, and endpoints.go skips the zero-length group.
				require.Len(t, groups, 1)
				require.Empty(t, groups[0].Targets)
				return
			}

			require.Len(t, groups, 1)
			require.Equal(t, ft.baseClusterName, groups[0].ClusterName)
			require.Len(t, groups[0].Targets, 1)
			require.Equal(t, tc.expectChosen, groups[0].Targets[0].TargetID)
		})
	}
}
