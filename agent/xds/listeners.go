// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_grpc_http1_bridge_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_http1_bridge/v3"
	envoy_grpc_stats_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_stats/v3"
	envoy_http_router_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	envoy_extensions_filters_listener_http_inspector_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/http_inspector/v3"
	envoy_original_dst_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/original_dst/v3"
	envoy_tls_inspector_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	envoy_connection_limit_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/connection_limit/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_sni_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/sni_cluster/v3"
	envoy_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/accesslogs"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/hashicorp/consul/types"
)

const virtualIPTag = "virtual"

// listenersFromSnapshot returns the xDS API representation of the "listeners" in the snapshot.
func (s *ResourceGenerator) listenersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.listenersFromSnapshotConnectProxy(cfgSnap)
	case structs.ServiceKindTerminatingGateway,
		structs.ServiceKindMeshGateway,
		structs.ServiceKindIngressGateway,
		structs.ServiceKindAPIGateway:
		return s.listenersFromSnapshotGateway(cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// listenersFromSnapshotConnectProxy returns the "listeners" for a connect proxy service
func (s *ResourceGenerator) listenersFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	resources := make([]proto.Message, 1)
	var err error

	// Configure inbound listener.
	resources[0], err = s.makeInboundListener(cfgSnap, xdscommon.PublicListenerName)
	if err != nil {
		return nil, err
	}

	// This outboundListener is exclusively used when transparent proxy mode is active.
	// In that situation there is a single listener where we are redirecting outbound traffic,
	// and each upstream gets a filter chain attached to that listener.
	var outboundListener *envoy_listener_v3.Listener

	if cfgSnap.Proxy.Mode == structs.ProxyModeTransparent {
		port := iptables.DefaultTProxyOutboundPort
		if cfgSnap.Proxy.TransparentProxy.OutboundListenerPort != 0 {
			port = cfgSnap.Proxy.TransparentProxy.OutboundListenerPort
		}

		originalDstFilter, err := makeEnvoyListenerFilter("envoy.filters.listener.original_dst", &envoy_original_dst_v3.OriginalDst{})
		if err != nil {
			return nil, err
		}

		opts := makeListenerOpts{
			name:       xdscommon.OutboundListenerName,
			accessLogs: cfgSnap.Proxy.AccessLogs,
			addr:       "127.0.0.1",
			port:       port,
			direction:  envoy_core_v3.TrafficDirection_OUTBOUND,
			logger:     s.Logger,
		}
		outboundListener = makeListener(opts)
		outboundListener.FilterChains = make([]*envoy_listener_v3.FilterChain, 0)

		outboundListener.ListenerFilters = []*envoy_listener_v3.ListenerFilter{
			// The original_dst filter is a listener filter that recovers the original destination
			// address before the iptables redirection. This filter is needed for transparent
			// proxies because they route to upstreams using filter chains that match on the
			// destination IP address. If the filter is not present, no chain will match.
			originalDstFilter,
		}
	}

	proxyCfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}
	var tracing *envoy_http_v3.HttpConnectionManager_Tracing
	if proxyCfg.ListenerTracingJSON != "" {
		if tracing, err = makeTracingFromUserConfig(proxyCfg.ListenerTracingJSON); err != nil {
			s.Logger.Warn("failed to parse ListenerTracingJSON config", "error", err)
		}
	}

	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil {
		return nil, err
	}

	for uid, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		upstreamCfg, skip := cfgSnap.ConnectProxy.GetUpstream(uid, &cfgSnap.ProxyID.EnterpriseMeta)
		if skip {
			// Discovery chain is not associated with a known explicit or implicit upstream so it is skipped.
			continue
		}

		cfg := s.getAndModifyUpstreamConfigForListener(uid, upstreamCfg, chain)

		// If escape hatch is present, create a listener from it and move on to the next
		if cfg.EnvoyListenerJSON != "" {
			upstreamListener, err := makeListenerFromUserConfig(cfg.EnvoyListenerJSON)
			if err != nil {
				return nil, err
			}
			resources = append(resources, upstreamListener)
			continue
		}

		// RDS, Envoy's Route Discovery Service, is only used for HTTP services with a customized discovery chain.
		useRDS := chain.Protocol != "tcp" && !chain.Default

		var clusterName string
		if !useRDS {
			// When not using RDS we must generate a cluster name to attach to the filter chain.
			// With RDS, cluster names get attached to the dynamic routes instead.
			target, err := simpleChainTarget(chain)
			if err != nil {
				return nil, err
			}

			clusterName = s.getTargetClusterName(upstreamsSnapshot, chain, target.ID, false, false)
			if clusterName == "" {
				continue
			}
		}

		filterName := fmt.Sprintf("%s.%s.%s.%s", chain.ServiceName, chain.Namespace, chain.Partition, chain.Datacenter)

		// Generate the upstream listeners for when they are explicitly set with a local bind port or socket path
		if upstreamCfg != nil && upstreamCfg.HasLocalPortOrSocket() {
			filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
				accessLogs:  &cfgSnap.Proxy.AccessLogs,
				routeName:   uid.EnvoyID(),
				clusterName: clusterName,
				filterName:  filterName,
				protocol:    cfg.Protocol,
				useRDS:      useRDS,
				tracing:     tracing,
			})
			if err != nil {
				return nil, err
			}

			opts := makeListenerOpts{
				name:       uid.EnvoyID(),
				accessLogs: cfgSnap.Proxy.AccessLogs,
				direction:  envoy_core_v3.TrafficDirection_OUTBOUND,
				logger:     s.Logger,
				upstream:   upstreamCfg,
			}
			upstreamListener := makeListener(opts)
			s.injectConnectionBalanceConfig(cfg.BalanceOutboundConnections, upstreamListener)
			upstreamListener.FilterChains = []*envoy_listener_v3.FilterChain{
				filterChain,
			}
			resources = append(resources, upstreamListener)

			// Avoid creating filter chains below for upstreams that have dedicated listeners
			continue
		}

		// The rest of this loop is used exclusively for transparent proxies.
		// Below we create a filter chain per upstream, rather than a listener per upstream
		// as we do for explicit upstreams above.

		filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
			accessLogs:  &cfgSnap.Proxy.AccessLogs,
			routeName:   uid.EnvoyID(),
			clusterName: clusterName,
			filterName:  filterName,
			protocol:    cfg.Protocol,
			useRDS:      useRDS,
			tracing:     tracing,
		})
		if err != nil {
			return nil, err
		}

		endpoints := cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[uid][chain.ID()]
		uniqueAddrs := make(map[string]struct{})

		// Match on the virtual IP for the upstream service (identified by the chain's ID).
		// We do not match on all endpoints here since it would lead to load balancing across
		// all instances when any instance address is dialed.
		for _, e := range endpoints {
			if e.Service.Kind == structs.ServiceKind(structs.TerminatingGateway) {
				key := structs.ServiceGatewayVirtualIPTag(chain.CompoundServiceName())

				if vip := e.Service.TaggedAddresses[key]; vip.Address != "" {
					uniqueAddrs[vip.Address] = struct{}{}
				}

				continue
			}
			if vip := e.Service.TaggedAddresses[structs.TaggedAddressVirtualIP]; vip.Address != "" {
				uniqueAddrs[vip.Address] = struct{}{}
			}

			// The virtualIPTag is used by consul-k8s to store the ClusterIP for a service.
			// We only match on this virtual IP if the upstream is in the proxy's partition.
			// This is because the IP is not guaranteed to be unique across k8s clusters.
			if acl.EqualPartitions(e.Node.PartitionOrDefault(), cfgSnap.ProxyID.PartitionOrDefault()) {
				if vip := e.Service.TaggedAddresses[virtualIPTag]; vip.Address != "" {
					uniqueAddrs[vip.Address] = struct{}{}
				}
			}
		}
		if len(uniqueAddrs) > 2 {
			s.Logger.Debug("detected multiple virtual IPs for an upstream, all will be used to match traffic",
				"upstream", uid, "ip_count", len(uniqueAddrs))
		}

		// For every potential address we collected, create the appropriate address prefix to match on.
		// In this case we are matching on exact addresses, so the prefix is the address itself,
		// and the prefix length is based on whether it's IPv4 or IPv6.
		filterChain.FilterChainMatch = makeFilterChainMatchFromAddrs(uniqueAddrs)

		// Only attach the filter chain if there are addresses to match on
		if filterChain.FilterChainMatch != nil && len(filterChain.FilterChainMatch.PrefixRanges) > 0 {
			outboundListener.FilterChains = append(outboundListener.FilterChains, filterChain)
		}
	}
	requiresTLSInspector := false
	requiresHTTPInspector := false

	configuredPorts := make(map[int]interface{})
	err = cfgSnap.ConnectProxy.DestinationsUpstream.ForEachKeyE(func(uid proxycfg.UpstreamID) error {
		svcConfig, ok := cfgSnap.ConnectProxy.DestinationsUpstream.Get(uid)
		if !ok || svcConfig == nil {
			return nil
		}

		if structs.IsProtocolHTTPLike(svcConfig.Protocol) {
			if _, ok := configuredPorts[svcConfig.Destination.Port]; ok {
				return nil
			}
			configuredPorts[svcConfig.Destination.Port] = struct{}{}
			const name = "~http" // name used for the shared route name
			routeName := clusterNameForDestination(cfgSnap, name, fmt.Sprintf("%d", svcConfig.Destination.Port), svcConfig.NamespaceOrDefault(), svcConfig.PartitionOrDefault())
			filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
				accessLogs: &cfgSnap.Proxy.AccessLogs,
				routeName:  routeName,
				filterName: routeName,
				protocol:   svcConfig.Protocol,
				useRDS:     true,
				tracing:    tracing,
			})
			if err != nil {
				return err
			}
			filterChain.FilterChainMatch = makeFilterChainMatchFromAddressWithPort("", svcConfig.Destination.Port)
			outboundListener.FilterChains = append(outboundListener.FilterChains, filterChain)
			requiresHTTPInspector = true
		} else {
			for _, address := range svcConfig.Destination.Addresses {
				clusterName := clusterNameForDestination(cfgSnap, uid.Name, address, uid.NamespaceOrDefault(), uid.PartitionOrDefault())

				filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
					accessLogs:  &cfgSnap.Proxy.AccessLogs,
					routeName:   uid.EnvoyID(),
					clusterName: clusterName,
					filterName:  clusterName,
					protocol:    svcConfig.Protocol,
					tracing:     tracing,
				})
				if err != nil {
					return err
				}

				filterChain.FilterChainMatch = makeFilterChainMatchFromAddressWithPort(address, svcConfig.Destination.Port)
				outboundListener.FilterChains = append(outboundListener.FilterChains, filterChain)

				requiresTLSInspector = len(filterChain.FilterChainMatch.ServerNames) != 0 || requiresTLSInspector
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if requiresTLSInspector {
		tlsInspector, err := makeTLSInspectorListenerFilter()
		if err != nil {
			return nil, err
		}
		outboundListener.ListenerFilters = append(outboundListener.ListenerFilters, tlsInspector)
	}

	if requiresHTTPInspector {
		httpInspector, err := makeHTTPInspectorListenerFilter()
		if err != nil {
			return nil, err
		}
		outboundListener.ListenerFilters = append(outboundListener.ListenerFilters, httpInspector)
	}

	// Looping over explicit and implicit upstreams is only needed for cross-peer
	// because they do not have discovery chains.
	for _, uid := range cfgSnap.ConnectProxy.PeeredUpstreamIDs() {
		upstreamCfg, skip := cfgSnap.ConnectProxy.GetUpstream(uid, &cfgSnap.ProxyID.EnterpriseMeta)
		if skip {
			// Not associated with a known explicit or implicit upstream so it is skipped.
			continue
		}

		peerMeta, found := cfgSnap.ConnectProxy.UpstreamPeerMeta(uid)
		if !found {
			s.Logger.Warn("failed to fetch upstream peering metadata for listener", "uid", uid)
		}
		cfg := s.getAndModifyUpstreamConfigForPeeredListener(uid, upstreamCfg, peerMeta)

		// If escape hatch is present, create a listener from it and move on to the next
		if cfg.EnvoyListenerJSON != "" {
			upstreamListener, err := makeListenerFromUserConfig(cfg.EnvoyListenerJSON)
			if err != nil {
				s.Logger.Error("failed to parse envoy_listener_json",
					"upstream", uid,
					"error", err)
				continue
			}
			resources = append(resources, upstreamListener)
			continue
		}

		tbs, ok := cfgSnap.ConnectProxy.UpstreamPeerTrustBundles.Get(uid.Peer)
		if !ok {
			// this should never happen since we loop through upstreams with
			// set trust bundles
			return nil, fmt.Errorf("trust bundle not ready for peer %s", uid.Peer)
		}

		clusterName := generatePeeredClusterName(uid, tbs)

		// Generate the upstream listeners for when they are explicitly set with a local bind port or socket path
		if upstreamCfg != nil && upstreamCfg.HasLocalPortOrSocket() {
			filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
				accessLogs:  &cfgSnap.Proxy.AccessLogs,
				clusterName: clusterName,
				filterName: fmt.Sprintf("%s.%s.%s",
					upstreamCfg.DestinationName,
					upstreamCfg.DestinationNamespace,
					upstreamCfg.DestinationPeer),
				routeName:  uid.EnvoyID(),
				protocol:   cfg.Protocol,
				useRDS:     false,
				statPrefix: "upstream_peered.",
			})
			if err != nil {
				return nil, err
			}

			opts := makeListenerOpts{
				name:       uid.EnvoyID(),
				accessLogs: cfgSnap.Proxy.AccessLogs,
				direction:  envoy_core_v3.TrafficDirection_OUTBOUND,
				logger:     s.Logger,
				upstream:   upstreamCfg,
			}
			upstreamListener := makeListener(opts)
			s.injectConnectionBalanceConfig(cfg.BalanceOutboundConnections, upstreamListener)

			upstreamListener.FilterChains = []*envoy_listener_v3.FilterChain{
				filterChain,
			}
			resources = append(resources, upstreamListener)

			// Avoid creating filter chains below for upstreams that have dedicated listeners
			continue
		}

		// The rest of this loop is used exclusively for transparent proxies.
		// Below we create a filter chain per upstream, rather than a listener per upstream
		// as we do for explicit upstreams above.

		filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
			accessLogs:  &cfgSnap.Proxy.AccessLogs,
			routeName:   uid.EnvoyID(),
			clusterName: clusterName,
			filterName: fmt.Sprintf("%s.%s.%s",
				uid.Name,
				uid.NamespaceOrDefault(),
				uid.Peer),
			protocol:   cfg.Protocol,
			useRDS:     false,
			statPrefix: "upstream_peered.",
			tracing:    tracing,
		})
		if err != nil {
			return nil, err
		}

		endpoints, _ := cfgSnap.ConnectProxy.PeerUpstreamEndpoints.Get(uid)
		uniqueAddrs := make(map[string]struct{})

		// Match on the virtual IP for the upstream service (identified by the chain's ID).
		// We do not match on all endpoints here since it would lead to load balancing across
		// all instances when any instance address is dialed.
		for _, e := range endpoints {
			if vip := e.Service.TaggedAddresses[structs.TaggedAddressVirtualIP]; vip.Address != "" {
				uniqueAddrs[vip.Address] = struct{}{}
			}

			// The virtualIPTag is used by consul-k8s to store the ClusterIP for a service.
			// For services imported from a peer,the partition will be equal in all cases.
			if acl.EqualPartitions(e.Node.PartitionOrDefault(), cfgSnap.ProxyID.PartitionOrDefault()) {
				if vip := e.Service.TaggedAddresses[virtualIPTag]; vip.Address != "" {
					uniqueAddrs[vip.Address] = struct{}{}
				}
			}
		}
		if len(uniqueAddrs) > 2 {
			s.Logger.Debug("detected multiple virtual IPs for an upstream, all will be used to match traffic",
				"upstream", uid, "ip_count", len(uniqueAddrs))
		}

		// For every potential address we collected, create the appropriate address prefix to match on.
		// In this case we are matching on exact addresses, so the prefix is the address itself,
		// and the prefix length is based on whether it's IPv4 or IPv6.
		filterChain.FilterChainMatch = makeFilterChainMatchFromAddrs(uniqueAddrs)

		// Only attach the filter chain if there are addresses to match on
		if filterChain.FilterChainMatch != nil && len(filterChain.FilterChainMatch.PrefixRanges) > 0 {
			outboundListener.FilterChains = append(outboundListener.FilterChains, filterChain)
		}

	}

	if outboundListener != nil {
		// Add a passthrough for every mesh endpoint that can be dialed directly,
		// as opposed to via a virtual IP.
		var passthroughChains []*envoy_listener_v3.FilterChain

		for _, targets := range cfgSnap.ConnectProxy.PassthroughUpstreams {
			for tid, addrs := range targets {
				uid := proxycfg.NewUpstreamIDFromTargetID(tid)

				sni := connect.ServiceSNI(
					uid.Name, "", uid.NamespaceOrDefault(), uid.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

				filterName := fmt.Sprintf("%s.%s.%s.%s", uid.Name, uid.NamespaceOrDefault(), uid.PartitionOrDefault(), cfgSnap.Datacenter)

				filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
					accessLogs:  &cfgSnap.Proxy.AccessLogs,
					clusterName: "passthrough~" + sni,
					filterName:  filterName,
					protocol:    "tcp",
				})
				if err != nil {
					return nil, err
				}
				filterChain.FilterChainMatch = makeFilterChainMatchFromAddrs(addrs)

				passthroughChains = append(passthroughChains, filterChain)
			}
		}

		outboundListener.FilterChains = append(outboundListener.FilterChains, passthroughChains...)

		// Filter chains are stable sorted to avoid draining if the list is provided out of order
		sort.SliceStable(outboundListener.FilterChains, func(i, j int) bool {
			si := ""
			sj := ""
			if len(outboundListener.FilterChains[i].FilterChainMatch.PrefixRanges) > 0 {
				si += outboundListener.FilterChains[i].FilterChainMatch.PrefixRanges[0].AddressPrefix +
					"/" + outboundListener.FilterChains[i].FilterChainMatch.PrefixRanges[0].PrefixLen.String() +
					":" + outboundListener.FilterChains[i].FilterChainMatch.DestinationPort.String()
			}
			if len(outboundListener.FilterChains[i].FilterChainMatch.ServerNames) > 0 {
				si += outboundListener.FilterChains[i].FilterChainMatch.ServerNames[0] +
					":" + outboundListener.FilterChains[i].FilterChainMatch.DestinationPort.String()
			} else {
				si += outboundListener.FilterChains[i].FilterChainMatch.DestinationPort.String()
			}

			if len(outboundListener.FilterChains[j].FilterChainMatch.PrefixRanges) > 0 {
				sj += outboundListener.FilterChains[j].FilterChainMatch.PrefixRanges[0].AddressPrefix +
					"/" + outboundListener.FilterChains[j].FilterChainMatch.PrefixRanges[0].PrefixLen.String() +
					":" + outboundListener.FilterChains[j].FilterChainMatch.DestinationPort.String()
			}
			if len(outboundListener.FilterChains[j].FilterChainMatch.ServerNames) > 0 {
				sj += outboundListener.FilterChains[j].FilterChainMatch.ServerNames[0] +
					":" + outboundListener.FilterChains[j].FilterChainMatch.DestinationPort.String()
			} else {
				sj += outboundListener.FilterChains[j].FilterChainMatch.DestinationPort.String()
			}

			return si < sj
		})

		// Add a catch-all filter chain that acts as a TCP proxy to destinations outside the mesh
		if meshConf := cfgSnap.MeshConfig(); meshConf == nil ||
			!meshConf.TransparentProxy.MeshDestinationsOnly {

			filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
				accessLogs:  &cfgSnap.Proxy.AccessLogs,
				clusterName: OriginalDestinationClusterName,
				filterName:  OriginalDestinationClusterName,
				protocol:    "tcp",
			})
			if err != nil {
				return nil, err
			}
			outboundListener.DefaultFilterChain = filterChain
		}

		// Only add the outbound listener if configured.
		if len(outboundListener.FilterChains) > 0 || outboundListener.DefaultFilterChain != nil {
			resources = append(resources, outboundListener)
		}
	}

	// Looping over explicit upstreams is only needed for prepared queries because they do not have discovery chains
	for uid, u := range cfgSnap.ConnectProxy.UpstreamConfig {
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			continue
		}

		cfg, err := structs.ParseUpstreamConfig(u.Config)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn("failed to parse", "upstream", uid, "error", err)
		}

		// If escape hatch is present, create a listener from it and move on to the next
		if cfg.EnvoyListenerJSON != "" {
			upstreamListener, err := makeListenerFromUserConfig(cfg.EnvoyListenerJSON)
			if err != nil {
				s.Logger.Error("failed to parse envoy_listener_json",
					"upstream", uid,
					"error", err)
				continue
			}
			resources = append(resources, upstreamListener)
			continue
		}

		opts := makeListenerOpts{
			name:       uid.EnvoyID(),
			accessLogs: cfgSnap.Proxy.AccessLogs,
			direction:  envoy_core_v3.TrafficDirection_OUTBOUND,
			logger:     s.Logger,
			upstream:   u,
		}
		upstreamListener := makeListener(opts)
		s.injectConnectionBalanceConfig(cfg.BalanceOutboundConnections, upstreamListener)

		filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
			// TODO (SNI partition) add partition for upstream SNI
			accessLogs:  &cfgSnap.Proxy.AccessLogs,
			clusterName: connect.UpstreamSNI(u, "", cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain),
			filterName:  uid.EnvoyID(),
			routeName:   uid.EnvoyID(),
			protocol:    cfg.Protocol,
			tracing:     tracing,
		})
		if err != nil {
			return nil, err
		}
		upstreamListener.FilterChains = []*envoy_listener_v3.FilterChain{
			filterChain,
		}
		resources = append(resources, upstreamListener)
	}

	cfgSnap.Proxy.Expose.Finalize()
	paths := cfgSnap.Proxy.Expose.Paths

	// Add service health checks to the list of paths to create listeners for if needed
	if cfgSnap.Proxy.Expose.Checks {
		psid := structs.NewServiceID(cfgSnap.Proxy.DestinationServiceID, &cfgSnap.ProxyID.EnterpriseMeta)
		for _, check := range cfgSnap.ConnectProxy.WatchedServiceChecks[psid] {
			p, err := parseCheckPath(check)
			if err != nil {
				s.Logger.Warn("failed to create listener for", "check", check.CheckID, "error", err)
				continue
			}
			paths = append(paths, p)
		}
	}

	// Configure additional listener for exposed check paths
	for _, path := range paths {
		clusterName := xdscommon.LocalAppClusterName
		if path.LocalPathPort != cfgSnap.Proxy.LocalServicePort {
			clusterName = makeExposeClusterName(path.LocalPathPort)
		}

		l, err := s.makeExposedCheckListener(cfgSnap, clusterName, path)
		if err != nil {
			return nil, err
		}
		resources = append(resources, l)
	}

	return resources, nil
}

func makeFilterChainMatchFromAddrs(addrs map[string]struct{}) *envoy_listener_v3.FilterChainMatch {
	ranges := make([]*envoy_core_v3.CidrRange, 0)

	for addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}

		pfxLen := uint32(32)
		if ip.To4() == nil {
			pfxLen = 128
		}
		ranges = append(ranges, &envoy_core_v3.CidrRange{
			AddressPrefix: addr,
			PrefixLen:     &wrapperspb.UInt32Value{Value: pfxLen},
		})
	}

	// The match rules are stable sorted to avoid draining if the list is provided out of order
	sort.SliceStable(ranges, func(i, j int) bool {
		return ranges[i].AddressPrefix < ranges[j].AddressPrefix
	})

	return &envoy_listener_v3.FilterChainMatch{
		PrefixRanges: ranges,
	}
}

func makeFilterChainMatchFromAddressWithPort(address string, port int) *envoy_listener_v3.FilterChainMatch {
	ranges := make([]*envoy_core_v3.CidrRange, 0)

	ip := net.ParseIP(address)
	if ip == nil {
		if address != "" {
			return &envoy_listener_v3.FilterChainMatch{
				ServerNames:     []string{address},
				DestinationPort: &wrapperspb.UInt32Value{Value: uint32(port)},
			}
		}
		return &envoy_listener_v3.FilterChainMatch{
			DestinationPort: &wrapperspb.UInt32Value{Value: uint32(port)},
		}
	}

	pfxLen := uint32(32)
	if ip.To4() == nil {
		pfxLen = 128
	}
	ranges = append(ranges, &envoy_core_v3.CidrRange{
		AddressPrefix: address,
		PrefixLen:     &wrapperspb.UInt32Value{Value: pfxLen},
	})

	return &envoy_listener_v3.FilterChainMatch{
		PrefixRanges:    ranges,
		DestinationPort: &wrapperspb.UInt32Value{Value: uint32(port)},
	}
}

func parseCheckPath(check structs.CheckType) (structs.ExposePath, error) {
	var path structs.ExposePath

	if check.HTTP != "" {
		path.Protocol = "http"

		// Get path and local port from original HTTP target
		u, err := url.Parse(check.HTTP)
		if err != nil {
			return path, fmt.Errorf("failed to parse url '%s': %v", check.HTTP, err)
		}
		path.Path = u.Path

		_, portStr, err := net.SplitHostPort(u.Host)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.HTTP, err)
		}
		path.LocalPathPort, err = strconv.Atoi(portStr)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.HTTP, err)
		}

		// Get listener port from proxied HTTP target
		u, err = url.Parse(check.ProxyHTTP)
		if err != nil {
			return path, fmt.Errorf("failed to parse url '%s': %v", check.ProxyHTTP, err)
		}

		_, portStr, err = net.SplitHostPort(u.Host)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.ProxyHTTP, err)
		}
		path.ListenerPort, err = strconv.Atoi(portStr)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.ProxyHTTP, err)
		}
	}

	if check.GRPC != "" {
		path.Path = "/grpc.health.v1.Health/Check"
		path.Protocol = "http2"

		// Get local port from original GRPC target of the form: host/service
		proxyServerAndService := strings.SplitN(check.GRPC, "/", 2)
		_, portStr, err := net.SplitHostPort(proxyServerAndService[0])
		if err != nil {
			return path, fmt.Errorf("failed to split host/port from '%s': %v", check.GRPC, err)
		}
		path.LocalPathPort, err = strconv.Atoi(portStr)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.GRPC, err)
		}

		// Get listener port from proxied GRPC target of the form: host/service
		proxyServerAndService = strings.SplitN(check.ProxyGRPC, "/", 2)
		_, portStr, err = net.SplitHostPort(proxyServerAndService[0])
		if err != nil {
			return path, fmt.Errorf("failed to split host/port from '%s': %v", check.ProxyGRPC, err)
		}
		path.ListenerPort, err = strconv.Atoi(portStr)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.ProxyGRPC, err)
		}
	}

	path.ParsedFromCheck = true

	return path, nil
}

// listenersFromSnapshotGateway returns the "listener" for a terminating-gateway or mesh-gateway service
func (s *ResourceGenerator) listenersFromSnapshotGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	cfg, err := ParseGatewayConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// We'll collect all of the desired listeners first, and deduplicate them later.
	type namedAddress struct {
		name string
		structs.ServiceAddress
	}
	addrs := make([]namedAddress, 0)

	var resources []proto.Message
	if !cfg.NoDefaultBind {
		addr := cfgSnap.Address
		if addr == "" {
			addr = "0.0.0.0"
		}

		a := structs.ServiceAddress{
			Address: addr,
			Port:    cfgSnap.Port,
		}
		addrs = append(addrs, namedAddress{name: "default", ServiceAddress: a})
	}

	if cfg.BindTaggedAddresses {
		for name, addrCfg := range cfgSnap.TaggedAddresses {
			a := structs.ServiceAddress{
				Address: addrCfg.Address,
				Port:    addrCfg.Port,
			}
			addrs = append(addrs, namedAddress{name: name, ServiceAddress: a})
		}
	}

	for name, addrCfg := range cfg.BindAddresses {
		a := structs.ServiceAddress{
			Address: addrCfg.Address,
			Port:    addrCfg.Port,
		}
		addrs = append(addrs, namedAddress{name: name, ServiceAddress: a})
	}

	// Prevent invalid configurations of binding to the same port/addr twice
	// including with the any addresses
	//
	// Sort the list and then if two items share a service address, take the
	// first one to ensure we generate one listener per address and it's
	// stable.
	sort.Slice(addrs, func(i, j int) bool {
		return addrs[i].name < addrs[j].name
	})

	// Make listeners and deduplicate on the fly.
	seen := make(map[structs.ServiceAddress]bool)
	for _, a := range addrs {
		if seen[a.ServiceAddress] {
			continue
		}
		seen[a.ServiceAddress] = true

		var l *envoy_listener_v3.Listener

		switch cfgSnap.Kind {
		case structs.ServiceKindTerminatingGateway:
			l, err = s.makeTerminatingGatewayListener(cfgSnap, a.name, a.Address, a.Port)
			if err != nil {
				return nil, err
			}
		case structs.ServiceKindAPIGateway:
			cfgSnapPreToIngress := *cfgSnap
			//// TODO Find a cleaner solution, can't currently pass unexported property types
			var err error
			cfgSnap.IngressGateway, err = cfgSnap.APIGateway.ToIngress(cfgSnap.Datacenter)
			if err != nil {
				return nil, err
			}
			listeners, err := s.makeIngressGatewayListeners(a.Address, cfgSnap)
			if err != nil {
				return nil, err
			}

			listeners, err = s.makeAPIGatewayListeners(a.Address, &cfgSnapPreToIngress)
			if err != nil {
				return nil, err
			}

			resources = append(resources, listeners...)
		case structs.ServiceKindIngressGateway:
			listeners, err := s.makeIngressGatewayListeners(a.Address, cfgSnap)
			if err != nil {
				return nil, err
			}
			resources = append(resources, listeners...)
		case structs.ServiceKindMeshGateway:
			l, err = s.makeMeshGatewayListener(a.name, a.Address, a.Port, cfgSnap)
			if err != nil {
				return nil, err
			}
		}
		if l != nil {
			resources = append(resources, l)
		}
	}
	return resources, err
}

// makeListener returns a listener with name and bind details set. Filters must
// be added before it's useful.
//
// Note on names: Envoy listeners attempt graceful transitions of connections
// when their config changes but that means they can't have their bind address
// or port changed in a running instance. Since our users might choose to change
// a bind address or port for the public or upstream listeners, we need to
// encode those into the unique name for the listener such that if the user
// changes them, we actually create a whole new listener on the new address and
// port. Envoy should take care of closing the old one once it sees it's no
// longer in the config.
type makeListenerOpts struct {
	addr       string
	accessLogs structs.AccessLogsConfig
	logger     hclog.Logger
	mode       string
	name       string
	path       string
	port       int
	direction  envoy_core_v3.TrafficDirection
	upstream   *structs.Upstream
}

func makeListener(opts makeListenerOpts) *envoy_listener_v3.Listener {
	if opts.upstream != nil && opts.upstream.LocalBindPort == 0 && opts.upstream.LocalBindSocketPath != "" {
		opts.path = opts.upstream.LocalBindSocketPath
		opts.mode = opts.upstream.LocalBindSocketMode
		return makePipeListener(opts)
	}
	if opts.upstream != nil {
		opts.port = opts.upstream.LocalBindPort
		opts.addr = opts.upstream.LocalBindAddress
		return makeListenerWithDefault(opts)
	}

	return makeListenerWithDefault(opts)
}

func makeListenerWithDefault(opts makeListenerOpts) *envoy_listener_v3.Listener {
	if opts.addr == "" {
		opts.addr = "127.0.0.1"
	}
	accessLog, err := accesslogs.MakeAccessLogs(&opts.accessLogs, true)
	if err != nil && opts.logger != nil {
		// Since access logging is non-essential for routing, warn and move on
		opts.logger.Warn("error generating access log xds", err)
	}
	return &envoy_listener_v3.Listener{
		Name:             fmt.Sprintf("%s:%s:%d", opts.name, opts.addr, opts.port),
		AccessLog:        accessLog,
		Address:          makeAddress(opts.addr, opts.port),
		TrafficDirection: opts.direction,
	}
}

func makePipeListener(opts makeListenerOpts) *envoy_listener_v3.Listener {
	// We've already validated this, so it should not fail.
	modeInt, err := strconv.ParseUint(opts.mode, 0, 32)
	if err != nil {
		modeInt = 0
	}
	accessLog, err := accesslogs.MakeAccessLogs(&opts.accessLogs, true)
	if err != nil && opts.logger != nil {
		// Since access logging is non-essential for routing, warn and move on
		opts.logger.Warn("error generating access log xds", err)
	}
	return &envoy_listener_v3.Listener{
		Name:             fmt.Sprintf("%s:%s", opts.name, opts.path),
		AccessLog:        accessLog,
		Address:          makePipeAddress(opts.path, uint32(modeInt)),
		TrafficDirection: opts.direction,
	}
}

// makeListenerFromUserConfig returns the listener config decoded from an
// arbitrary proto3 json format string or an error if it's invalid.
//
// For now we only support embedding in JSON strings because of the hcl parsing
// pain (see Background section in the comment for decode.HookWeakDecodeFromSlice).
// This may be fixed in decode.HookWeakDecodeFromSlice in the future.
//
// When we do that we can support just nesting the config directly into the
// JSON/hcl naturally but this is a stop-gap that gets us an escape hatch
// immediately. It's also probably not a bad thing to support long-term since
// any config generated by other systems will likely be in canonical protobuf
// from rather than our slight variant in JSON/hcl.
func makeListenerFromUserConfig(configJSON string) (*envoy_listener_v3.Listener, error) {
	// Type field is present so decode it as a anypb.Any
	var any anypb.Any
	if err := protojson.Unmarshal([]byte(configJSON), &any); err != nil {
		return nil, err
	}
	var l envoy_listener_v3.Listener
	if err := proto.Unmarshal(any.Value, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *ResourceGenerator) injectConnectionBalanceConfig(balanceType string, listener *envoy_listener_v3.Listener) {
	switch balanceType {
	case "":
		// Default with no balancing.
	case structs.ConnectionExactBalance:
		listener.ConnectionBalanceConfig = &envoy_listener_v3.Listener_ConnectionBalanceConfig{
			BalanceType: &envoy_listener_v3.Listener_ConnectionBalanceConfig_ExactBalance_{},
		}
	default:
		s.Logger.Warn("ignoring invalid connection balance option", "value", balanceType)
	}
}

// Ensure that the first filter in each filter chain of a public listener is
// the authz filter to prevent unauthorized access.
func (s *ResourceGenerator) injectConnectFilters(cfgSnap *proxycfg.ConfigSnapshot, listener *envoy_listener_v3.Listener) error {
	authzFilter, err := makeRBACNetworkFilter(
		cfgSnap.ConnectProxy.Intentions,
		cfgSnap.IntentionDefaultAllow,
		rbacLocalInfo{
			trustDomain: cfgSnap.Roots.TrustDomain,
			datacenter:  cfgSnap.Datacenter,
			partition:   cfgSnap.ProxyID.PartitionOrDefault(),
		},
		cfgSnap.ConnectProxy.InboundPeerTrustBundles,
	)
	if err != nil {
		return err
	}

	for idx := range listener.FilterChains {
		// Insert our authz filter before any others
		listener.FilterChains[idx].Filters =
			append([]*envoy_listener_v3.Filter{
				authzFilter,
			}, listener.FilterChains[idx].Filters...)
	}
	return nil
}

const (
	httpConnectionManagerOldName = "envoy.http_connection_manager"
	httpConnectionManagerNewName = "envoy.filters.network.http_connection_manager"
)

func extractRdsResourceNames(listener *envoy_listener_v3.Listener) ([]string, error) {
	var found []string

	for chainIdx, chain := range listener.FilterChains {
		for filterIdx, filter := range chain.Filters {
			if filter.Name != httpConnectionManagerNewName {
				continue
			}

			tc, ok := filter.ConfigType.(*envoy_listener_v3.Filter_TypedConfig)
			if !ok {
				return nil, fmt.Errorf(
					"filter chain %d has a %q filter %d with an unsupported config type: %T",
					chainIdx,
					filter.Name,
					filterIdx,
					filter.ConfigType,
				)
			}

			var hcm envoy_http_v3.HttpConnectionManager
			if err := tc.TypedConfig.UnmarshalTo(&hcm); err != nil {
				return nil, err
			}

			if hcm.RouteSpecifier == nil {
				continue
			}

			rds, ok := hcm.RouteSpecifier.(*envoy_http_v3.HttpConnectionManager_Rds)
			if !ok {
				continue
			}

			if rds.Rds == nil {
				continue
			}

			found = append(found, rds.Rds.RouteConfigName)
		}
	}

	return found, nil
}

// Locate the existing http connect manager L4 filter and inject our RBAC filter at the top.
func injectHTTPFilterOnFilterChains(
	listener *envoy_listener_v3.Listener,
	authzFilter *envoy_http_v3.HttpFilter,
) error {
	for chainIdx, chain := range listener.FilterChains {
		var (
			hcmFilter    *envoy_listener_v3.Filter
			hcmFilterIdx int
		)

		for filterIdx, filter := range chain.Filters {
			if filter.Name == httpConnectionManagerOldName ||
				filter.Name == httpConnectionManagerNewName {
				hcmFilter = filter
				hcmFilterIdx = filterIdx
				break
			}
		}
		if hcmFilter == nil {
			return fmt.Errorf(
				"filter chain %d lacks either a %q or %q filter",
				chainIdx,
				httpConnectionManagerOldName,
				httpConnectionManagerNewName,
			)
		}

		var hcm envoy_http_v3.HttpConnectionManager
		tc, ok := hcmFilter.ConfigType.(*envoy_listener_v3.Filter_TypedConfig)
		if !ok {
			return fmt.Errorf(
				"filter chain %d has a %q filter with an unsupported config type: %T",
				chainIdx,
				hcmFilter.Name,
				hcmFilter.ConfigType,
			)
		}

		if err := tc.TypedConfig.UnmarshalTo(&hcm); err != nil {
			return err
		}

		// Insert our authz filter before any others
		hcm.HttpFilters = append([]*envoy_http_v3.HttpFilter{
			authzFilter,
		}, hcm.HttpFilters...)

		// And persist the modified filter.
		newFilter, err := makeFilter(hcmFilter.Name, &hcm)
		if err != nil {
			return err
		}
		chain.Filters[hcmFilterIdx] = newFilter
	}

	return nil
}

// NOTE: This method MUST only be used for connect proxy public listeners,
// since TLS validation will be done against root certs for all peers
// that might dial this proxy.
func (s *ResourceGenerator) injectConnectTLSForPublicListener(cfgSnap *proxycfg.ConfigSnapshot, listener *envoy_listener_v3.Listener) error {
	transportSocket, err := createDownstreamTransportSocketForConnectTLS(cfgSnap, cfgSnap.PeeringTrustBundles())
	if err != nil {
		return err
	}

	for idx := range listener.FilterChains {
		listener.FilterChains[idx].TransportSocket = transportSocket
	}
	return nil
}

func getAlpnProtocols(protocol string) []string {
	var alpnProtocols []string

	switch protocol {
	case "grpc", "http2":
		alpnProtocols = append(alpnProtocols, "h2", "http/1.1")
	case "http":
		alpnProtocols = append(alpnProtocols, "http/1.1")
	}

	return alpnProtocols
}

func createDownstreamTransportSocketForConnectTLS(cfgSnap *proxycfg.ConfigSnapshot, peerBundles []*pbpeering.PeeringTrustBundle) (*envoy_core_v3.TransportSocket, error) {
	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
	case structs.ServiceKindMeshGateway:
	default:
		return nil, fmt.Errorf("cannot inject peering trust bundles for kind %q", cfgSnap.Kind)
	}

	// Determine listener protocol type from configured service protocol. Don't hard fail on a config typo,
	//The parse func returns default config if there is an error, so it's safe to continue.
	cfg, _ := ParseProxyConfig(cfgSnap.Proxy.Config)

	// Create TLS validation context for mTLS with leaf certificate and root certs.
	tlsContext := makeCommonTLSContext(
		cfgSnap.Leaf(),
		cfgSnap.RootPEMs(),
		makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSIncoming()),
	)

	if tlsContext != nil {
		// Configure alpn protocols on CommonTLSContext
		tlsContext.AlpnProtocols = getAlpnProtocols(cfg.Protocol)
	}

	// Inject peering trust bundles if this service is exported to peered clusters.
	if len(peerBundles) > 0 {
		spiffeConfig, err := makeSpiffeValidatorConfig(
			cfgSnap.Roots.TrustDomain,
			cfgSnap.RootPEMs(),
			peerBundles,
		)
		if err != nil {
			return nil, err
		}

		typ, ok := tlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContext)
		if !ok {
			return nil, fmt.Errorf("unexpected type for TLS context validation: %T", tlsContext.ValidationContextType)
		}

		// makeCommonTLSFromLead injects the local trust domain's CA root certs as the TrustedCA.
		// We nil it out here since the local roots are included in the SPIFFE validator config.
		typ.ValidationContext.TrustedCa = nil
		typ.ValidationContext.CustomValidatorConfig = &envoy_core_v3.TypedExtensionConfig{
			// The typed config name is hard-coded because it is not available as a wellknown var in the control plane lib.
			Name:        "envoy.tls.cert_validator.spiffe",
			TypedConfig: spiffeConfig,
		}
	}

	return makeDownstreamTLSTransportSocket(&envoy_tls_v3.DownstreamTlsContext{
		CommonTlsContext:         tlsContext,
		RequireClientCertificate: &wrapperspb.BoolValue{Value: true},
	})
}

// SPIFFECertValidatorConfig is used to validate certificates from trust domains other than our own.
// With cluster peering we expect peered clusters to have independent certificate authorities.
// This means that we cannot use a single set of root CA certificates to validate client certificates for mTLS,
// but rather we need to validate against different roots depending on the trust domain of the certificate presented.
func makeSpiffeValidatorConfig(trustDomain, roots string, peerBundles []*pbpeering.PeeringTrustBundle) (*anypb.Any, error) {
	// Store the trust bundle for the local trust domain.
	bundles := map[string]string{trustDomain: roots}

	// Store the trust bundle for each trust domain of the peers this proxy is exported to.
	// This allows us to validate traffic from other trust domains.
	for _, b := range peerBundles {
		var pems string
		for _, pem := range b.RootPEMs {
			pems += lib.EnsureTrailingNewline(pem)
		}
		bundles[b.TrustDomain] = pems
	}

	cfg := &envoy_tls_v3.SPIFFECertValidatorConfig{
		TrustDomains: make([]*envoy_tls_v3.SPIFFECertValidatorConfig_TrustDomain, 0, len(bundles)),
	}

	for domain, bundle := range bundles {
		cfg.TrustDomains = append(cfg.TrustDomains, &envoy_tls_v3.SPIFFECertValidatorConfig_TrustDomain{
			Name: domain,
			TrustBundle: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_InlineString{
					InlineString: bundle,
				},
			},
		})
	}

	// Sort the trust domains so that the output is stable.
	// This benefits tests but also prevents Envoy from mistakenly thinking the listener
	// changed and needs to be drained only because this ordering is different.
	sort.Slice(cfg.TrustDomains, func(i int, j int) bool {
		return cfg.TrustDomains[i].Name < cfg.TrustDomains[j].Name
	})
	return anypb.New(cfg)
}

func (s *ResourceGenerator) makeInboundListener(cfgSnap *proxycfg.ConfigSnapshot, name string) (proto.Message, error) {
	var l *envoy_listener_v3.Listener
	var err error

	cfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// This controls if we do L4 or L7 intention checks.
	useHTTPFilter := structs.IsProtocolHTTPLike(cfg.Protocol)

	// Generate and return custom public listener from config if one was provided.
	if cfg.PublicListenerJSON != "" {
		l, err = makeListenerFromUserConfig(cfg.PublicListenerJSON)
		if err != nil {
			return nil, err
		}

		// For HTTP-like services attach an RBAC http filter and do a best-effort insert
		if useHTTPFilter {
			httpAuthzFilter, err := makeRBACHTTPFilter(
				cfgSnap.ConnectProxy.Intentions,
				cfgSnap.IntentionDefaultAllow,
				rbacLocalInfo{
					trustDomain: cfgSnap.Roots.TrustDomain,
					datacenter:  cfgSnap.Datacenter,
					partition:   cfgSnap.ProxyID.PartitionOrDefault(),
				},
				cfgSnap.ConnectProxy.InboundPeerTrustBundles,
			)
			if err != nil {
				return nil, err
			}

			// Try our best to inject the HTTP RBAC filter.
			if err := injectHTTPFilterOnFilterChains(l, httpAuthzFilter); err != nil {
				s.Logger.Warn(
					"could not inject the HTTP RBAC filter to enforce intentions on user-provided "+
						"'envoy_public_listener_json' config; falling back on the RBAC network filter instead",
					"proxy", cfgSnap.ProxyID,
					"error", err,
				)

				// If we get an error inject the RBAC network filter instead.
				useHTTPFilter = false
			}
		}

		err := s.finalizePublicListenerFromConfig(l, cfgSnap, useHTTPFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to attach Consul filters and TLS context to custom public listener: %v", err)
		}
		return l, nil
	}

	// No JSON user config, use default listener address
	// Default to listening on all addresses, but override with bind address if one is set.
	addr := cfgSnap.Address
	if addr == "" {
		addr = "0.0.0.0"
	}
	if cfg.BindAddress != "" {
		addr = cfg.BindAddress
	}

	// Override with bind port if one is set, otherwise default to
	// proxy service's address
	port := cfgSnap.Port
	if cfg.BindPort != 0 {
		port = cfg.BindPort
	}

	opts := makeListenerOpts{
		name:       name,
		accessLogs: cfgSnap.Proxy.AccessLogs,
		addr:       addr,
		port:       port,
		direction:  envoy_core_v3.TrafficDirection_INBOUND,
		logger:     s.Logger,
	}
	l = makeListener(opts)
	s.injectConnectionBalanceConfig(cfg.BalanceInboundConnections, l)

	var tracing *envoy_http_v3.HttpConnectionManager_Tracing
	if cfg.ListenerTracingJSON != "" {
		if tracing, err = makeTracingFromUserConfig(cfg.ListenerTracingJSON); err != nil {
			s.Logger.Warn("failed to parse ListenerTracingJSON config", "error", err)
		}
	}

	filterOpts := listenerFilterOpts{
		protocol:         cfg.Protocol,
		filterName:       name,
		routeName:        name,
		cluster:          xdscommon.LocalAppClusterName,
		requestTimeoutMs: cfg.LocalRequestTimeoutMs,
		idleTimeoutMs:    cfg.LocalIdleTimeoutMs,
		tracing:          tracing,
		accessLogs:       &cfgSnap.Proxy.AccessLogs,
		logger:           s.Logger,
	}
	if useHTTPFilter {
		rbacFilter, err := makeRBACHTTPFilter(
			cfgSnap.ConnectProxy.Intentions,
			cfgSnap.IntentionDefaultAllow,
			rbacLocalInfo{
				trustDomain: cfgSnap.Roots.TrustDomain,
				datacenter:  cfgSnap.Datacenter,
				partition:   cfgSnap.ProxyID.PartitionOrDefault(),
			},
			cfgSnap.ConnectProxy.InboundPeerTrustBundles,
		)
		if err != nil {
			return nil, err
		}

		filterOpts.httpAuthzFilters = []*envoy_http_v3.HttpFilter{rbacFilter}

		meshConfig := cfgSnap.MeshConfig()
		includeXFCC := meshConfig == nil || meshConfig.HTTP == nil || !meshConfig.HTTP.SanitizeXForwardedClientCert
		notGRPC := cfg.Protocol != "grpc"
		if includeXFCC && notGRPC {
			filterOpts.forwardClientDetails = true
			filterOpts.forwardClientPolicy = envoy_http_v3.HttpConnectionManager_APPEND_FORWARD

			addMeta, err := parseXFCCToDynamicMetaHTTPFilter()
			if err != nil {
				return nil, err
			}
			filterOpts.httpAuthzFilters = append(filterOpts.httpAuthzFilters, addMeta)

		}
	}

	// If an inbound connect limit is set, inject a connection limit filter on each chain.
	if cfg.MaxInboundConnections > 0 {
		connectionLimitFilter, err := makeConnectionLimitFilter(cfg.MaxInboundConnections)
		if err != nil {
			return nil, err
		}
		l.FilterChains = []*envoy_listener_v3.FilterChain{
			{
				Filters: []*envoy_listener_v3.Filter{
					connectionLimitFilter,
				},
			},
		}
	}

	filter, err := makeListenerFilter(filterOpts)
	if err != nil {
		return nil, err
	}

	if len(l.FilterChains) > 0 {
		// The list of FilterChains has already been initialized
		l.FilterChains[0].Filters = append(l.FilterChains[0].Filters, filter)
	} else {
		l.FilterChains = []*envoy_listener_v3.FilterChain{
			{
				Filters: []*envoy_listener_v3.Filter{
					filter,
				},
			},
		}
	}

	err = s.finalizePublicListenerFromConfig(l, cfgSnap, useHTTPFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to attach Consul filters and TLS context to custom public listener: %v", err)
	}

	// When permissive mTLS mode is enabled, include an additional filter chain
	// that matches on the `destination_port == <service port>`. Traffic sent
	// directly to the service port is passed through to the application
	// unmodified.
	if cfgSnap.Proxy.MutualTLSMode == structs.MutualTLSModePermissive {
		chain, err := makePermissiveFilterChain(cfgSnap, filterOpts)
		if err != nil {
			return nil, fmt.Errorf("unable to add permissive mtls filter chain: %w", err)
		}
		if chain == nil {
			s.Logger.Debug("no service port defined for service in permissive mTLS mode; not adding filter chain for non-mTLS traffic")
		} else {
			l.FilterChains = append(l.FilterChains, chain)

			// With tproxy, the REDIRECT iptables target rewrites the destination ip/port
			// to the proxy ip/port (e.g. 127.0.0.1:20000) for incoming packets.
			// We need the original_dst filter to recover the original destination address.
			l.UseOriginalDst = &wrapperspb.BoolValue{Value: true}
		}
	}
	return l, err
}

func makePermissiveFilterChain(cfgSnap *proxycfg.ConfigSnapshot, opts listenerFilterOpts) (*envoy_listener_v3.FilterChain, error) {
	servicePort := cfgSnap.Proxy.LocalServicePort
	if servicePort <= 0 {
		// No service port means the service does not accept incoming traffic, so
		// the connect proxy does not need to listen for incoming non-mTLS traffic.
		return nil, nil
	}

	opts.statPrefix += "permissive_"
	filter, err := makeTCPProxyFilter(opts)
	if err != nil {
		return nil, err
	}

	chain := &envoy_listener_v3.FilterChain{
		FilterChainMatch: &envoy_listener_v3.FilterChainMatch{
			DestinationPort: &wrapperspb.UInt32Value{Value: uint32(servicePort)},
		},
		Filters: []*envoy_listener_v3.Filter{filter},
	}
	return chain, nil
}

// finalizePublicListenerFromConfig is used for best-effort injection of Consul filter-chains onto listeners.
// This include L4 authorization filters and TLS context.
func (s *ResourceGenerator) finalizePublicListenerFromConfig(l *envoy_listener_v3.Listener, cfgSnap *proxycfg.ConfigSnapshot, useHTTPFilter bool) error {
	if !useHTTPFilter {
		// Best-effort injection of L4 intentions
		if err := s.injectConnectFilters(cfgSnap, l); err != nil {
			return nil
		}
	}

	// Always apply TLS certificates
	if err := s.injectConnectTLSForPublicListener(cfgSnap, l); err != nil {
		return nil
	}

	return nil
}

func (s *ResourceGenerator) makeExposedCheckListener(cfgSnap *proxycfg.ConfigSnapshot, cluster string, path structs.ExposePath) (proto.Message, error) {
	cfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// No user config, use default listener
	addr := cfgSnap.Address

	// Override with bind address if one is set, otherwise default to 0.0.0.0
	if cfg.BindAddress != "" {
		addr = cfg.BindAddress
	} else if addr == "" {
		addr = "0.0.0.0"
	}

	// Strip any special characters from path to make a valid and hopefully unique name
	r := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	strippedPath := r.ReplaceAllString(path.Path, "")
	listenerName := fmt.Sprintf("exposed_path_%s", strippedPath)

	listenerOpts := makeListenerOpts{
		name:       listenerName,
		accessLogs: cfgSnap.Proxy.AccessLogs,
		addr:       addr,
		port:       path.ListenerPort,
		direction:  envoy_core_v3.TrafficDirection_INBOUND,
		logger:     s.Logger,
	}
	l := makeListener(listenerOpts)

	filterName := fmt.Sprintf("exposed_path_filter_%s_%d", strippedPath, path.ListenerPort)

	filterOpts := listenerFilterOpts{
		useRDS:           false,
		protocol:         path.Protocol,
		filterName:       filterName,
		routeName:        filterName,
		cluster:          cluster,
		statPrefix:       "",
		routePath:        path.Path,
		httpAuthzFilters: nil,
		accessLogs:       &cfgSnap.Proxy.AccessLogs,
		logger:           s.Logger,
		// in the exposed check listener we don't set the tracing configuration
	}
	f, err := makeListenerFilter(filterOpts)
	if err != nil {
		return nil, err
	}

	chain := &envoy_listener_v3.FilterChain{
		Filters: []*envoy_listener_v3.Filter{f},
	}

	// For registered checks restrict traffic sources to localhost and Consul's advertise addr
	if path.ParsedFromCheck {

		// For the advertise addr we use a CidrRange that only matches one address
		advertise := s.CfgFetcher.AdvertiseAddrLAN()

		// Get prefix length based on whether address is ipv4 (32 bits) or ipv6 (128 bits)
		advertiseLen := 32
		ip := net.ParseIP(advertise)
		if ip != nil && strings.Contains(advertise, ":") {
			advertiseLen = 128
		}

		ranges := make([]*envoy_core_v3.CidrRange, 0, 3)
		ranges = append(ranges,
			&envoy_core_v3.CidrRange{AddressPrefix: "127.0.0.1", PrefixLen: &wrapperspb.UInt32Value{Value: 8}},
			&envoy_core_v3.CidrRange{AddressPrefix: advertise, PrefixLen: &wrapperspb.UInt32Value{Value: uint32(advertiseLen)}},
		)

		if ok, err := kernelSupportsIPv6(); err != nil {
			return nil, err
		} else if ok {
			ranges = append(ranges,
				&envoy_core_v3.CidrRange{AddressPrefix: "::1", PrefixLen: &wrapperspb.UInt32Value{Value: 128}},
			)
		}

		chain.FilterChainMatch = &envoy_listener_v3.FilterChainMatch{
			SourcePrefixRanges: ranges,
		}
	}

	l.FilterChains = []*envoy_listener_v3.FilterChain{chain}

	return l, err
}

func (s *ResourceGenerator) makeTerminatingGatewayListener(
	cfgSnap *proxycfg.ConfigSnapshot,
	name, addr string,
	port int,
) (*envoy_listener_v3.Listener, error) {

	listenerOpts := makeListenerOpts{
		name:       name,
		accessLogs: cfgSnap.Proxy.AccessLogs,
		addr:       addr,
		port:       port,
		direction:  envoy_core_v3.TrafficDirection_INBOUND,
		logger:     s.Logger,
	}
	l := makeListener(listenerOpts)

	tlsInspector, err := makeTLSInspectorListenerFilter()
	if err != nil {
		return nil, err
	}
	l.ListenerFilters = []*envoy_listener_v3.ListenerFilter{tlsInspector}

	// Make a FilterChain for each linked service
	// Match on the cluster name,
	for _, svc := range cfgSnap.TerminatingGateway.ValidServices() {
		clusterName := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

		// Resolvers are optional.
		resolver, hasResolver := cfgSnap.TerminatingGateway.ServiceResolvers[svc]

		intentions := cfgSnap.TerminatingGateway.Intentions[svc]
		svcConfig := cfgSnap.TerminatingGateway.ServiceConfigs[svc]

		cfg, err := ParseProxyConfig(svcConfig.ProxyConfig)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn(
				"failed to parse Connect.Proxy.Config for linked service",
				"service", svc.String(),
				"error", err,
			)
		}

		opts := terminatingGatewayFilterChainOpts{
			cluster:    clusterName,
			service:    svc,
			intentions: intentions,
			protocol:   cfg.Protocol,
		}

		clusterChain, err := s.makeFilterChainTerminatingGateway(cfgSnap, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to make filter chain for cluster %q: %v", clusterName, err)
		}
		l.FilterChains = append(l.FilterChains, clusterChain)

		// if there is a service-resolver for this service then also setup subset filter chains for it
		if hasResolver {
			// generate 1 filter chain for each service subset
			for subsetName := range resolver.Subsets {
				subsetClusterName := connect.ServiceSNI(svc.Name, subsetName, svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

				opts.cluster = subsetClusterName
				subsetClusterChain, err := s.makeFilterChainTerminatingGateway(cfgSnap, opts)
				if err != nil {
					return nil, fmt.Errorf("failed to make filter chain for cluster %q: %v", subsetClusterName, err)
				}
				l.FilterChains = append(l.FilterChains, subsetClusterChain)
			}
		}
	}

	for _, svc := range cfgSnap.TerminatingGateway.ValidDestinations() {
		intentions := cfgSnap.TerminatingGateway.Intentions[svc]
		svcConfig := cfgSnap.TerminatingGateway.ServiceConfigs[svc]

		cfg, err := ParseProxyConfig(svcConfig.ProxyConfig)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn(
				"failed to parse Connect.Proxy.Config for linked destination",
				"destination", svc.String(),
				"error", err,
			)
		}

		var dest *structs.DestinationConfig
		dest = &svcConfig.Destination

		opts := terminatingGatewayFilterChainOpts{
			service:    svc,
			intentions: intentions,
			protocol:   cfg.Protocol,
			port:       dest.Port,
		}
		for _, address := range dest.Addresses {
			clusterName := clusterNameForDestination(cfgSnap, svc.Name, address, svc.NamespaceOrDefault(), svc.PartitionOrDefault())

			opts.cluster = clusterName
			opts.address = address
			clusterChain, err := s.makeFilterChainTerminatingGateway(cfgSnap, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to make filter chain for cluster %q: %v", clusterName, err)
			}
			l.FilterChains = append(l.FilterChains, clusterChain)
		}

	}

	// Before we add the fallback, sort these chains by the matched name. All
	// of these filter chains are independent, but envoy requires them to be in
	// some order. If we put them in a random order then every xDS iteration
	// envoy will force the listener to be replaced. Sorting these has no
	// effect on how they operate, but it does mean that we won't churn
	// listeners at idle.
	sort.Slice(l.FilterChains, func(i, j int) bool {
		return l.FilterChains[i].FilterChainMatch.ServerNames[0] < l.FilterChains[j].FilterChainMatch.ServerNames[0]
	})

	// This fallback catch-all filter ensures a listener will be present for health checks to pass
	// Envoy will reset these connections since known endpoints are caught by filter chain matches above
	filterOpts := listenerFilterOpts{
		accessLogs: &cfgSnap.Proxy.AccessLogs,
		cluster:    "",
		filterName: name,
		logger:     s.Logger,
		statPrefix: "terminating_gateway.",
	}
	tcpProxy, err := makeTCPProxyFilter(filterOpts)
	if err != nil {
		return nil, err
	}

	sniCluster, err := makeSNIClusterFilter()
	if err != nil {
		return nil, err
	}

	fallback := &envoy_listener_v3.FilterChain{
		Filters: []*envoy_listener_v3.Filter{
			sniCluster,
			tcpProxy,
		},
	}
	l.FilterChains = append(l.FilterChains, fallback)

	return l, nil
}

type terminatingGatewayFilterChainOpts struct {
	cluster    string
	service    structs.ServiceName
	intentions structs.SimplifiedIntentions
	protocol   string
	address    string // only valid for destination listeners
	port       int    // only valid for destination listeners
}

func (s *ResourceGenerator) makeFilterChainTerminatingGateway(cfgSnap *proxycfg.ConfigSnapshot, tgtwyOpts terminatingGatewayFilterChainOpts) (*envoy_listener_v3.FilterChain, error) {
	tlsContext := &envoy_tls_v3.DownstreamTlsContext{
		CommonTlsContext: makeCommonTLSContext(
			cfgSnap.TerminatingGateway.ServiceLeaves[tgtwyOpts.service],
			cfgSnap.RootPEMs(),
			makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSIncoming()),
		),
		RequireClientCertificate: &wrapperspb.BoolValue{Value: true},
	}
	transportSocket, err := makeDownstreamTLSTransportSocket(tlsContext)
	if err != nil {
		return nil, err
	}

	filterChain := &envoy_listener_v3.FilterChain{
		FilterChainMatch: makeSNIFilterChainMatch(tgtwyOpts.cluster),
		Filters:          make([]*envoy_listener_v3.Filter, 0, 3),
		TransportSocket:  transportSocket,
	}

	// This controls if we do L4 or L7 intention checks.
	useHTTPFilter := structs.IsProtocolHTTPLike(tgtwyOpts.protocol)

	// If this is L4, the first filter we setup is to do intention checks.
	if !useHTTPFilter {
		authFilter, err := makeRBACNetworkFilter(
			tgtwyOpts.intentions,
			cfgSnap.IntentionDefaultAllow,
			rbacLocalInfo{
				trustDomain: cfgSnap.Roots.TrustDomain,
				datacenter:  cfgSnap.Datacenter,
				partition:   cfgSnap.ProxyID.PartitionOrDefault(),
			},
			nil, // TODO(peering): verify intentions w peers don't apply to terminatingGateway
		)
		if err != nil {
			return nil, err
		}
		filterChain.Filters = append(filterChain.Filters, authFilter)
	}

	proxyCfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}
	var tracing *envoy_http_v3.HttpConnectionManager_Tracing
	if proxyCfg.ListenerTracingJSON != "" {
		if tracing, err = makeTracingFromUserConfig(proxyCfg.ListenerTracingJSON); err != nil {
			s.Logger.Warn("failed to parse ListenerTracingJSON config", "error", err)
		}
	}

	// Lastly we setup the actual proxying component. For L4 this is a straight
	// tcp proxy. For L7 this is a very hands-off HTTP proxy just to inject an
	// HTTP filter to do intention checks here instead.
	opts := listenerFilterOpts{
		protocol:   tgtwyOpts.protocol,
		filterName: fmt.Sprintf("%s.%s.%s.%s", tgtwyOpts.service.Name, tgtwyOpts.service.NamespaceOrDefault(), tgtwyOpts.service.PartitionOrDefault(), cfgSnap.Datacenter),
		routeName:  tgtwyOpts.cluster, // Set cluster name for route config since each will have its own
		cluster:    tgtwyOpts.cluster,
		statPrefix: "upstream.",
		routePath:  "",
		tracing:    tracing,
		accessLogs: &cfgSnap.Proxy.AccessLogs,
		logger:     s.Logger,
	}

	if useHTTPFilter {
		var err error
		rbacFilter, err := makeRBACHTTPFilter(
			tgtwyOpts.intentions,
			cfgSnap.IntentionDefaultAllow,
			rbacLocalInfo{
				trustDomain: cfgSnap.Roots.TrustDomain,
				datacenter:  cfgSnap.Datacenter,
				partition:   cfgSnap.ProxyID.PartitionOrDefault(),
			},
			nil, // TODO(peering): verify intentions w peers don't apply to terminatingGateway
		)
		if err != nil {
			return nil, err
		}

		opts.httpAuthzFilters = []*envoy_http_v3.HttpFilter{rbacFilter}

		opts.cluster = ""
		opts.useRDS = true

		if meshConfig := cfgSnap.MeshConfig(); meshConfig == nil || meshConfig.HTTP == nil || !meshConfig.HTTP.SanitizeXForwardedClientCert {
			opts.forwardClientDetails = true
			// This assumes that we have a client cert (mTLS) (implied by the context of this function)
			opts.forwardClientPolicy = envoy_http_v3.HttpConnectionManager_APPEND_FORWARD
		}
	}

	filter, err := makeListenerFilter(opts)
	if err != nil {
		s.Logger.Error("failed to make listener", "cluster", tgtwyOpts.cluster, "error", err)
		return nil, err
	}
	filterChain.Filters = append(filterChain.Filters, filter)

	return filterChain, nil
}

func (s *ResourceGenerator) makeMeshGatewayListener(name, addr string, port int, cfgSnap *proxycfg.ConfigSnapshot) (*envoy_listener_v3.Listener, error) {
	tlsInspector, err := makeTLSInspectorListenerFilter()
	if err != nil {
		return nil, err
	}

	sniCluster, err := makeSNIClusterFilter()
	if err != nil {
		return nil, err
	}

	// The cluster name here doesn't matter as the sni_cluster
	// filter will fill it in for us.
	filterOpts := listenerFilterOpts{
		accessLogs: &cfgSnap.Proxy.AccessLogs,
		cluster:    "",
		filterName: name,
		logger:     s.Logger,
		statPrefix: "mesh_gateway_local.",
	}
	tcpProxy, err := makeTCPProxyFilter(filterOpts)
	if err != nil {
		return nil, err
	}

	sniClusterChain := &envoy_listener_v3.FilterChain{
		Filters: []*envoy_listener_v3.Filter{
			sniCluster,
			tcpProxy,
		},
	}

	opts := makeListenerOpts{
		name:       name,
		accessLogs: cfgSnap.Proxy.AccessLogs,
		addr:       addr,
		port:       port,
		direction:  envoy_core_v3.TrafficDirection_UNSPECIFIED,
		logger:     s.Logger,
	}
	l := makeListener(opts)
	l.ListenerFilters = []*envoy_listener_v3.ListenerFilter{tlsInspector}

	for _, svc := range cfgSnap.MeshGatewayValidExportedServices() {
		peerNames := cfgSnap.MeshGateway.ExportedServicesWithPeers[svc]
		chain := cfgSnap.MeshGateway.DiscoveryChain[svc]

		filterChain, err := s.makeMeshGatewayPeerFilterChain(cfgSnap, svc, peerNames, chain)
		if err != nil {
			return nil, err
		} else if filterChain == nil {
			continue
		}

		l.FilterChains = append(l.FilterChains, filterChain)
	}

	// We need 1 Filter Chain per remote cluster
	keys := cfgSnap.MeshGateway.GatewayKeys()
	for _, key := range keys {
		if key.Matches(cfgSnap.Datacenter, cfgSnap.ProxyID.PartitionOrEmpty()) {
			continue // skip local
		}

		clusterName := connect.GatewaySNI(key.Datacenter, key.Partition, cfgSnap.Roots.TrustDomain)
		filterName := fmt.Sprintf("%s.%s", name, key.String())

		filterOpts := listenerFilterOpts{
			accessLogs: &cfgSnap.Proxy.AccessLogs,
			cluster:    clusterName,
			filterName: filterName,
			logger:     s.Logger,
			statPrefix: "mesh_gateway_remote.",
		}
		dcTCPProxy, err := makeTCPProxyFilter(filterOpts)
		if err != nil {
			return nil, err
		}

		l.FilterChains = append(l.FilterChains, &envoy_listener_v3.FilterChain{
			FilterChainMatch: &envoy_listener_v3.FilterChainMatch{
				ServerNames: []string{fmt.Sprintf("*.%s", clusterName)},
			},
			Filters: []*envoy_listener_v3.Filter{
				dcTCPProxy,
			},
		})
	}

	// --------
	// WAN Federation over mesh gateways
	// --------

	if cfgSnap.ProxyID.InDefaultPartition() &&
		cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" &&
		cfgSnap.ServerSNIFn != nil {

		for _, key := range keys {
			if key.Datacenter == cfgSnap.Datacenter {
				continue // skip local
			}
			clusterName := cfgSnap.ServerSNIFn(key.Datacenter, "")
			filterName := fmt.Sprintf("%s.%s", name, key.String())
			filterOpts := listenerFilterOpts{
				accessLogs: &cfgSnap.Proxy.AccessLogs,
				cluster:    clusterName,
				filterName: filterName,
				logger:     s.Logger,
				statPrefix: "mesh_gateway_remote.",
			}
			dcTCPProxy, err := makeTCPProxyFilter(filterOpts)
			if err != nil {
				return nil, err
			}

			l.FilterChains = append(l.FilterChains, &envoy_listener_v3.FilterChain{
				FilterChainMatch: &envoy_listener_v3.FilterChainMatch{
					ServerNames: []string{fmt.Sprintf("*.%s", clusterName)},
				},
				Filters: []*envoy_listener_v3.Filter{
					dcTCPProxy,
				},
			})
		}

		// Wildcard all flavors to each server.
		servers, _ := cfgSnap.MeshGateway.WatchedLocalServers.Get(structs.ConsulServiceName)
		for _, srv := range servers {
			clusterName := cfgSnap.ServerSNIFn(cfgSnap.Datacenter, srv.Node.Node)

			filterName := fmt.Sprintf("%s.%s", name, cfgSnap.Datacenter)
			filterOpts := listenerFilterOpts{
				accessLogs: &cfgSnap.Proxy.AccessLogs,
				cluster:    clusterName,
				filterName: filterName,
				logger:     s.Logger,
				statPrefix: "mesh_gateway_local_server.",
			}
			dcTCPProxy, err := makeTCPProxyFilter(filterOpts)
			if err != nil {
				return nil, err
			}

			l.FilterChains = append(l.FilterChains, &envoy_listener_v3.FilterChain{
				FilterChainMatch: &envoy_listener_v3.FilterChainMatch{
					ServerNames: []string{clusterName},
				},
				Filters: []*envoy_listener_v3.Filter{
					dcTCPProxy,
				},
			})
		}
	}

	// --------
	// Peering control plane
	// --------

	// Create a single filter chain for local servers to be dialed by peers.
	// When peering through gateways we load balance across the local servers. They cannot be addressed individually.
	if cfgSnap.MeshConfig().PeerThroughMeshGateways() {
		servers, _ := cfgSnap.MeshGateway.WatchedLocalServers.Get(structs.ConsulServiceName)

		// Peering control-plane traffic can only ever be handled by the local leader.
		// We avoid routing to read replicas since they will never be Raft voters.
		if haveVoters(servers) {
			clusterName := connect.PeeringServerSAN(cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
			filterName := fmt.Sprintf("%s.%s", name, cfgSnap.Datacenter)

			filterOpts := listenerFilterOpts{
				accessLogs: &cfgSnap.Proxy.AccessLogs,
				cluster:    clusterName,
				filterName: filterName,
				logger:     s.Logger,
				statPrefix: "mesh_gateway_local_peering_server.",
			}
			filter, err := makeTCPProxyFilter(filterOpts)
			if err != nil {
				return nil, err
			}

			l.FilterChains = append(l.FilterChains, &envoy_listener_v3.FilterChain{
				FilterChainMatch: &envoy_listener_v3.FilterChainMatch{
					ServerNames: []string{clusterName},
				},
				Filters: []*envoy_listener_v3.Filter{
					filter,
				},
			})
		}
	}

	// Create a filter chain per outbound peer server cluster. Listen for the SNI provided
	// as the peer's ServerName.
	var peerServerFilterChains []*envoy_listener_v3.FilterChain
	for name := range cfgSnap.MeshGateway.PeerServers {

		filterOpts := listenerFilterOpts{
			accessLogs: &cfgSnap.Proxy.AccessLogs,
			cluster:    name,
			filterName: name,
			logger:     s.Logger,
			statPrefix: "mesh_gateway_remote_peering_servers.",
		}
		dcTCPProxy, err := makeTCPProxyFilter(filterOpts)
		if err != nil {
			return nil, err
		}

		peerServerFilterChains = append(peerServerFilterChains, &envoy_listener_v3.FilterChain{
			FilterChainMatch: makeSNIFilterChainMatch(name),
			Filters: []*envoy_listener_v3.Filter{
				dcTCPProxy,
			},
		})
	}

	// Sort so the output is stable and the listener doesn't get drained
	sort.Slice(peerServerFilterChains, func(i, j int) bool {
		return peerServerFilterChains[i].FilterChainMatch.ServerNames[0] < peerServerFilterChains[j].FilterChainMatch.ServerNames[0]
	})
	l.FilterChains = append(l.FilterChains, peerServerFilterChains...)

	// This needs to get tacked on at the end as it has no
	// matching and will act as a catch all
	l.FilterChains = append(l.FilterChains, sniClusterChain)

	return l, nil
}

func (s *ResourceGenerator) makeMeshGatewayPeerFilterChain(
	cfgSnap *proxycfg.ConfigSnapshot,
	svc structs.ServiceName,
	peerNames []string,
	chain *structs.CompiledDiscoveryChain,
) (*envoy_listener_v3.FilterChain, error) {
	var (
		useHTTPFilter = structs.IsProtocolHTTPLike(chain.Protocol)
		// RDS, Envoy's Route Discovery Service, is only used for HTTP services.
		useRDS = useHTTPFilter
	)

	if useHTTPFilter && cfgSnap.MeshGateway.Leaf == nil {
		return nil, nil // ignore; not ready
	}

	var clusterName string
	if !useRDS {
		// When not using RDS we must generate a cluster name to attach to the filter chain.
		// With RDS, cluster names get attached to the dynamic routes instead.
		target, err := simpleChainTarget(chain)
		if err != nil {
			return nil, err
		}
		clusterName = meshGatewayExportedClusterNamePrefix + CustomizeClusterName(target.Name, chain)
	}

	uid := proxycfg.NewUpstreamIDFromServiceName(svc)

	filterName := fmt.Sprintf("%s.%s.%s.%s", chain.ServiceName, chain.Namespace, chain.Partition, chain.Datacenter)

	filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
		accessLogs:           &cfgSnap.Proxy.AccessLogs,
		routeName:            uid.EnvoyID(),
		clusterName:          clusterName,
		filterName:           filterName,
		protocol:             chain.Protocol,
		useRDS:               useRDS,
		statPrefix:           "mesh_gateway_local_peered.",
		forwardClientDetails: true,
		forwardClientPolicy:  envoy_http_v3.HttpConnectionManager_SANITIZE_SET,
	})
	if err != nil {
		return nil, err
	}

	var peeredServerNames []string
	for _, peerName := range peerNames {
		peeredSNI := connect.PeeredServiceSNI(
			svc.Name,
			svc.NamespaceOrDefault(),
			svc.PartitionOrDefault(),
			peerName,
			cfgSnap.Roots.TrustDomain,
		)
		peeredServerNames = append(peeredServerNames, peeredSNI)
	}
	filterChain.FilterChainMatch = &envoy_listener_v3.FilterChainMatch{
		ServerNames: peeredServerNames,
	}

	if useHTTPFilter {
		// We only terminate TLS if we're doing an L7 proxy.
		var peerBundles []*pbpeering.PeeringTrustBundle
		for _, bundle := range cfgSnap.MeshGateway.PeeringTrustBundles {
			if stringslice.Contains(peerNames, bundle.PeerName) {
				peerBundles = append(peerBundles, bundle)
			}
		}

		peeredTransportSocket, err := createDownstreamTransportSocketForConnectTLS(cfgSnap, peerBundles)
		if err != nil {
			return nil, err
		}
		filterChain.TransportSocket = peeredTransportSocket
	}

	return filterChain, nil
}

type filterChainOpts struct {
	accessLogs           *structs.AccessLogsConfig
	routeName            string
	clusterName          string
	filterName           string
	protocol             string
	useRDS               bool
	tlsContext           *envoy_tls_v3.DownstreamTlsContext
	statPrefix           string
	forwardClientDetails bool
	forwardClientPolicy  envoy_http_v3.HttpConnectionManager_ForwardClientCertDetails
	tracing              *envoy_http_v3.HttpConnectionManager_Tracing
}

func (s *ResourceGenerator) makeUpstreamFilterChain(opts filterChainOpts) (*envoy_listener_v3.FilterChain, error) {
	if opts.statPrefix == "" {
		opts.statPrefix = "upstream."
	}
	filter, err := makeListenerFilter(listenerFilterOpts{
		useRDS:               opts.useRDS,
		protocol:             opts.protocol,
		filterName:           opts.filterName,
		routeName:            opts.routeName,
		cluster:              opts.clusterName,
		statPrefix:           opts.statPrefix,
		forwardClientDetails: opts.forwardClientDetails,
		forwardClientPolicy:  opts.forwardClientPolicy,
		tracing:              opts.tracing,
		accessLogs:           opts.accessLogs,
		logger:               s.Logger,
	})
	if err != nil {
		return nil, err
	}

	transportSocket, err := makeDownstreamTLSTransportSocket(opts.tlsContext)
	if err != nil {
		return nil, err
	}
	return &envoy_listener_v3.FilterChain{
		Filters: []*envoy_listener_v3.Filter{
			filter,
		},
		TransportSocket: transportSocket,
	}, nil
}

// simpleChainTarget returns the discovery target for a chain with a single node.
// A chain can have a single target if it is for a TCP service or an HTTP service without
// multiple splits/routes/failovers.
func simpleChainTarget(chain *structs.CompiledDiscoveryChain) (*structs.DiscoveryTarget, error) {
	startNode := chain.Nodes[chain.StartNode]
	if startNode == nil {
		return nil, fmt.Errorf("missing first node in compiled discovery chain for: %s", chain.ServiceName)
	}
	if startNode.Type != structs.DiscoveryGraphNodeTypeResolver {
		return nil, fmt.Errorf("expected discovery chain with single node, found unexpected start node: %s", startNode.Type)
	}
	targetID := startNode.Resolver.Target
	return chain.Targets[targetID], nil
}

func (s *ResourceGenerator) getAndModifyUpstreamConfigForListener(
	uid proxycfg.UpstreamID,
	u *structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
) structs.UpstreamConfig {
	var (
		cfg structs.UpstreamConfig
		err error
	)

	configMap := make(map[string]interface{})
	if u != nil {
		configMap = u.Config
	}
	if chain == nil || chain.Default {
		cfg, err = structs.ParseUpstreamConfigNoDefaults(configMap)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn("failed to parse", "upstream", uid, "error", err)
		}
	} else {
		// Use NoDefaults here so that we can set the protocol to the chain
		// protocol if necessary
		cfg, err = structs.ParseUpstreamConfigNoDefaults(configMap)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn("failed to parse", "upstream", uid, "error", err)
		}

		if cfg.EnvoyListenerJSON != "" {
			s.Logger.Warn("ignoring escape hatch setting because already configured for",
				"discovery chain", chain.ServiceName, "upstream", uid, "config", "envoy_listener_json")

			// Remove from config struct so we don't use it later on
			cfg.EnvoyListenerJSON = ""
		}
	}
	protocol := cfg.Protocol
	if chain != nil {
		if protocol == "" {
			protocol = chain.Protocol
		}
		if protocol == "" {
			protocol = "tcp"
		}
	} else {
		protocol = "tcp"
	}

	// set back on the config so that we can use it from return value
	cfg.Protocol = protocol

	return cfg
}

func (s *ResourceGenerator) getAndModifyUpstreamConfigForPeeredListener(
	uid proxycfg.UpstreamID,
	u *structs.Upstream,
	peerMeta structs.PeeringServiceMeta,
) structs.UpstreamConfig {
	var (
		cfg structs.UpstreamConfig
		err error
	)

	configMap := make(map[string]interface{})
	if u != nil {
		configMap = u.Config
	}

	cfg, err = structs.ParseUpstreamConfigNoDefaults(configMap)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse", "upstream", uid, "error", err)
	}

	// Ignore the configured protocol for peer upstreams, since it is defined by the remote
	// cluster, which we cannot control.
	protocol := peerMeta.Protocol
	if protocol == "" {
		protocol = "tcp"
	}

	// set back on the config so that we can use it from return value
	cfg.Protocol = protocol

	if cfg.ConnectTimeoutMs == 0 {
		cfg.ConnectTimeoutMs = 5000
	}

	if cfg.MeshGateway.Mode == "" && u != nil {
		cfg.MeshGateway = u.MeshGateway
	}

	return cfg
}

type listenerFilterOpts struct {
	// All listener filters
	accessLogs *structs.AccessLogsConfig
	cluster    string
	filterName string
	logger     hclog.Logger
	protocol   string
	statPrefix string

	// HTTP listener filter options
	forwardClientDetails bool
	forwardClientPolicy  envoy_http_v3.HttpConnectionManager_ForwardClientCertDetails
	httpAuthzFilters     []*envoy_http_v3.HttpFilter
	idleTimeoutMs        *int
	requestTimeoutMs     *int
	routeName            string
	routePath            string
	tracing              *envoy_http_v3.HttpConnectionManager_Tracing
	useRDS               bool
}

func makeListenerFilter(opts listenerFilterOpts) (*envoy_listener_v3.Filter, error) {
	switch opts.protocol {
	case "grpc", "http2", "http":
		return makeHTTPFilter(opts)
	case "tcp":
		fallthrough
	default:
		if opts.useRDS {
			return nil, fmt.Errorf("RDS is not compatible with the tcp proxy filter")
		} else if opts.cluster == "" {
			return nil, fmt.Errorf("cluster name is required for a tcp proxy filter")
		}
		return makeTCPProxyFilter(opts)
	}
}

func makeTLSInspectorListenerFilter() (*envoy_listener_v3.ListenerFilter, error) {
	return makeEnvoyListenerFilter("envoy.filters.listener.tls_inspector", &envoy_tls_inspector_v3.TlsInspector{})
}

func makeHTTPInspectorListenerFilter() (*envoy_listener_v3.ListenerFilter, error) {
	return makeEnvoyListenerFilter("envoy.filters.listener.http_inspector", &envoy_extensions_filters_listener_http_inspector_v3.HttpInspector{})
}

func makeSNIFilterChainMatch(sniMatches ...string) *envoy_listener_v3.FilterChainMatch {
	if sniMatches == nil {
		return nil
	}
	return &envoy_listener_v3.FilterChainMatch{
		ServerNames: sniMatches,
	}
}

func makeSNIClusterFilter() (*envoy_listener_v3.Filter, error) {
	return makeFilter("envoy.filters.network.sni_cluster", &envoy_sni_cluster_v3.SniCluster{})
}

func makeTCPProxyFilter(opts listenerFilterOpts) (*envoy_listener_v3.Filter, error) {
	accessLogs, err := accesslogs.MakeAccessLogs(opts.accessLogs, false)
	if err != nil && opts.logger != nil {
		opts.logger.Warn("could not make access log xds for tcp proxy", err)
	}

	cfg := &envoy_tcp_proxy_v3.TcpProxy{
		AccessLog:        accessLogs,
		ClusterSpecifier: &envoy_tcp_proxy_v3.TcpProxy_Cluster{Cluster: opts.cluster},
		StatPrefix:       makeStatPrefix(opts.statPrefix, opts.filterName),
	}
	return makeFilter("envoy.filters.network.tcp_proxy", cfg)
}

func makeConnectionLimitFilter(limit int) (*envoy_listener_v3.Filter, error) {
	cfg := &envoy_connection_limit_v3.ConnectionLimit{
		StatPrefix:     "inbound_connection_limit",
		MaxConnections: wrapperspb.UInt64(uint64(limit)),
	}
	return makeFilter("envoy.filters.network.connection_limit", cfg)
}

func makeStatPrefix(prefix, filterName string) string {
	// Replace colons here because Envoy does that in the metrics for the actual
	// clusters but doesn't in the stat prefix here while dashboards assume they
	// will match.
	return fmt.Sprintf("%s%s", prefix, strings.Replace(filterName, ":", "_", -1))
}

func makeTracingFromUserConfig(configJSON string) (*envoy_http_v3.HttpConnectionManager_Tracing, error) {
	// Type field is present so decode it as a anypb.Any
	var any anypb.Any
	if err := protojson.Unmarshal([]byte(configJSON), &any); err != nil {
		return nil, err
	}
	var t envoy_http_v3.HttpConnectionManager_Tracing
	if err := proto.Unmarshal(any.Value, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func makeHTTPFilter(opts listenerFilterOpts) (*envoy_listener_v3.Filter, error) {
	router, err := makeEnvoyHTTPFilter("envoy.filters.http.router", &envoy_http_router_v3.Router{})
	if err != nil {
		return nil, err
	}

	accessLogs, err := accesslogs.MakeAccessLogs(opts.accessLogs, false)
	if err != nil && opts.logger != nil {
		opts.logger.Warn("could not make access log xds for http connection manager", err)
	}

	cfg := &envoy_http_v3.HttpConnectionManager{
		AccessLog:  accessLogs,
		StatPrefix: makeStatPrefix(opts.statPrefix, opts.filterName),
		CodecType:  envoy_http_v3.HttpConnectionManager_AUTO,
		HttpFilters: []*envoy_http_v3.HttpFilter{
			router,
		},
		Tracing: &envoy_http_v3.HttpConnectionManager_Tracing{
			// Don't trace any requests by default unless the client application
			// explicitly propagates trace headers that indicate this should be
			// sampled.
			RandomSampling: &envoy_type_v3.Percent{Value: 0.0},
		},
	}

	if opts.tracing != nil {
		cfg.Tracing = opts.tracing
	}

	if opts.useRDS {
		if opts.cluster != "" {
			return nil, fmt.Errorf("cannot specify cluster name when using RDS")
		}
		cfg.RouteSpecifier = &envoy_http_v3.HttpConnectionManager_Rds{
			Rds: &envoy_http_v3.Rds{
				RouteConfigName: opts.routeName,
				ConfigSource: &envoy_core_v3.ConfigSource{
					ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
					ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
						Ads: &envoy_core_v3.AggregatedConfigSource{},
					},
				},
			},
		}
	} else {
		if opts.cluster == "" {
			return nil, fmt.Errorf("must specify cluster name when not using RDS")
		}

		route := &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{
					Prefix: "/",
				},
				// TODO(banks) Envoy supports matching only valid GRPC
				// requests which might be nice to add here for gRPC services
				// but it's not supported in our current envoy SDK version
				// although docs say it was supported by 1.8.0. Going to defer
				// that until we've updated the deps.
			},
			Action: &envoy_route_v3.Route_Route{
				Route: &envoy_route_v3.RouteAction{
					ClusterSpecifier: &envoy_route_v3.RouteAction_Cluster{
						Cluster: opts.cluster,
					},
				},
			},
		}

		if opts.requestTimeoutMs != nil {
			r := route.GetRoute()
			r.Timeout = durationpb.New(time.Duration(*opts.requestTimeoutMs) * time.Millisecond)
		}

		if opts.idleTimeoutMs != nil {
			r := route.GetRoute()
			r.IdleTimeout = durationpb.New(time.Duration(*opts.idleTimeoutMs) * time.Millisecond)
		}

		// If a path is provided, do not match on a catch-all prefix
		if opts.routePath != "" {
			route.Match.PathSpecifier = &envoy_route_v3.RouteMatch_Path{Path: opts.routePath}
		}

		cfg.RouteSpecifier = &envoy_http_v3.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy_route_v3.RouteConfiguration{
				Name: opts.routeName,
				VirtualHosts: []*envoy_route_v3.VirtualHost{
					{
						Name:    opts.filterName,
						Domains: []string{"*"},
						Routes: []*envoy_route_v3.Route{
							route,
						},
					},
				},
			},
		}
	}

	if opts.protocol == "http2" || opts.protocol == "grpc" {
		cfg.Http2ProtocolOptions = &envoy_core_v3.Http2ProtocolOptions{}
	}

	// Note the default leads to setting HttpConnectionManager_SANITIZE
	if opts.forwardClientDetails {
		cfg.ForwardClientCertDetails = opts.forwardClientPolicy
		cfg.SetCurrentClientCertDetails = &envoy_http_v3.HttpConnectionManager_SetCurrentClientCertDetails{
			Subject: &wrapperspb.BoolValue{Value: true},
			Cert:    true,
			Chain:   true,
			Dns:     true,
			Uri:     true,
		}
	}

	// Like injectConnectFilters for L4, here we ensure that the first filter
	// (other than the "envoy.grpc_http1_bridge" filter) in the http filter
	// chain of a public listener is the authz filter to prevent unauthorized
	// access and that every filter chain uses our TLS certs.
	if len(opts.httpAuthzFilters) > 0 {
		cfg.HttpFilters = append(opts.httpAuthzFilters, cfg.HttpFilters...)
	}

	if opts.protocol == "grpc" {
		grpcHttp1Bridge, err := makeEnvoyHTTPFilter(
			"envoy.filters.http.grpc_http1_bridge",
			&envoy_grpc_http1_bridge_v3.Config{},
		)
		if err != nil {
			return nil, err
		}

		// In envoy 1.14.x the default value "stats_for_all_methods=true" was
		// deprecated, and was changed to "false" in 1.18.x. Avoid using the
		// default. TODO: we may want to expose this to users somehow easily.
		grpcStatsFilter, err := makeEnvoyHTTPFilter(
			"envoy.filters.http.grpc_stats",
			&envoy_grpc_stats_v3.FilterConfig{
				PerMethodStatSpecifier: &envoy_grpc_stats_v3.FilterConfig_StatsForAllMethods{
					StatsForAllMethods: makeBoolValue(true),
				},
			},
		)
		if err != nil {
			return nil, err
		}

		// Add grpc bridge before router and authz, and the stats in front of that.
		cfg.HttpFilters = append([]*envoy_http_v3.HttpFilter{
			grpcStatsFilter,
			grpcHttp1Bridge,
		}, cfg.HttpFilters...)
	}

	return makeFilter("envoy.filters.network.http_connection_manager", cfg)
}

func makeEnvoyListenerFilter(name string, cfg proto.Message) (*envoy_listener_v3.ListenerFilter, error) {
	any, err := anypb.New(cfg)
	if err != nil {
		return nil, err
	}
	return &envoy_listener_v3.ListenerFilter{
		Name:       name,
		ConfigType: &envoy_listener_v3.ListenerFilter_TypedConfig{TypedConfig: any},
	}, nil
}

func makeFilter(name string, cfg proto.Message) (*envoy_listener_v3.Filter, error) {
	any, err := anypb.New(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_listener_v3.Filter{
		Name:       name,
		ConfigType: &envoy_listener_v3.Filter_TypedConfig{TypedConfig: any},
	}, nil
}

func makeEnvoyHTTPFilter(name string, cfg proto.Message) (*envoy_http_v3.HttpFilter, error) {
	any, err := anypb.New(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_http_v3.HttpFilter{
		Name:       name,
		ConfigType: &envoy_http_v3.HttpFilter_TypedConfig{TypedConfig: any},
	}, nil
}

func makeCommonTLSContext(
	leaf *structs.IssuedCert,
	rootPEMs string,
	tlsParams *envoy_tls_v3.TlsParameters,
) *envoy_tls_v3.CommonTlsContext {
	if rootPEMs == "" {
		return nil
	}
	if tlsParams == nil {
		tlsParams = &envoy_tls_v3.TlsParameters{}
	}

	return &envoy_tls_v3.CommonTlsContext{
		TlsParams: tlsParams,
		TlsCertificates: []*envoy_tls_v3.TlsCertificate{
			{
				CertificateChain: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: lib.EnsureTrailingNewline(leaf.CertPEM),
					},
				},
				PrivateKey: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: lib.EnsureTrailingNewline(leaf.PrivateKeyPEM),
					},
				},
			},
		},
		ValidationContextType: &envoy_tls_v3.CommonTlsContext_ValidationContext{
			ValidationContext: &envoy_tls_v3.CertificateValidationContext{
				// TODO(banks): later for L7 support we may need to configure ALPN here.
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: rootPEMs,
					},
				},
			},
		},
	}
}

func makeDownstreamTLSTransportSocket(tlsContext *envoy_tls_v3.DownstreamTlsContext) (*envoy_core_v3.TransportSocket, error) {
	if tlsContext == nil {
		return nil, nil
	}
	return makeTransportSocket("tls", tlsContext)
}

func makeUpstreamTLSTransportSocket(tlsContext *envoy_tls_v3.UpstreamTlsContext) (*envoy_core_v3.TransportSocket, error) {
	if tlsContext == nil {
		return nil, nil
	}
	return makeTransportSocket("tls", tlsContext)
}

func makeTransportSocket(name string, config proto.Message) (*envoy_core_v3.TransportSocket, error) {
	any, err := anypb.New(config)
	if err != nil {
		return nil, err
	}
	return &envoy_core_v3.TransportSocket{
		Name: name,
		ConfigType: &envoy_core_v3.TransportSocket_TypedConfig{
			TypedConfig: any,
		},
	}, nil
}

func makeCommonTLSContextFromFiles(caFile, certFile, keyFile string) *envoy_tls_v3.CommonTlsContext {
	ctx := envoy_tls_v3.CommonTlsContext{
		TlsParams: &envoy_tls_v3.TlsParameters{},
	}

	// Verify certificate of peer if caFile is specified
	if caFile != "" {
		ctx.ValidationContextType = &envoy_tls_v3.CommonTlsContext_ValidationContext{
			ValidationContext: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_Filename{
						Filename: caFile,
					},
				},
			},
		}
	}

	// Present certificate for mTLS if cert and key files are specified
	if certFile != "" && keyFile != "" {
		ctx.TlsCertificates = []*envoy_tls_v3.TlsCertificate{
			{
				CertificateChain: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_Filename{
						Filename: certFile,
					},
				},
				PrivateKey: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_Filename{
						Filename: keyFile,
					},
				},
			},
		}
	}

	return &ctx
}

func validateListenerTLSConfig(tlsMinVersion types.TLSVersion, cipherSuites []types.TLSCipherSuite) error {
	// Validate. Configuring cipher suites is only applicable to connections negotiated
	// via TLS 1.2 or earlier. Other cases shouldn't be possible as we validate them at
	// input but be resilient to bugs later.
	if len(cipherSuites) != 0 {
		if _, ok := tlsVersionsWithConfigurableCipherSuites[tlsMinVersion]; !ok {
			return fmt.Errorf("configuring CipherSuites is only applicable to connections negotiated with TLS 1.2 or earlier, TLSMinVersion is set to %s in config", tlsMinVersion)
		}
	}

	return nil
}

var tlsVersionsWithConfigurableCipherSuites = map[types.TLSVersion]struct{}{
	// Remove these two if Envoy ever sets TLS 1.3 as default minimum
	types.TLSVersionUnspecified: {},
	types.TLSVersionAuto:        {},

	types.TLSv1_0: {},
	types.TLSv1_1: {},
	types.TLSv1_2: {},
}

func makeTLSParametersFromProxyTLSConfig(tlsConf *structs.MeshDirectionalTLSConfig) *envoy_tls_v3.TlsParameters {
	if tlsConf == nil {
		return &envoy_tls_v3.TlsParameters{}
	}

	return makeTLSParametersFromTLSConfig(tlsConf.TLSMinVersion, tlsConf.TLSMaxVersion, tlsConf.CipherSuites)
}

func makeTLSParametersFromTLSConfig(
	tlsMinVersion types.TLSVersion,
	tlsMaxVersion types.TLSVersion,
	cipherSuites []types.TLSCipherSuite,
) *envoy_tls_v3.TlsParameters {
	tlsParams := envoy_tls_v3.TlsParameters{}

	if tlsMinVersion != types.TLSVersionUnspecified {
		if minVersion, ok := envoyTLSVersions[tlsMinVersion]; ok {
			tlsParams.TlsMinimumProtocolVersion = minVersion
		}
	}
	if tlsMaxVersion != types.TLSVersionUnspecified {
		if maxVersion, ok := envoyTLSVersions[tlsMaxVersion]; ok {
			tlsParams.TlsMaximumProtocolVersion = maxVersion
		}
	}
	if len(cipherSuites) != 0 {
		tlsParams.CipherSuites = types.MarshalEnvoyTLSCipherSuiteStrings(cipherSuites)
	}

	return &tlsParams
}

var envoyTLSVersions = map[types.TLSVersion]envoy_tls_v3.TlsParameters_TlsProtocol{
	types.TLSVersionAuto: envoy_tls_v3.TlsParameters_TLS_AUTO,
	types.TLSv1_0:        envoy_tls_v3.TlsParameters_TLSv1_0,
	types.TLSv1_1:        envoy_tls_v3.TlsParameters_TLSv1_1,
	types.TLSv1_2:        envoy_tls_v3.TlsParameters_TLSv1_2,
	types.TLSv1_3:        envoy_tls_v3.TlsParameters_TLSv1_3,
}
