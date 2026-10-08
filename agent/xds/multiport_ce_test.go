// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package xds

import (
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// Multiport is an enterprise-only feature. Every enterprise extension point in
// the xDS generators must be inert in CE: identity for pass-through helpers,
// no-op for the append* helpers. If any of these start doing work in CE, the
// generated Envoy config would diverge for single-port services.

func TestCE_MultiportHelpersAreInert(t *testing.T) {
	uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("web", nil))
	chain := &structs.CompiledDiscoveryChain{ServiceName: "web", Protocol: "tcp"}

	t.Run("servicePortsFromEndpoints", func(t *testing.T) {
		nodes := structs.CheckServiceNodes{
			{
				Service: &structs.NodeService{
					Service: "web",
					Ports: structs.ServicePorts{
						{Name: "http", Port: 8080, Default: true},
						{Name: "metrics", Port: 9090},
					},
				},
			},
		}
		require.Nil(t, servicePortsFromEndpoints(nodes),
			"CE must not surface named ports from endpoints")
	})

	t.Run("portBasedALPN", func(t *testing.T) {
		require.Nil(t, portBasedALPN("http"))
	})

	t.Run("destinationPortForDiscoveryChain", func(t *testing.T) {
		// Identity on the upstream's configured port; no enterprise rewriting.
		up := &structs.Upstream{DestinationName: "web", DestinationPort: "http"}
		require.Equal(t, "http", destinationPortForDiscoveryChain(nil, uid, up, chain))

		require.Equal(t, "", destinationPortForDiscoveryChain(nil, uid, nil, chain),
			"a nil upstream must yield no port qualifier")

		require.Equal(t, "", destinationPortForDiscoveryChain(
			nil, uid, &structs.Upstream{DestinationName: "web"}, chain))
	})

	t.Run("discoveryChainForPortQualifiedUpstream", func(t *testing.T) {
		up := &structs.Upstream{DestinationName: "web", DestinationPort: "http"}
		got := discoveryChainForPortQualifiedUpstream(nil, uid, up, chain)
		require.Same(t, chain, got,
			"CE must never substitute a port-qualified discovery chain")
	})

	t.Run("usesDefaultPortForConfiguredMultiportChain", func(t *testing.T) {
		require.False(t, usesDefaultPortForConfiguredMultiportChain(nil, uid, chain, false, "http"))
		require.False(t, usesDefaultPortForConfiguredMultiportChain(nil, uid, chain, true, ""))
	})

	t.Run("usesDirectPortsAlongsideConfiguredChain", func(t *testing.T) {
		require.False(t, usesDirectPortsAlongsideConfiguredChain(nil, chain, false))
	})
}

func TestCE_MultiportAppendHelpersAreNoOps(t *testing.T) {
	s := &ResourceGenerator{}
	uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("web", nil))
	chain := &structs.CompiledDiscoveryChain{ServiceName: "web", Protocol: "tcp"}
	up := &structs.Upstream{DestinationName: "web", DestinationPort: "http"}

	t.Run("clusters", func(t *testing.T) {
		in := []*envoy_cluster_v3.Cluster{{Name: "existing"}}
		out, err := s.appendEntConfiguredChainDirectPortClusters(in, uid, up, chain, nil)
		require.NoError(t, err)
		require.Equal(t, in, out)

		msgs := []proto.Message{&envoy_cluster_v3.Cluster{Name: "existing"}}
		got, err := s.appendEntPeeredMultiportClusters(msgs, nil, uid, nil)
		require.NoError(t, err)
		require.Equal(t, msgs, got)

		got, err = s.appendEntGatewayOutgoingPeeringServiceMultiportClusters(
			msgs, nil, proxycfg.PeeringServiceValue{}, structs.CheckServiceNode{})
		require.NoError(t, err)
		require.Equal(t, msgs, got)
	})

	t.Run("endpoints", func(t *testing.T) {
		msgs := []proto.Message{&envoy_cluster_v3.Cluster{Name: "existing"}}
		got, err := s.appendEntConfiguredChainDirectPortLoadAssignments(
			msgs, uid, up, chain, nil, proxycfg.GatewayKey{}, nil, nil)
		require.NoError(t, err)
		require.Equal(t, msgs, got)
	})

	t.Run("listeners", func(t *testing.T) {
		l := &envoy_listener_v3.Listener{Name: "outbound"}
		require.NoError(t, s.appendEntPeeredUpstreamMultiportFilterChains(
			l, nil, uid, "cluster", "filter", filterChainOpts{}))
		require.Empty(t, l.FilterChains, "CE must not add per-port filter chains")

		chains, err := s.appendEntPeeredMultiportFilterChains(
			nil, structs.NewServiceName("web", nil), nil, "filter", chain, false, nil, nil)
		require.NoError(t, err)
		require.Nil(t, chains)

		require.NoError(t, s.appendEntGatewayOutgoingPeeringServiceMultiportFilterChains(l, "mesh_gateway", nil))
		require.NoError(t, s.appendEntMeshGatewayMultiportFilterChains(l, "mesh_gateway", nil))
		require.Empty(t, l.FilterChains)
	})

	t.Run("routes", func(t *testing.T) {
		msgs := []proto.Message{&envoy_route_v3.RouteConfiguration{Name: "existing"}}
		got, err := s.appendEntMeshGatewayPeeredMultiportRoutes(
			msgs, nil, structs.NewServiceName("web", nil), chain, nil)
		require.NoError(t, err)
		require.Equal(t, msgs, got)
	})
}

// TestCE_SinglePortClusterNamingUnchanged asserts that cluster and endpoint
// naming for an ordinary single-port service is byte-for-byte what it was
// before multiport existed: no "<port>." qualifier is ever prepended.
func TestCE_SinglePortClusterNamingUnchanged(t *testing.T) {
	const cluster = "web.default.dc1.internal.trustdomain.consul"

	// No destination port is the single-port case: the name passes through.
	require.Equal(t, cluster, destinationPortClusterName(cluster, ""))
	require.Equal(t, "", destinationPortClusterName("", ""))
	require.Equal(t, "", destinationPortClusterName("", "http"))

	// Single-port services advertise no named ports in CE, so the per-port
	// CDS/EDS naming path in endpoints.go can never be taken.
	nodes := structs.CheckServiceNodes{
		{Service: &structs.NodeService{Service: "web", Port: 8080}},
	}
	require.Nil(t, servicePortsFromEndpoints(nodes))

	// And no port-based ALPN is negotiated.
	require.Nil(t, destinationPortALPN(""))
	require.Nil(t, portBasedALPN("http"))
}
