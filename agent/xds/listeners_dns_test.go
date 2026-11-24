// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_dns_filter_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/dns_filter/v3"
	envoy_cares_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/network/dns_resolver/cares/v3"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/netutil"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func TestMakeVirtualDNSDomains(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, nil)

	domains := makeVirtualDNSDomains(snap)
	require.NotEmpty(t, domains)

	// Build a lookup of fqdn -> addresses for assertions.
	got := make(map[string][]string)
	for _, d := range domains {
		got[d.Name] = d.GetEndpoint().GetAddressList().GetAddress()
	}

	// The "google" upstream advertises the Consul-allocated virtual IP
	// (240.0.0.1). The consul-k8s "virtual" ClusterIP tag is intentionally
	// not advertised (see virtualIPsForNodes).
	require.Equal(t, []string{"240.0.0.1"}, got["google.virtual.default.ns.default.ap.dc1.dc.consul"])

	// Domains are sorted by FQDN to keep LDS output stable.
	for i := 1; i < len(domains); i++ {
		require.Less(t, domains[i-1].Name, domains[i].Name)
	}
}

func TestMakeVirtualDNSDomains_SkipsCrossPartitionConfiguredVIPs(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, nil)

	uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("cross-partition", nil))

	snap.ConnectProxy.DiscoveryChain[uid] = &structs.CompiledDiscoveryChain{
		ServiceName:      uid.Name,
		Namespace:        uid.NamespaceOrDefault(),
		Partition:        "other",
		Datacenter:       snap.Datacenter,
		AutoVirtualIPs:   []string{"240.20.20.20"},
		ManualVirtualIPs: []string{"10.20.20.20"},
	}

	domains := makeVirtualDNSDomains(snap)
	fqdn := virtualFQDNForUpstream(snap, uid, "")

	for _, d := range domains {
		require.NotEqual(t, fqdn, d.Name)
		require.NotContains(t, d.GetEndpoint().GetAddressList().GetAddress(), "240.20.20.20")
		require.NotContains(t, d.GetEndpoint().GetAddressList().GetAddress(), "10.20.20.20")
	}
}

func TestMakeVirtualDNSDomains_SkipsUnknownUpstream(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, nil)

	// A discovery chain in the proxy's own partition but which is neither an
	// explicit nor an implicit (intention-allowed) upstream. GetUpstream reports
	// skip=true for it, so the tproxy listener builds no filter chain to
	// intercept its VIP. The DNS table must skip it too, otherwise it would
	// advertise a VIP that Envoy never intercepts.
	uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("not-an-upstream", nil))

	snap.ConnectProxy.DiscoveryChain[uid] = &structs.CompiledDiscoveryChain{
		ServiceName:      uid.Name,
		Namespace:        uid.NamespaceOrDefault(),
		Partition:        snap.ProxyID.PartitionOrDefault(),
		Datacenter:       snap.Datacenter,
		AutoVirtualIPs:   []string{"240.30.30.30"},
		ManualVirtualIPs: []string{"10.30.30.30"},
	}
	// Ensure it is not treated as an implicit upstream.
	delete(snap.ConnectProxy.IntentionUpstreams, uid)

	domains := makeVirtualDNSDomains(snap)
	fqdn := virtualFQDNForUpstream(snap, uid, "")

	for _, d := range domains {
		require.NotEqual(t, fqdn, d.Name)
		require.NotContains(t, d.GetEndpoint().GetAddressList().GetAddress(), "240.30.30.30")
		require.NotContains(t, d.GetEndpoint().GetAddressList().GetAddress(), "10.30.30.30")
	}
}

func TestMakeInlineDNSListener(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, nil)
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	loopbackAddr := loopbackListenerAddress

	msg, err := s.makeInlineDNSListener(snap)
	require.NoError(t, err)
	require.NotNil(t, msg)

	l, ok := msg.(*envoy_listener_v3.Listener)
	require.True(t, ok)

	// Bound to loopback:8653 over UDP.
	sa := l.GetAddress().GetSocketAddress()
	require.NotNil(t, sa)
	require.Equal(t, loopbackAddr, sa.GetAddress())
	require.Equal(t, uint32(virtualDNSListenerPort), sa.GetPortValue())
	require.Equal(t, envoy_core_v3.SocketAddress_UDP, sa.GetProtocol())

	// UDP listener config must be present for a udp_listener filter.
	require.NotNil(t, l.GetUdpListenerConfig())

	// A single dns_filter listener filter carrying the inline table.
	require.Len(t, l.GetListenerFilters(), 1)
	lf := l.GetListenerFilters()[0]
	require.Equal(t, dnsFilterName, lf.GetName())

	var dnsCfg envoy_dns_filter_v3.DnsFilterConfig
	require.NoError(t, lf.GetTypedConfig().UnmarshalTo(&dnsCfg))
	require.Equal(t, virtualDNSStatPrefix, dnsCfg.GetStatPrefix())
	require.NotEmpty(t, dnsCfg.GetServerConfig().GetInlineDnsTable().GetVirtualDomains())
}

func TestMakeEgressDNSListener(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	loopbackAddr := loopbackListenerAddress

	// No recursors configured -> no listener.
	msg, err := s.makeEgressDNSListener(nil)
	require.NoError(t, err)
	require.Nil(t, msg)

	// "8.8.8.8" uses the default port 53; "1.1.1.1:5353" keeps its explicit port.
	msg, err = s.makeEgressDNSListener([]string{"8.8.8.8", "1.1.1.1:5353"})
	require.NoError(t, err)
	require.NotNil(t, msg)

	l, ok := msg.(*envoy_listener_v3.Listener)
	require.True(t, ok)

	// Bound to loopback:8654 over UDP.
	sa := l.GetAddress().GetSocketAddress()
	require.NotNil(t, sa)
	require.Equal(t, loopbackAddr, sa.GetAddress())
	require.Equal(t, uint32(egressDNSListenerPort), sa.GetPortValue())
	require.Equal(t, envoy_core_v3.SocketAddress_UDP, sa.GetProtocol())

	// UDP listener config must be present for a udp_listener filter.
	require.NotNil(t, l.GetUdpListenerConfig())

	// A single dns_filter listener filter carrying the c-ares client config.
	require.Len(t, l.GetListenerFilters(), 1)
	lf := l.GetListenerFilters()[0]
	require.Equal(t, dnsFilterName, lf.GetName())

	var dnsCfg envoy_dns_filter_v3.DnsFilterConfig
	require.NoError(t, lf.GetTypedConfig().UnmarshalTo(&dnsCfg))
	require.Equal(t, egressDNSStatPrefix, dnsCfg.GetStatPrefix())

	client := dnsCfg.GetClientConfig()
	require.NotNil(t, client)
	require.Equal(t, caresDNSResolverName, client.GetTypedDnsResolverConfig().GetName())

	var cares envoy_cares_v3.CaresDnsResolverConfig
	require.NoError(t, client.GetTypedDnsResolverConfig().GetTypedConfig().UnmarshalTo(&cares))
	require.Len(t, cares.GetResolvers(), 2)

	got := make(map[string]uint32)
	for _, r := range cares.GetResolvers() {
		rsa := r.GetSocketAddress()
		require.Equal(t, envoy_core_v3.SocketAddress_UDP, rsa.GetProtocol())
		got[rsa.GetAddress()] = rsa.GetPortValue()
	}
	require.Equal(t, uint32(53), got["8.8.8.8"])
	require.Equal(t, uint32(5353), got["1.1.1.1"])
}

func TestVirtualFQDNForUpstream(t *testing.T) {
	snap := &proxycfg.ConfigSnapshot{Datacenter: "dc1"}

	t.Run("empty name returns empty string", func(t *testing.T) {
		require.Equal(t, "", virtualFQDNForUpstream(snap, proxycfg.UpstreamID{}, ""))
	})

	t.Run("uses upstream datacenter when set", func(t *testing.T) {
		uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("db", nil))
		uid.Datacenter = "dc2"

		fqdns := virtualFQDNForUpstream(snap, uid, "")
		require.Equal(t, "db.virtual.default.ns.default.ap.dc2.dc.consul", fqdns)
	})

	t.Run("falls back to snapshot datacenter", func(t *testing.T) {
		uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("web", nil))

		fqdns := virtualFQDNForUpstream(snap, uid, "")
		require.Equal(t, "web.virtual.default.ns.default.ap.dc1.dc.consul", fqdns)
	})

	t.Run("no datacenter keeps expanded form with empty dc segment", func(t *testing.T) {
		uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("cache", nil))

		fqdns := virtualFQDNForUpstream(&proxycfg.ConfigSnapshot{}, uid, "")
		require.Equal(t, "", fqdns)
	})

	t.Run("named port is the leading label", func(t *testing.T) {
		uid := proxycfg.NewUpstreamIDFromServiceName(structs.NewServiceName("api", nil))

		require.Equal(t, "http.api.virtual.default.ns.default.ap.dc1.dc.consul", virtualFQDNForUpstream(snap, uid, "http"))
	})

	t.Run("peer label follows virtual", func(t *testing.T) {
		uid := proxycfg.NewUpstreamIDFromPeeredServiceName(structs.PeeredServiceName{
			ServiceName: structs.NewServiceName("db", nil),
			Peer:        "peer-a",
		})

		require.Equal(t, "db.virtual.peer-a.peer.default.ns.default.ap.dc1.dc.consul", virtualFQDNForUpstream(snap, uid, ""))
		require.Equal(t, "grpc.db.virtual.peer-a.peer.default.ns.default.ap.dc1.dc.consul", virtualFQDNForUpstream(snap, uid, "grpc"))
	})
}

// A local service and a peer-imported service with the same name (e.g. a
// sameness group with includeLocal and a peer member) must get distinct keys,
// each with a single address, instead of one merged multi-address record.
func TestMakeVirtualDNSDomains_LocalAndPeerSameName(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, nil)

	peerUID := proxycfg.NewUpstreamIDFromPeeredServiceName(structs.PeeredServiceName{
		ServiceName: structs.NewServiceName("google", nil),
		Peer:        "peer-a",
	})
	snap.ConnectProxy.PeerUpstreamEndpoints.InitWatch(peerUID, nil)
	snap.ConnectProxy.PeerUpstreamEndpoints.Set(peerUID, structs.CheckServiceNodes{{
		Node: &structs.Node{Node: "node1", Address: "127.0.0.1", PeerName: "peer-a"},
		Service: &structs.NodeService{
			ID:       "google-1",
			Service:  "google",
			PeerName: "peer-a",
			TaggedAddresses: map[string]structs.ServiceAddress{
				structs.TaggedAddressVirtualIP: {Address: "240.0.0.99"},
			},
		},
	}})

	got := make(map[string][]string)
	for _, d := range makeVirtualDNSDomains(snap) {
		got[d.Name] = d.GetEndpoint().GetAddressList().GetAddress()
	}

	require.Equal(t, []string{"240.0.0.1"}, got["google.virtual.default.ns.default.ap.dc1.dc.consul"])
	require.Equal(t, []string{"240.0.0.99"}, got["google.virtual.peer-a.peer.default.ns.default.ap.dc1.dc.consul"])
}

func TestMakeRecursorAddresses(t *testing.T) {
	t.Run("nil and empty input", func(t *testing.T) {
		require.Nil(t, makeRecursorAddresses(nil))
		require.Empty(t, makeRecursorAddresses([]string{"", ""}))
	})

	t.Run("defaults port and preserves explicit port", func(t *testing.T) {
		addrs := makeRecursorAddresses([]string{"8.8.8.8", "1.1.1.1:5353"})
		require.Len(t, addrs, 2)

		got := make(map[string]uint32)
		for _, a := range addrs {
			sa := a.GetSocketAddress()
			require.Equal(t, envoy_core_v3.SocketAddress_UDP, sa.GetProtocol())
			got[sa.GetAddress()] = sa.GetPortValue()
		}
		require.Equal(t, uint32(53), got["8.8.8.8"])
		require.Equal(t, uint32(5353), got["1.1.1.1"])
	})

	t.Run("skips non-IP entries", func(t *testing.T) {
		addrs := makeRecursorAddresses([]string{"not-an-ip", "example.com:53", "9.9.9.9"})
		require.Len(t, addrs, 1)
		require.Equal(t, "9.9.9.9", addrs[0].GetSocketAddress().GetAddress())
	})
}

func TestMakeInlineDNSListenerNoDomains(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	// A snapshot with no upstreams yields no virtual domains, so no listener.
	msg, err := s.makeInlineDNSListener(&proxycfg.ConfigSnapshot{})
	require.NoError(t, err)
	require.Nil(t, msg)
}

// TestListenersFromSnapshotConnectProxy_LocalizedDNSGating verifies that the
// inline virtual DNS listener and the egress recursor DNS listener are only
// added to the connect-proxy LDS resources when the featuregate.LocalizedDNS
// feature is enabled on the snapshot, regardless of whether there is virtual
// IP or recursor data available.
func TestListenersFromSnapshotConnectProxy_LocalizedDNSGating(t *testing.T) {
	origGetAgentBindAddrFunc := netutil.GetAgentBindAddrFunc
	netutil.GetAgentBindAddrFunc = netutil.GetMockGetAgentBindAddrFunc("0.0.0.0")
	defer func() { netutil.GetAgentBindAddrFunc = origGetAgentBindAddrFunc }()

	listenerNames := func(t *testing.T, s *ResourceGenerator, snap *proxycfg.ConfigSnapshot) []string {
		t.Helper()
		resources, err := s.listenersFromSnapshotConnectProxy(snap)
		require.NoError(t, err)

		var names []string
		for _, res := range resources {
			l, ok := res.(*envoy_listener_v3.Listener)
			require.True(t, ok)
			names = append(names, l.Name)
		}
		return names
	}

	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			snap := proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, nil)
			snap.LocalizedDNSEnabled = enabled

			s := &ResourceGenerator{
				Logger: hclog.NewNullLogger(),
				CfgFetcher: &mockCfgFetcher{
					dnsRecursors: []string{"8.8.8.8"},
				},
			}

			names := listenerNames(t, s, snap)
			if enabled {
				require.Contains(t, names, fmt.Sprintf("%s:%s:%d", virtualDNSListenerName, loopbackListenerAddress, virtualDNSListenerPort))
				require.Contains(t, names, fmt.Sprintf("%s:%s:%d", egressDNSListenerName, loopbackListenerAddress, egressDNSListenerPort))
			} else {
				for _, name := range names {
					require.NotContains(t, name, virtualDNSListenerName)
					require.NotContains(t, name, egressDNSListenerName)
				}
			}
		})
	}
}

func TestListenerNamesForDNS(t *testing.T) {
	require.Equal(t, "virtual_dns:127.0.0.1:8653", listenerNameForVirtualDNS("127.0.0.1"))
	require.Equal(t, "virtual_dns:[::1]:8653", listenerNameForVirtualDNS("::1"))
	require.Equal(t, "egress_dns:127.0.0.1:8654", listenerNameForEgressDNS("127.0.0.1"))
	require.Equal(t, "egress_dns:[::1]:8654", listenerNameForEgressDNS("::1"))
}

func TestMakeUDPAddress(t *testing.T) {
	addr := makeUDPAddress("127.0.0.1", 8653)
	sa := addr.GetSocketAddress()
	require.Equal(t, envoy_core_v3.SocketAddress_UDP, sa.GetProtocol())
	require.Equal(t, "127.0.0.1", sa.GetAddress())
	require.Equal(t, uint32(8653), sa.GetPortValue())
}
