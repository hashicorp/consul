// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"net"
	"sort"
	"strconv"
	"time"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_dns_table_v3 "github.com/envoyproxy/go-control-plane/envoy/data/dns/v3"
	envoy_dns_filter_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/dns_filter/v3"
	envoy_cares_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/network/dns_resolver/cares/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/naming"
)

const (
	// virtualDNSListenerName is the base name for the inline virtual DNS listener.
	virtualDNSListenerName = "virtual_dns"

	// virtualDNSListenerAddr is the loopback address the inline DNS listener binds to.
	virtualDNSListenerAddr = "127.0.0.1"

	// virtualDNSListenerPort is the UDP port the inline DNS listener binds to.
	// Envoy resolves virtual service FQDNs locally from the inline table.
	virtualDNSListenerPort = 8653

	// virtualDNSStatPrefix is the stat prefix used by the dns_filter.
	virtualDNSStatPrefix = "consul_virtual_dns"

	// dnsFilterName is the Envoy filter type name for the DNS filter, shared by
	// both the inline virtual DNS listener and the egress recursor DNS listener.
	dnsFilterName = "envoy.filters.udp.dns_filter"

	// virtualDNSSuffix is the DNS suffix used for virtual service addresses.
	virtualDNSSuffix = ".virtual.consul"

	// egressDNSListenerName is the base name for the egress recursor DNS listener.
	egressDNSListenerName = "egress_dns"

	// egressDNSListenerPort is the UDP port the egress DNS listener binds to.
	// Envoy forwards non-Consul queries to the configured recursors.
	egressDNSListenerPort = 8654

	// egressDNSStatPrefix is the stat prefix used by the egress dns_filter.
	egressDNSStatPrefix = "consul_egress_dns"

	// egressDNSDefaultRecursorPort is the default DNS port used when a recursor
	// address does not specify one.
	egressDNSDefaultRecursorPort = 53

	// caresDNSResolverName is the Envoy extension name for the c-ares resolver.
	caresDNSResolverName = "envoy.network.dns_resolver.cares"
)

// makeInlineDNSListener builds the inline virtual DNS UDP listener for a connect
// proxy. It contains a dns_filter configured with an inline DNS table mapping
// each upstream's virtual FQDN (e.g. "db.virtual.consul") to its virtual IP(s),
// built from the catalog data in the config snapshot.
//
// The listener is part of the LDS resources for the proxy, so it is recomputed
// and re-pushed automatically whenever the snapshot changes (e.g. when an
// upstream's virtual IP changes), allowing Envoy to hot-swap it with no restart.
//
// It returns nil (without error) when there are no virtual IPs to advertise, so
// that an empty listener is not pushed.
func (s *ResourceGenerator) makeInlineDNSListener(cfgSnap *proxycfg.ConfigSnapshot) (proto.Message, error) {
	virtualDomains := makeVirtualDNSDomains(cfgSnap)
	if len(virtualDomains) == 0 {
		return nil, nil
	}

	dnsFilterCfg := &envoy_dns_filter_v3.DnsFilterConfig{
		StatPrefix: virtualDNSStatPrefix,
		ServerConfig: &envoy_dns_filter_v3.DnsFilterConfig_ServerContextConfig{
			ConfigSource: &envoy_dns_filter_v3.DnsFilterConfig_ServerContextConfig_InlineDnsTable{
				InlineDnsTable: &envoy_dns_table_v3.DnsTable{
					VirtualDomains: virtualDomains,
				},
			},
		},
	}

	dnsFilter, err := makeEnvoyListenerFilter(dnsFilterName, dnsFilterCfg)
	if err != nil {
		return nil, err
	}

	l := &envoy_listener_v3.Listener{
		Name:             listenerNameForVirtualDNS(),
		Address:          makeUDPAddress(virtualDNSListenerAddr, virtualDNSListenerPort),
		TrafficDirection: envoy_core_v3.TrafficDirection_OUTBOUND,
		// The dns_filter is a UDP listener filter, so the listener must declare a
		// UDP listener config in addition to binding a UDP socket.
		UdpListenerConfig: &envoy_listener_v3.UdpListenerConfig{},
		ListenerFilters:   []*envoy_listener_v3.ListenerFilter{dnsFilter},
	}

	return l, nil
}

// makeVirtualDNSDomains builds the sorted list of virtual DNS domains (FQDN -> VIP)
// from the connect proxy's upstreams in the config snapshot.
//
// The set of virtual IPs collected for an upstream mirrors the logic used to build
// the transparent-proxy filter chain match for that upstream: only the virtual IP of
// the upstream service itself is used (the chain's primary target plus any auto/manual
// virtual IPs), not the addresses of other discovery-chain targets.
func makeVirtualDNSDomains(cfgSnap *proxycfg.ConfigSnapshot) []*envoy_dns_table_v3.DnsTable_DnsVirtualDomain {
	// fqdn -> set of addresses (deduplicated and sorted later for stable output).
	table := make(map[string]map[string]struct{})

	addEntry := func(fqdn, addr string) {
		if fqdn == "" || addr == "" {
			return
		}
		if net.ParseIP(addr) == nil {
			return
		}
		if _, ok := table[fqdn]; !ok {
			table[fqdn] = make(map[string]struct{})
		}
		table[fqdn][addr] = struct{}{}
	}

	// Upstreams discovered via discovery chains (explicit and transparent-proxy).
	for uid, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		fqdn := virtualFQDNsForUpstream(cfgSnap, uid)
		if fqdn == "" {
			continue
		}

		// Auto-assigned and manually-configured virtual IPs for the upstream service.
		// Only advertise these when the upstream chain is in the proxy's own partition,
		// mirroring the transparent-proxy filter-chain match logic in listeners.go. The
		// tproxy listener only intercepts these VIPs for same-partition upstreams, so
		// advertising them for other partitions would hand clients addresses that Envoy
		// won't intercept, causing traffic to bypass the proxy.
		if chain.Partition == cfgSnap.ProxyID.PartitionOrDefault() {
			for _, ip := range chain.AutoVirtualIPs {
				addEntry(fqdn, ip)
			}
			for _, ip := range chain.ManualVirtualIPs {
				addEntry(fqdn, ip)
			}
		}

		// Match only on the virtual IP for the upstream service, identified by the
		// chain's primary target, to avoid pulling in addresses of other targets.
		nodes := cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[uid][chain.ID()]
		gatewayVIPTag := structs.ServiceGatewayVirtualIPTag(chain.CompoundServiceName())
		for _, addr := range virtualIPsForNodes(cfgSnap, nodes, gatewayVIPTag) {
			addEntry(fqdn, addr)
		}
	}

	// Upstreams reached through a peer.
	cfgSnap.ConnectProxy.PeerUpstreamEndpoints.ForEachKey(func(uid proxycfg.UpstreamID) bool {
		fqdn := virtualFQDNsForUpstream(cfgSnap, uid)
		if fqdn == "" {
			return true
		}
		if nodes, ok := cfgSnap.ConnectProxy.PeerUpstreamEndpoints.Get(uid); ok {
			// Upstreams reached through a peer are never terminating gateways, so no
			// gateway VIP tag applies here.
			for _, addr := range virtualIPsForNodes(cfgSnap, nodes, "") {
				addEntry(fqdn, addr)
			}
		}
		return true
	})

	domains := make([]*envoy_dns_table_v3.DnsTable_DnsVirtualDomain, 0, len(table))
	for fqdn, addrSet := range table {
		addrs := make([]string, 0, len(addrSet))
		for addr := range addrSet {
			addrs = append(addrs, addr)
		}
		sort.Strings(addrs)

		domains = append(domains, &envoy_dns_table_v3.DnsTable_DnsVirtualDomain{
			Name: fqdn,
			Endpoint: &envoy_dns_table_v3.DnsTable_DnsEndpoint{
				EndpointConfig: &envoy_dns_table_v3.DnsTable_DnsEndpoint_AddressList{
					AddressList: &envoy_dns_table_v3.DnsTable_AddressList{
						Address: addrs,
					},
				},
			},
		})
	}

	// Stable sort by FQDN to avoid spurious LDS updates from map iteration order.
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Name < domains[j].Name
	})

	return domains
}

// virtualIPsForNodes extracts the unique virtual IP addresses advertised by the
// given service nodes, matching the same VIP tags used to build tproxy filter chains.
//
// gatewayVIPTag is the terminating-gateway-specific tagged-address key for the
// upstream (from structs.ServiceGatewayVirtualIPTag); pass "" when the upstream
// cannot be served by a terminating gateway (e.g. peer upstreams).
func virtualIPsForNodes(cfgSnap *proxycfg.ConfigSnapshot, nodes structs.CheckServiceNodes, gatewayVIPTag string) []string {
	uniqueAddrs := make(map[string]struct{})
	for _, e := range nodes {
		// Terminating gateways advertise the upstream's VIP under a gateway-specific
		// tag, mirroring the tproxy filter-chain match logic in listeners.go. Match
		// only on that tag for these endpoints, skipping the standard VIP tags.
		if e.Service.Kind == structs.ServiceKind(structs.TerminatingGateway) {
			if gatewayVIPTag != "" {
				if vip := e.Service.TaggedAddresses[gatewayVIPTag]; vip.Address != "" {
					uniqueAddrs[vip.Address] = struct{}{}
				}
			}
			continue
		}

		if vip := e.Service.TaggedAddresses[structs.TaggedAddressVirtualIP]; vip.Address != "" {
			uniqueAddrs[vip.Address] = struct{}{}
		}

		// The virtualIPTag is used by consul-k8s to store the ClusterIP for a service.
		// For services imported from a peer, the partition will be equal in all cases.
		if acl.EqualPartitions(e.Node.PartitionOrDefault(), cfgSnap.ProxyID.PartitionOrDefault()) {
			if vip := e.Service.TaggedAddresses[naming.VirtualIPTag]; vip.Address != "" {
				uniqueAddrs[vip.Address] = struct{}{}
			}
		}
	}

	addrs := make([]string, 0, len(uniqueAddrs))
	for addr := range uniqueAddrs {
		addrs = append(addrs, addr)
	}
	return addrs
}

// virtualFQDNsForUpstream returns the fully-expanded virtual DNS name for an upstream service.
//
// The returned name includes the upstream's namespace, partition and datacenter, matching:
//
//	<Service-Name>.virtual.<Namespace>.ns.<Admin-Partition>.ap.<Datacenter>.dc.consul
func virtualFQDNsForUpstream(cfgSnap *proxycfg.ConfigSnapshot, uid proxycfg.UpstreamID) string {
	if uid.Name == "" {
		return ""
	}

	dc := uid.Datacenter
	if dc == "" && cfgSnap != nil && cfgSnap.Datacenter != "" {
		dc = cfgSnap.Datacenter
	}

	expanded := uid.Name +
		".virtual." + uid.NamespaceOrDefault() +
		".ns." + uid.PartitionOrDefault() +
		".ap." + dc +
		".dc.consul"

	return expanded
}

// listenerNameForVirtualDNS returns the stable Envoy listener name for the inline DNS listener.
func listenerNameForVirtualDNS() string {
	return virtualDNSListenerName + ":" + net.JoinHostPort(virtualDNSListenerAddr, strconv.Itoa(virtualDNSListenerPort))
}

// makeEgressDNSListener builds the egress recursor DNS UDP listener for a connect
// proxy. It contains a dns_filter configured with a client context that forwards
// non-Consul queries to the configured upstream recursors via Envoy's c-ares DNS
// resolver, binding on 127.0.0.1:8654.
//
// Like the inline DNS listener, it is part of the LDS resources for the proxy, so
// it is recomputed and re-pushed automatically whenever the recursor configuration
// changes, allowing Envoy to hot-swap it with no restart.
//
// It returns nil (without error) when no recursors are configured, so that an
// empty listener is not pushed.
func (s *ResourceGenerator) makeEgressDNSListener(recursors []string) (proto.Message, error) {
	resolvers := makeRecursorAddresses(recursors)
	if len(resolvers) == 0 {
		return nil, nil
	}

	caresCfg := &envoy_cares_v3.CaresDnsResolverConfig{
		Resolvers: resolvers,
	}
	caresAny, err := anypb.New(caresCfg)
	if err != nil {
		return nil, err
	}

	dnsFilterCfg := &envoy_dns_filter_v3.DnsFilterConfig{
		StatPrefix: egressDNSStatPrefix,
		// A server config is required even for a forwarding-only listener; an empty
		// inline table means no local answers, so all queries fall through to the
		// upstream recursors configured in the client context.
		ServerConfig: &envoy_dns_filter_v3.DnsFilterConfig_ServerContextConfig{
			ConfigSource: &envoy_dns_filter_v3.DnsFilterConfig_ServerContextConfig_InlineDnsTable{
				InlineDnsTable: &envoy_dns_table_v3.DnsTable{},
			},
		},
		ClientConfig: &envoy_dns_filter_v3.DnsFilterConfig_ClientContextConfig{
			ResolverTimeout:   durationpb.New(2 * time.Second),
			MaxPendingLookups: 256,
			TypedDnsResolverConfig: &envoy_core_v3.TypedExtensionConfig{
				Name:        caresDNSResolverName,
				TypedConfig: caresAny,
			},
		},
	}

	dnsFilter, err := makeEnvoyListenerFilter(dnsFilterName, dnsFilterCfg)
	if err != nil {
		return nil, err
	}

	l := &envoy_listener_v3.Listener{
		Name:              listenerNameForEgressDNS(),
		Address:           makeUDPAddress(virtualDNSListenerAddr, egressDNSListenerPort),
		TrafficDirection:  envoy_core_v3.TrafficDirection_OUTBOUND,
		UdpListenerConfig: &envoy_listener_v3.UdpListenerConfig{},
		ListenerFilters:   []*envoy_listener_v3.ListenerFilter{dnsFilter},
	}

	return l, nil
}

// makeRecursorAddresses parses pre-resolved recursor addresses (IP or IP:port)
// into Envoy UDP socket addresses, defaulting to port 53 when none is specified.
//
// Callers are responsible for resolving hostnames to IPs before passing them in:
// Envoy's c-ares resolver requires its Resolvers to be IP socket addresses (they
// are the resolvers, so they cannot themselves be names that need resolving). The
// agent resolves recursors at the config boundary (see Agent.DNSRecursors, which
// reuses recursorAddr), matching how the built-in DNS server treats recursors.
// Any entry that is not a valid IP is skipped.
func makeRecursorAddresses(recursors []string) []*envoy_core_v3.Address {
	if len(recursors) == 0 {
		return nil
	}

	addrs := make([]*envoy_core_v3.Address, 0, len(recursors))
	for _, r := range recursors {
		if r == "" {
			continue
		}
		host, port := r, egressDNSDefaultRecursorPort
		if h, p, err := net.SplitHostPort(r); err == nil {
			if parsed, perr := strconv.Atoi(p); perr == nil {
				host, port = h, parsed
			}
		}
		if net.ParseIP(host) == nil {
			continue
		}
		addrs = append(addrs, makeUDPAddress(host, port))
	}
	return addrs
}

// listenerNameForEgressDNS returns the stable Envoy listener name for the egress DNS listener.
func listenerNameForEgressDNS() string {
	return egressDNSListenerName + ":" + net.JoinHostPort(virtualDNSListenerAddr, strconv.Itoa(egressDNSListenerPort))
}

// makeUDPAddress builds an Envoy UDP socket address.
func makeUDPAddress(ip string, port int) *envoy_core_v3.Address {
	return &envoy_core_v3.Address{
		Address: &envoy_core_v3.Address_SocketAddress{
			SocketAddress: &envoy_core_v3.SocketAddress{
				Protocol: envoy_core_v3.SocketAddress_UDP,
				Address:  ip,
				PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}
