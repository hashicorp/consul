// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/netutil"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/sdk/testutil"
)

const testRewrittenHost = "internal.example.com"

func setDBUpstreamConfig(cfg map[string]interface{}) func(ns *structs.NodeService) {
	return func(ns *structs.NodeService) {
		for i := range ns.Proxy.Upstreams {
			if ns.Proxy.Upstreams[i].DestinationName == "db" {
				ns.Proxy.Upstreams[i].Config = cfg
			}
		}
	}
}

func TestUpstreamHostRewrite_InlineRoute(t *testing.T) {
	cases := map[string]struct {
		config   map[string]interface{}
		wantHost string
	}{
		"host_rewrite set": {
			config:   map[string]interface{}{"protocol": "http", "host_rewrite": testRewrittenHost},
			wantHost: testRewrittenHost,
		},
		"host_rewrite unset": {
			config: map[string]interface{}{"protocol": "http"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			snap := proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, setDBUpstreamConfig(tc.config), nil)

			route := dbInlineRoute(t, snap)
			require.Equal(t, tc.wantHost, route.GetRoute().GetHostRewriteLiteral())
		})
	}
}

func TestUpstreamHostRewrite_InlineRouteIgnoredForTCP(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false,
		setDBUpstreamConfig(map[string]interface{}{"protocol": "tcp", "host_rewrite": testRewrittenHost}), nil)

	listeners := listenersForTest(t, snap)

	l := findListener(t, listeners, "db:127.0.0.1:9191")
	require.Equal(t, "envoy.filters.network.tcp_proxy", l.FilterChains[0].Filters[0].Name)
}

func TestUpstreamHostRewrite_RDSRoutes(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotDiscoveryChain(t, "chain-and-router", false,
		setDBUpstreamConfig(map[string]interface{}{"host_rewrite": testRewrittenHost}), nil)

	g := NewResourceGenerator(testutil.Logger(t), nil, false)
	resources, err := g.routesFromSnapshot(snap)
	require.NoError(t, err)

	var routes []*envoy_route_v3.Route
	for _, res := range resources {
		rc := res.(*envoy_route_v3.RouteConfiguration)
		if rc.Name == "db" {
			routes = rc.VirtualHosts[0].Routes
		}
	}
	require.NotEmpty(t, routes)
	for _, r := range routes {
		require.Equal(t, testRewrittenHost, r.GetRoute().GetHostRewriteLiteral(), "route %s", r.Match)
	}
}

func TestInjectHostRewriteToRoutes_KeepsExistingRewrite(t *testing.T) {
	existing := &envoy_route_v3.Route{Action: &envoy_route_v3.Route_Route{Route: &envoy_route_v3.RouteAction{
		HostRewriteSpecifier: &envoy_route_v3.RouteAction_AutoHostRewrite{AutoHostRewrite: response.MakeBoolValue(true)},
	}}}
	plain := &envoy_route_v3.Route{Action: &envoy_route_v3.Route_Route{Route: &envoy_route_v3.RouteAction{}}}
	redirect := &envoy_route_v3.Route{Action: &envoy_route_v3.Route_Redirect{Redirect: &envoy_route_v3.RedirectAction{}}}

	injectHostRewriteToRoutes(testRewrittenHost, existing, plain, redirect)

	require.True(t, existing.GetRoute().GetAutoHostRewrite().GetValue())
	require.Equal(t, testRewrittenHost, plain.GetRoute().GetHostRewriteLiteral())
	require.Nil(t, redirect.GetRoute())
}

func listenersForTest(t *testing.T, snap *proxycfg.ConfigSnapshot) []proto.Message {
	t.Helper()
	orig := netutil.GetAgentBindAddrFunc
	netutil.GetAgentBindAddrFunc = netutil.GetMockGetAgentBindAddrFunc("0.0.0.0")
	t.Cleanup(func() { netutil.GetAgentBindAddrFunc = orig })

	g := NewResourceGenerator(testutil.Logger(t), configFetcherFunc(func() string { return "10.10.10.1" }), false)
	listeners, err := g.listenersFromSnapshot(snap)
	require.NoError(t, err)
	return listeners
}

func dbInlineRoute(t *testing.T, snap *proxycfg.ConfigSnapshot) *envoy_route_v3.Route {
	t.Helper()
	listeners := listenersForTest(t, snap)

	l := findListener(t, listeners, "db:127.0.0.1:9191")
	hcm := &envoy_http_v3.HttpConnectionManager{}
	require.NoError(t, l.FilterChains[0].Filters[0].GetTypedConfig().UnmarshalTo(hcm))
	return hcm.GetRouteConfig().VirtualHosts[0].Routes[0]
}

func findListener(t *testing.T, resources []proto.Message, name string) *envoy_listener_v3.Listener {
	t.Helper()
	for _, res := range resources {
		if l, ok := res.(*envoy_listener_v3.Listener); ok && l.Name == name {
			return l
		}
	}
	require.Failf(t, "listener not found", "%s", name)
	return nil
}
