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
		"host_rewrite with newline is ignored": {
			config: map[string]interface{}{"protocol": "http", "host_rewrite": testRewrittenHost + "\n"},
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
	cases := map[string]struct {
		hostRewrite string
		wantHost    string
	}{
		"host_rewrite set": {
			hostRewrite: testRewrittenHost,
			wantHost:    testRewrittenHost,
		},
		"host_rewrite with newline is ignored": {
			hostRewrite: testRewrittenHost + "\r\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			snap := proxycfg.TestConfigSnapshotDiscoveryChain(t, "chain-and-router", false,
				setDBUpstreamConfig(map[string]interface{}{"host_rewrite": tc.hostRewrite}), nil)

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
				require.Equal(t, tc.wantHost, r.GetRoute().GetHostRewriteLiteral(), "route %s", r.Match)
			}
		})
	}
}

func TestUpstreamHostRewrite_PeeredExplicitUpstream(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotPeeringWithHTTP2(t, func(ns *structs.NodeService) {
		ns.Proxy.Upstreams[0].Config = map[string]interface{}{"host_rewrite": testRewrittenHost}
	})

	l := findListener(t, listenersForTest(t, snap), "payments?peer=cloud:127.0.0.1:9090")
	route := inlineRoute(t, l.FilterChains[0])
	require.Equal(t, testRewrittenHost, route.GetRoute().GetHostRewriteLiteral())
}

func TestUpstreamHostRewrite_PeeredTransparentUpstream(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotPeeringTProxy(t)

	// Make the implicit peered "db" upstream speak http and give it a host_rewrite.
	dbUID := proxycfg.NewUpstreamIDFromPeeredServiceName(structs.PeeredServiceName{
		ServiceName: structs.NewServiceName("db", nil),
		Peer:        "peer-a",
	})
	endpoints, ok := snap.ConnectProxy.PeerUpstreamEndpoints.Get(dbUID)
	require.True(t, ok)
	for _, ep := range endpoints {
		ep.Service.Connect.PeerMeta.Protocol = "http"
	}
	snap.ConnectProxy.UpstreamConfig[dbUID] = &structs.Upstream{
		DestinationName: "db",
		DestinationPeer: "peer-a",
		Config:          map[string]interface{}{"host_rewrite": testRewrittenHost},
	}

	l := findListener(t, listenersForTest(t, snap), "outbound_listener:127.0.0.1:15001")
	var dbChain *envoy_listener_v3.FilterChain
	for _, fc := range l.FilterChains {
		for _, r := range fc.GetFilterChainMatch().GetPrefixRanges() {
			if r.AddressPrefix == "240.0.0.2" {
				dbChain = fc
			}
		}
	}
	require.NotNil(t, dbChain, "filter chain for peered db upstream not found")
	route := inlineRoute(t, dbChain)
	require.Equal(t, testRewrittenHost, route.GetRoute().GetHostRewriteLiteral())
}

func TestUpstreamHostRewrite_PreparedQueryUpstream(t *testing.T) {
	snap := proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
		for i := range ns.Proxy.Upstreams {
			if ns.Proxy.Upstreams[i].DestinationType == structs.UpstreamDestTypePreparedQuery {
				ns.Proxy.Upstreams[i].Config = map[string]interface{}{"protocol": "http", "host_rewrite": testRewrittenHost}
			}
		}
	}, nil)

	l := findListener(t, listenersForTest(t, snap), "prepared_query:geo-cache:127.10.10.10:8181")
	route := inlineRoute(t, l.FilterChains[0])
	require.Equal(t, testRewrittenHost, route.GetRoute().GetHostRewriteLiteral())
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
	return inlineRoute(t, l.FilterChains[0])
}

// inlineRoute returns the first route of the inline route config in the
// filter chain's HTTP connection manager.
func inlineRoute(t *testing.T, fc *envoy_listener_v3.FilterChain) *envoy_route_v3.Route {
	t.Helper()
	var hcmFilter *envoy_listener_v3.Filter
	for _, f := range fc.Filters {
		if f.Name == "envoy.filters.network.http_connection_manager" {
			hcmFilter = f
		}
	}
	require.NotNil(t, hcmFilter, "filter chain has no http_connection_manager")
	hcm := &envoy_http_v3.HttpConnectionManager{}
	require.NoError(t, hcmFilter.GetTypedConfig().UnmarshalTo(hcm))
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
