// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func compileChain(t *testing.T, c *api.Client, service string, opts *api.DiscoveryChainOptions) *api.CompiledDiscoveryChain {
	t.Helper()
	resp, _, err := c.DiscoveryChain().Get(service, opts, nil)
	require.NoError(t, err)
	require.NotNil(t, resp.Chain)
	return resp.Chain
}

func node(t *testing.T, chain *api.CompiledDiscoveryChain, id string) *api.DiscoveryGraphNode {
	t.Helper()
	n, ok := chain.Nodes[id]
	require.True(t, ok, "node %q not in chain; have %v", id, nodeIDs(chain))
	return n
}

func nodeIDs(chain *api.CompiledDiscoveryChain) []string {
	var ids []string
	for id := range chain.Nodes {
		ids = append(ids, id)
	}
	return ids
}

// resolverNodeIDs returns the IDs of the resolver nodes that make up chain.
func resolverNodeIDs(chain *api.CompiledDiscoveryChain) []string {
	var ids []string
	for id, n := range chain.Nodes {
		if n.Type == api.DiscoveryGraphNodeTypeResolver {
			ids = append(ids, id)
		}
	}
	return ids
}

// TestDiscoveryChain_Default verifies the chain compiled for a service with no
// traffic-management configuration: a single resolver to the service itself.
func TestDiscoveryChain_Default(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	chain := compileChain(t, e.server, "plain", nil)

	require.True(t, chain.Default)
	require.Equal(t, "plain", chain.ServiceName)
	require.Equal(t, "dc1", chain.Datacenter)
	require.Equal(t, "tcp", chain.Protocol)

	start := node(t, chain, chain.StartNode)
	require.Equal(t, api.DiscoveryGraphNodeTypeResolver, start.Type)
	require.True(t, start.Resolver.Default)
	require.Len(t, chain.Targets, 1)
	require.Contains(t, chain.Targets, start.Resolver.Target)
	require.Equal(t, "plain", chain.Targets[start.Resolver.Target].Service)
}

// TestDiscoveryChain_RouterSplitterResolver builds a full chain: an HTTP router
// that sends /api to another service and everything else to a splitter, which
// divides traffic between two resolver subsets.
func TestDiscoveryChain_RouterSplitterResolver(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	c := e.server

	for _, svc := range []string{"shop", "shop-api"} {
		setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: svc, Protocol: "http"})
	}
	setEntry(t, c, &api.ServiceResolverConfigEntry{
		Kind:          api.ServiceResolver,
		Name:          "shop",
		DefaultSubset: "v1",
		Subsets: map[string]api.ServiceResolverSubset{
			"v1": {Filter: "Service.Meta.version == v1"},
			"v2": {Filter: "Service.Meta.version == v2", OnlyPassing: true},
		},
	})
	setEntry(t, c, &api.ServiceSplitterConfigEntry{
		Kind: api.ServiceSplitter,
		Name: "shop",
		Splits: []api.ServiceSplit{
			{Weight: 80, ServiceSubset: "v1"},
			{Weight: 20, ServiceSubset: "v2"},
		},
	})
	setEntry(t, c, &api.ServiceRouterConfigEntry{
		Kind: api.ServiceRouter,
		Name: "shop",
		Routes: []api.ServiceRoute{{
			Match:       &api.ServiceRouteMatch{HTTP: &api.ServiceRouteHTTPMatch{PathPrefix: "/api"}},
			Destination: &api.ServiceRouteDestination{Service: "shop-api", RequestTimeout: 3 * time.Second},
		}},
	})

	chain := compileChain(t, c, "shop", nil)

	require.False(t, chain.Default)
	require.Equal(t, "http", chain.Protocol)

	router := node(t, chain, chain.StartNode)
	require.Equal(t, api.DiscoveryGraphNodeTypeRouter, router.Type)
	require.Len(t, router.Routes, 2, "the explicit route plus the implicit catch-all")

	t.Run("explicit route goes to the other service", func(t *testing.T) {
		route := router.Routes[0]
		require.Equal(t, "/api", route.Definition.Match.HTTP.PathPrefix)
		require.Equal(t, 3*time.Second, route.Definition.Destination.RequestTimeout)

		next := node(t, chain, route.NextNode)
		require.Equal(t, api.DiscoveryGraphNodeTypeResolver, next.Type)
		require.Equal(t, "shop-api", chain.Targets[next.Resolver.Target].Service)
	})

	t.Run("catch-all route goes through the splitter to the subsets", func(t *testing.T) {
		next := node(t, chain, router.Routes[1].NextNode)
		require.Equal(t, api.DiscoveryGraphNodeTypeSplitter, next.Type)
		require.Len(t, next.Splits, 2)
		require.InDelta(t, 80, next.Splits[0].Weight, 0.001)
		require.InDelta(t, 20, next.Splits[1].Weight, 0.001)

		subsets := map[string]api.ServiceResolverSubset{}
		for _, split := range next.Splits {
			resolver := node(t, chain, split.NextNode)
			require.Equal(t, api.DiscoveryGraphNodeTypeResolver, resolver.Type)
			target := chain.Targets[resolver.Resolver.Target]
			require.Equal(t, "shop", target.Service)
			subsets[target.ServiceSubset] = target.Subset
		}
		require.Equal(t, "Service.Meta.version == v1", subsets["v1"].Filter)
		require.Equal(t, "Service.Meta.version == v2", subsets["v2"].Filter)
		require.True(t, subsets["v2"].OnlyPassing)
		require.False(t, subsets["v1"].OnlyPassing)
	})

	t.Run("targets have distinct SNIs", func(t *testing.T) {
		seen := map[string]bool{}
		for _, id := range resolverNodeIDs(chain) {
			target := chain.Targets[chain.Nodes[id].Resolver.Target]
			require.NotEmpty(t, target.SNI)
			require.False(t, seen[target.SNI], "duplicate SNI %s", target.SNI)
			seen[target.SNI] = true
		}
	})
}

// TestDiscoveryChain_ResolverBehaviors covers redirects, failover, and
// subset-based default routing in a resolver.
func TestDiscoveryChain_ResolverBehaviors(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	c := e.server

	t.Run("redirect moves traffic to another service", func(t *testing.T) {
		setEntry(t, c, &api.ServiceResolverConfigEntry{
			Kind:     api.ServiceResolver,
			Name:     "legacy",
			Redirect: &api.ServiceResolverRedirect{Service: "modern"},
		})

		chain := compileChain(t, c, "legacy", nil)
		start := node(t, chain, chain.StartNode)
		require.Equal(t, api.DiscoveryGraphNodeTypeResolver, start.Type)
		require.Equal(t, "modern", chain.Targets[start.Resolver.Target].Service)
		require.Len(t, chain.Targets, 1)
	})

	t.Run("failover adds ordered fallback targets", func(t *testing.T) {
		setEntry(t, c, &api.ServiceResolverConfigEntry{
			Kind: api.ServiceResolver,
			Name: "resilient",
			Failover: map[string]api.ServiceResolverFailover{
				"*": {Service: "resilient-backup"},
			},
			ConnectTimeout: 7 * time.Second,
		})

		chain := compileChain(t, c, "resilient", nil)
		start := node(t, chain, chain.StartNode)
		require.Equal(t, 7*time.Second, start.Resolver.ConnectTimeout)
		require.NotNil(t, start.Resolver.Failover)
		require.Len(t, start.Resolver.Failover.Targets, 1)

		primary := chain.Targets[start.Resolver.Target]
		backup := chain.Targets[start.Resolver.Failover.Targets[0]]
		require.Equal(t, "resilient", primary.Service)
		require.Equal(t, "resilient-backup", backup.Service)
		require.Len(t, chain.Targets, 2)
	})

	t.Run("load balancer policy is carried on the resolver node", func(t *testing.T) {
		setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "balanced", Protocol: "http"})
		setEntry(t, c, &api.ServiceResolverConfigEntry{
			Kind: api.ServiceResolver,
			Name: "balanced",
			LoadBalancer: &api.LoadBalancer{
				Policy:         "ring_hash",
				RingHashConfig: &api.RingHashConfig{MinimumRingSize: 1024, MaximumRingSize: 4096},
				HashPolicies: []api.HashPolicy{
					{Field: "header", FieldValue: "x-user-id"},
					{SourceIP: true},
				},
			},
		})

		chain := compileChain(t, c, "balanced", nil)
		lb := node(t, chain, chain.StartNode).LoadBalancer
		require.NotNil(t, lb)
		require.Equal(t, "ring_hash", lb.Policy)
		require.EqualValues(t, 1024, lb.RingHashConfig.MinimumRingSize)
		require.EqualValues(t, 4096, lb.RingHashConfig.MaximumRingSize)
		require.Len(t, lb.HashPolicies, 2)
		require.Equal(t, "header", lb.HashPolicies[0].Field)
		require.Equal(t, "x-user-id", lb.HashPolicies[0].FieldValue)
		require.True(t, lb.HashPolicies[1].SourceIP)
	})

	t.Run("least_request policy", func(t *testing.T) {
		setEntry(t, c, &api.ServiceResolverConfigEntry{
			Kind: api.ServiceResolver,
			Name: "lightest",
			LoadBalancer: &api.LoadBalancer{
				Policy:             "least_request",
				LeastRequestConfig: &api.LeastRequestConfig{ChoiceCount: 5},
			},
		})

		chain := compileChain(t, c, "lightest", nil)
		lb := node(t, chain, chain.StartNode).LoadBalancer
		require.NotNil(t, lb)
		require.Equal(t, "least_request", lb.Policy)
		require.EqualValues(t, 5, lb.LeastRequestConfig.ChoiceCount)
	})

	t.Run("invalid load balancer policy is rejected", func(t *testing.T) {
		_, _, err := c.ConfigEntries().Set(&api.ServiceResolverConfigEntry{
			Kind:         api.ServiceResolver,
			Name:         "bad-lb",
			LoadBalancer: &api.LoadBalancer{Policy: "telepathy"},
		}, nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "policy"), err.Error())
	})
}

// TestDiscoveryChain_QueryOverrides verifies the per-request overrides that
// proxies use to evaluate a chain with local settings.
func TestDiscoveryChain_QueryOverrides(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	c := e.server

	setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "tunable", Protocol: "tcp"})

	chain := compileChain(t, c, "tunable", &api.DiscoveryChainOptions{
		OverrideProtocol:       "http",
		OverrideConnectTimeout: 11 * time.Second,
		OverrideMeshGateway:    api.MeshGatewayConfig{Mode: api.MeshGatewayModeLocal},
	})

	require.Equal(t, "http", chain.Protocol)
	start := node(t, chain, chain.StartNode)
	require.Equal(t, 11*time.Second, start.Resolver.ConnectTimeout)
	target := chain.Targets[start.Resolver.Target]
	require.Equal(t, api.MeshGatewayModeLocal, target.MeshGateway.Mode)

	// The stored configuration is unchanged by overrides.
	require.Equal(t, "tcp", compileChain(t, c, "tunable", nil).Protocol)
}
