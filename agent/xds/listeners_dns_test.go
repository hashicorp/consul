// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_dns_filter_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/dns_filter/v3"
	envoy_cares_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/network/dns_resolver/cares/v3"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
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

	// The "google" upstream advertises both the consul-k8s "virtual" ClusterIP
	// (10.0.0.1) and the Consul-allocated virtual IP (240.0.0.1). Both are
	// collected and sorted for stable output.
	require.Equal(t, []string{"10.0.0.1", "240.0.0.1"}, got["google.virtual.consul"])

	// Domains are sorted by FQDN to keep LDS output stable.
	for i := 1; i < len(domains); i++ {
		require.Less(t, domains[i-1].Name, domains[i].Name)
	}
}

func TestMakeInlineDNSListener(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, nil)
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	msg, err := s.makeInlineDNSListener(snap)
	require.NoError(t, err)
	require.NotNil(t, msg)

	l, ok := msg.(*envoy_listener_v3.Listener)
	require.True(t, ok)

	// Bound to 127.0.0.1:8653 over UDP.
	sa := l.GetAddress().GetSocketAddress()
	require.NotNil(t, sa)
	require.Equal(t, "127.0.0.1", sa.GetAddress())
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

	// Bound to 127.0.0.1:8654 over UDP.
	sa := l.GetAddress().GetSocketAddress()
	require.NotNil(t, sa)
	require.Equal(t, "127.0.0.1", sa.GetAddress())
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
