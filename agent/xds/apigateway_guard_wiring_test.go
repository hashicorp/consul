// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"strings"
	"testing"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
)

// TestNewServer_APIGatewayEnvOverrides covers the only writer of
// Server.BootstrapGateTimeout and Server.DisableAPIGatewayFailoverGuard in the
// running binary.
//
// Both fields document a way to neutralize their feature without a binary
// rollback, which is only true if something actually assigns them outside of
// tests. Asserting the field in isolation cannot catch that: a field can be
// read, honored, and unit-tested while remaining permanently at its zero value
// in production. This test starts from NewServer for that reason.
func TestNewServer_APIGatewayEnvOverrides(t *testing.T) {
	newServer := func(t *testing.T) *Server {
		t.Helper()
		return NewServer("node-1", testutil.Logger(t), nil, nil, nil)
	}

	t.Run("unset env yields the safe defaults", func(t *testing.T) {
		s := newServer(t)
		require.Equal(t, DefaultBootstrapGateTimeout, s.BootstrapGateTimeout)
		require.False(t, s.DisableAPIGatewayFailoverGuard,
			"the guard averts a crash, so it must be on unless explicitly disabled")
	})

	t.Run("bootstrap gate timeout", func(t *testing.T) {
		cases := map[string]struct {
			env    string
			expect time.Duration
		}{
			"override":                {env: "5s", expect: 5 * time.Second},
			"negative disables":       {env: "-1s", expect: -1 * time.Second},
			"malformed falls back":    {env: "not-a-duration", expect: DefaultBootstrapGateTimeout},
			"empty falls back":        {env: "", expect: DefaultBootstrapGateTimeout},
			"bare integer falls back": {env: "30", expect: DefaultBootstrapGateTimeout},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				t.Setenv(EnvBootstrapGateTimeout, tc.env)
				require.Equal(t, tc.expect, newServer(t).BootstrapGateTimeout)
			})
		}

		// A negative value must reach newBootstrapGate as a gate that never
		// withholds anything, which is the behavior the field documents.
		t.Setenv(EnvBootstrapGateTimeout, "-1s")
		require.True(t, newBootstrapGate(newServer(t).BootstrapGateTimeout).open,
			"a negative timeout must produce an already-open gate")
	})

	t.Run("failover guard kill switch", func(t *testing.T) {
		cases := map[string]struct {
			env    string
			expect bool
		}{
			"true":                   {env: "true", expect: true},
			"1":                      {env: "1", expect: true},
			"TRUE":                   {env: "TRUE", expect: true},
			"false":                  {env: "false", expect: false},
			"0":                      {env: "0", expect: false},
			"empty":                  {env: "", expect: false},
			"malformed fails closed": {env: "yes-please", expect: false},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				t.Setenv(EnvDisableAPIGatewayFailoverGuard, tc.env)
				require.Equal(t, tc.expect, newServer(t).DisableAPIGatewayFailoverGuard)
			})
		}
	})
}

// TestGetEnvoyConfiguration_FailoverGuardWiring proves the kill switch survives
// the whole path from the Server field to the clusters Envoy receives.
//
// processDelta reads s.DisableAPIGatewayFailoverGuard and hands it to
// getEnvoyConfiguration, which is the last hop before resource generation, so
// exercising that function with a real api-gateway snapshot covers every link
// except the single field reference in processDelta itself. The assertion is on
// the emitted CDS resources rather than on discoChainTargets, because an
// aggregate cluster is the shape that actually faults Envoy.
func TestGetEnvoyConfiguration_FailoverGuardWiring(t *testing.T) {
	const service = "backend"

	snapshotWithUnreadyFailover := func(t *testing.T) *proxycfg.ConfigSnapshot {
		t.Helper()

		// A gateway with a TCP route to "backend", so cluster generation
		// actually walks this upstream (getReadyListeners is the entry point).
		snap := proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil,
			func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
				entry.Listeners = []structs.APIGatewayListener{{
					Name:     "tcp-listener",
					Protocol: structs.ListenerProtocolTCP,
					Port:     9000,
				}}
				bound.Listeners = []structs.BoundAPIGatewayListener{{
					Name:   "tcp-listener",
					Routes: []structs.ResourceReference{{Kind: structs.TCPRoute, Name: "tcp-route"}},
				}}
			},
			[]structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Kind:     structs.TCPRoute,
					Name:     "tcp-route",
					Parents:  []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gateway"}},
					Services: []structs.TCPService{{Name: service}},
				},
			}, nil, nil)

		// Replace the compiled chain with one that fails over, and leave the
		// failover member's endpoints unassembled -- the state the guard exists
		// to catch.
		set := configentry.NewDiscoveryChainSet()
		set.AddEntries(&structs.ServiceResolverConfigEntry{
			Kind:     structs.ServiceResolver,
			Name:     service,
			Failover: map[string]structs.ServiceResolverFailover{"*": {Service: "backend-fail"}},
		})
		chain := discoverychain.TestCompileConfigEntries(
			t, service, "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)

		uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName(service, nil))
		upstreams, err := snap.ToConfigSnapshotUpstreams()
		require.NoError(t, err)
		upstreams.DiscoveryChain[uid] = chain

		var primaryTargetID string
		for _, node := range chain.Nodes {
			if node.Type == structs.DiscoveryGraphNodeTypeResolver && node.Resolver != nil && node.Resolver.Failover != nil {
				primaryTargetID = node.Resolver.Target
			}
		}
		require.NotEmpty(t, primaryTargetID, "expected a resolver node carrying failover")

		upstreams.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{
			primaryTargetID: proxycfg.TestUpstreamNodes(t, service),
		}
		return snap
	}

	// aggregateClusterNames returns the names of every aggregate cluster in the
	// generated CDS resources. This is the shape that faults Envoy when a member
	// has no EDS assignment.
	aggregateClusterNames := func(t *testing.T, disableGuard bool) []string {
		t.Helper()

		res, err := getEnvoyConfiguration(snapshotWithUnreadyFailover(t), testutil.Logger(t), nil, disableGuard)
		require.NoError(t, err)

		var names []string
		var sawBackendCluster bool
		for _, msg := range res[xdscommon.ClusterType] {
			cluster, ok := msg.(*envoy_cluster_v3.Cluster)
			require.True(t, ok)
			if custom := cluster.GetClusterType(); custom != nil && custom.Name == "envoy.clusters.aggregate" {
				names = append(names, cluster.Name)
			}
			sawBackendCluster = sawBackendCluster || strings.Contains(cluster.Name, service)
		}
		require.True(t, sawBackendCluster,
			"precondition: cluster generation must have reached the %q upstream, otherwise this test is vacuous", service)
		return names
	}

	t.Run("guard enabled: no aggregate cluster is emitted", func(t *testing.T) {
		require.Empty(t, aggregateClusterNames(t, false),
			"an unready failover member must be rendered as a plain EDS cluster")
	})

	t.Run("kill switch disables the guard: the aggregate is emitted", func(t *testing.T) {
		require.NotEmpty(t, aggregateClusterNames(t, true),
			"the kill switch must restore the pre-guard shape all the way to CDS output")
	})
}
