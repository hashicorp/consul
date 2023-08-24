// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxystateconverter

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/config"
	"github.com/hashicorp/consul/agent/xds/naming"
	"github.com/hashicorp/consul/agent/xds/platform"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/hashicorp/consul/types"
)

// listenersFromSnapshot adds listeners to pbmesh.ProxyState using the config snapshot.
func (s *Converter) listenersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) error {
	if cfgSnap == nil {
		return errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.listenersFromSnapshotConnectProxy(cfgSnap)
	case structs.ServiceKindTerminatingGateway,
		structs.ServiceKindMeshGateway,
		structs.ServiceKindIngressGateway,
		structs.ServiceKindAPIGateway:
		// TODO(proxystate): gateway support will be added in the future
		//return s.listenersFromSnapshotGateway(cfgSnap)
	default:
		return fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
	return nil
}

// listenersFromSnapshotConnectProxy returns the "listeners" for a connect proxy service
func (s *Converter) listenersFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) error {
	// This is the list of listeners we add to. It will be empty to start.
	listeners := s.proxyState.Listeners
	var err error

	// Configure inbound listener.
	inboundListener, err := s.makeInboundListener(cfgSnap, xdscommon.PublicListenerName)
	if err != nil {
		return err
	}
	listeners = append(listeners, inboundListener)

	// This outboundListener is exclusively used when transparent proxy mode is active.
	// In that situation there is a single listener where we are redirecting outbound traffic,
	// and each upstream gets a filter chain attached to that listener.
	var outboundListener *pbproxystate.Listener

	if cfgSnap.Proxy.Mode == structs.ProxyModeTransparent {
		port := iptables.DefaultTProxyOutboundPort
		if cfgSnap.Proxy.TransparentProxy.OutboundListenerPort != 0 {
			port = cfgSnap.Proxy.TransparentProxy.OutboundListenerPort
		}

		opts := makeListenerOpts{
			name: xdscommon.OutboundListenerName,
			//accessLogs: cfgSnap.Proxy.AccessLogs,
			addr:      "127.0.0.1",
			port:      port,
			direction: pbproxystate.Direction_DIRECTION_OUTBOUND,
			logger:    s.Logger,
		}
		outboundListener = makeListener(opts)
		if outboundListener.Capabilities == nil {
			outboundListener.Capabilities = []pbproxystate.Capability{}
		}
		outboundListener.Capabilities = append(outboundListener.Capabilities, pbproxystate.Capability_CAPABILITY_TRANSPARENT)
	}

	// TODO(proxystate): tracing escape hatch will be added in the future. It will be added to the top level in proxystate, and used in xds generation.
	//proxyCfg, err := config.ParseProxyConfig(cfgSnap.Proxy.Config)
	//if err != nil {
	//	// Don't hard fail on a config typo, just warn. The parse func returns
	//	// default config if there is an error so it's safe to continue.
	//	s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	//}
	//var tracing *envoy_http_v3.HttpConnectionManager_Tracing
	//if proxyCfg.ListenerTracingJSON != "" {
	//	if tracing, err = makeTracingFromUserConfig(proxyCfg.ListenerTracingJSON); err != nil {
	//		s.Logger.Warn("failed to parse ListenerTracingJSON config", "error", err)
	//	}
	//}

	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil {
		return err
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
			upstreamListener := &pbproxystate.Listener{
				EscapeHatchListener: cfg.EnvoyListenerJSON,
			}
			listeners = append(listeners, upstreamListener)
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
				return err
			}

			clusterName = s.getTargetClusterName(upstreamsSnapshot, chain, target.ID, false)
			if clusterName == "" {
				continue
			}
		}

		filterName := fmt.Sprintf("%s.%s.%s.%s", chain.ServiceName, chain.Namespace, chain.Partition, chain.Datacenter)

		// Generate the upstream listeners for when they are explicitly set with a local bind port or socket path
		if upstreamCfg != nil && upstreamCfg.HasLocalPortOrSocket() {
			router, err := s.makeUpstreamRouter(routerOpts{
				// TODO(proxystate): access logs and tracing will be added in the future.
				//accessLogs:  &cfgSnap.Proxy.AccessLogs,
				routeName:   uid.EnvoyID(),
				clusterName: clusterName,
				filterName:  filterName,
				protocol:    cfg.Protocol,
				useRDS:      useRDS,
				//tracing:     tracing,
			})
			if err != nil {
				return err
			}

			opts := makeListenerOpts{
				name: uid.EnvoyID(),
				//accessLogs: cfgSnap.Proxy.AccessLogs,
				direction: pbproxystate.Direction_DIRECTION_OUTBOUND,
				logger:    s.Logger,
				upstream:  upstreamCfg,
			}
			upstreamListener := makeListener(opts)
			upstreamListener.BalanceConnections = balanceConnections[cfg.BalanceOutboundConnections]

			upstreamListener.Routers = append(upstreamListener.Routers, router)
			listeners = append(listeners, upstreamListener)

			// Avoid creating filter chains below for upstreams that have dedicated listeners
			continue
		}

		// The rest of this loop is used exclusively for transparent proxies.
		// Below we create a filter chain per upstream, rather than a listener per upstream
		// as we do for explicit upstreams above.

		upstreamRouter, err := s.makeUpstreamRouter(routerOpts{
			//accessLogs:  &cfgSnap.Proxy.AccessLogs,
			routeName:   uid.EnvoyID(),
			clusterName: clusterName,
			filterName:  filterName,
			protocol:    cfg.Protocol,
			useRDS:      useRDS,
			//tracing:     tracing,
		})
		if err != nil {
			return err
		}

		endpoints := cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[uid][chain.ID()]
		uniqueAddrs := make(map[string]struct{})

		if chain.Partition == cfgSnap.ProxyID.PartitionOrDefault() {
			for _, ip := range chain.AutoVirtualIPs {
				uniqueAddrs[ip] = struct{}{}
			}
			for _, ip := range chain.ManualVirtualIPs {
				uniqueAddrs[ip] = struct{}{}
			}
		}

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
				if vip := e.Service.TaggedAddresses[naming.VirtualIPTag]; vip.Address != "" {
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
		upstreamRouter.Match = makeRouterMatchFromAddrs(uniqueAddrs)

		// Only attach the filter chain if there are addresses to match on
		if upstreamRouter.Match != nil && len(upstreamRouter.Match.PrefixRanges) > 0 {
			outboundListener.Routers = append(outboundListener.Routers, upstreamRouter)
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
			upstreamRouter, err := s.makeUpstreamRouter(routerOpts{
				//accessLogs: &cfgSnap.Proxy.AccessLogs,
				routeName:  routeName,
				filterName: routeName,
				protocol:   svcConfig.Protocol,
				useRDS:     true,
				//tracing:    tracing,
			})
			if err != nil {
				return err
			}
			upstreamRouter.Match = makeRouterMatchFromAddressWithPort("", svcConfig.Destination.Port)
			outboundListener.Routers = append(outboundListener.Routers, upstreamRouter)
			requiresHTTPInspector = true
		} else {
			for _, address := range svcConfig.Destination.Addresses {
				clusterName := clusterNameForDestination(cfgSnap, uid.Name, address, uid.NamespaceOrDefault(), uid.PartitionOrDefault())

				upstreamRouter, err := s.makeUpstreamRouter(routerOpts{
					//accessLogs:  &cfgSnap.Proxy.AccessLogs,
					routeName:   uid.EnvoyID(),
					clusterName: clusterName,
					filterName:  clusterName,
					protocol:    svcConfig.Protocol,
					//tracing:     tracing,
				})
				if err != nil {
					return err
				}

				upstreamRouter.Match = makeRouterMatchFromAddressWithPort(address, svcConfig.Destination.Port)
				outboundListener.Routers = append(outboundListener.Routers, upstreamRouter)

				requiresTLSInspector = len(upstreamRouter.Match.ServerNames) != 0 || requiresTLSInspector
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if requiresTLSInspector {
		outboundListener.Capabilities = append(outboundListener.Capabilities, pbproxystate.Capability_CAPABILITY_L4_TLS_INSPECTION)
	}

	if requiresHTTPInspector {
		outboundListener.Capabilities = append(outboundListener.Capabilities, pbproxystate.Capability_CAPABILITY_L7_PROTOCOL_INSPECTION)
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
			upstreamListener := &pbproxystate.Listener{
				EscapeHatchListener: cfg.EnvoyListenerJSON,
			}
			listeners = append(listeners, upstreamListener)
			continue
		}

		tbs, ok := cfgSnap.ConnectProxy.UpstreamPeerTrustBundles.Get(uid.Peer)
		if !ok {
			// this should never happen since we loop through upstreams with
			// set trust bundles
			return fmt.Errorf("trust bundle not ready for peer %s", uid.Peer)
		}

		clusterName := generatePeeredClusterName(uid, tbs)

		// Generate the upstream listeners for when they are explicitly set with a local bind port or socket path
		if upstreamCfg != nil && upstreamCfg.HasLocalPortOrSocket() {
			upstreamRouter, err := s.makeUpstreamRouter(routerOpts{
				//accessLogs:  &cfgSnap.Proxy.AccessLogs,
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
				return err
			}

			opts := makeListenerOpts{
				name: uid.EnvoyID(),
				//accessLogs: cfgSnap.Proxy.AccessLogs,
				direction: pbproxystate.Direction_DIRECTION_OUTBOUND,
				logger:    s.Logger,
				upstream:  upstreamCfg,
			}
			upstreamListener := makeListener(opts)
			upstreamListener.BalanceConnections = balanceConnections[cfg.BalanceOutboundConnections]

			upstreamListener.Routers = []*pbproxystate.Router{
				upstreamRouter,
			}
			listeners = append(listeners, upstreamListener)

			// Avoid creating filter chains below for upstreams that have dedicated listeners
			continue
		}

		// The rest of this loop is used exclusively for transparent proxies.
		// Below we create a filter chain per upstream, rather than a listener per upstream
		// as we do for explicit upstreams above.

		upstreamRouter, err := s.makeUpstreamRouter(routerOpts{
			//accessLogs:  &cfgSnap.Proxy.AccessLogs,
			routeName:   uid.EnvoyID(),
			clusterName: clusterName,
			filterName: fmt.Sprintf("%s.%s.%s",
				uid.Name,
				uid.NamespaceOrDefault(),
				uid.Peer),
			protocol:   cfg.Protocol,
			useRDS:     false,
			statPrefix: "upstream_peered.",
			//tracing:    tracing,
		})
		if err != nil {
			return err
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
				if vip := e.Service.TaggedAddresses[naming.VirtualIPTag]; vip.Address != "" {
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
		upstreamRouter.Match = makeRouterMatchFromAddrs(uniqueAddrs)

		// Only attach the filter chain if there are addresses to match on
		if upstreamRouter.Match != nil && len(upstreamRouter.Match.PrefixRanges) > 0 {
			outboundListener.Routers = append(outboundListener.Routers, upstreamRouter)
		}

	}

	if outboundListener != nil {
		// Add a passthrough for every mesh endpoint that can be dialed directly,
		// as opposed to via a virtual IP.
		var passthroughRouters []*pbproxystate.Router

		for _, targets := range cfgSnap.ConnectProxy.PassthroughUpstreams {
			for tid, addrs := range targets {
				uid := proxycfg.NewUpstreamIDFromTargetID(tid)

				sni := connect.ServiceSNI(
					uid.Name, "", uid.NamespaceOrDefault(), uid.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

				routerName := fmt.Sprintf("%s.%s.%s.%s", uid.Name, uid.NamespaceOrDefault(), uid.PartitionOrDefault(), cfgSnap.Datacenter)

				upstreamRouter, err := s.makeUpstreamRouter(routerOpts{
					//accessLogs:  &cfgSnap.Proxy.AccessLogs,
					clusterName: "passthrough~" + sni,
					filterName:  routerName,
					protocol:    "tcp",
				})
				if err != nil {
					return err
				}
				upstreamRouter.Match = makeRouterMatchFromAddrs(addrs)

				passthroughRouters = append(passthroughRouters, upstreamRouter)
			}
		}

		outboundListener.Routers = append(outboundListener.Routers, passthroughRouters...)

		// Add a catch-all filter chain that acts as a TCP proxy to destinations outside the mesh
		if meshConf := cfgSnap.MeshConfig(); meshConf == nil ||
			!meshConf.TransparentProxy.MeshDestinationsOnly {

			upstreamRouter, err := s.makeUpstreamRouter(routerOpts{
				//accessLogs:  &cfgSnap.Proxy.AccessLogs,
				clusterName: naming.OriginalDestinationClusterName,
				filterName:  naming.OriginalDestinationClusterName,
				protocol:    "tcp",
			})
			if err != nil {
				return err
			}
			outboundListener.DefaultRouter = upstreamRouter
		}

		// Only add the outbound listener if configured.
		if len(outboundListener.Routers) > 0 || outboundListener.DefaultRouter != nil {
			listeners = append(listeners, outboundListener)

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
			upstreamListener := &pbproxystate.Listener{
				EscapeHatchListener: cfg.EnvoyListenerJSON,
			}
			listeners = append(listeners, upstreamListener)
			continue
		}

		opts := makeListenerOpts{
			name: uid.EnvoyID(),
			//accessLogs: cfgSnap.Proxy.AccessLogs,
			direction: pbproxystate.Direction_DIRECTION_OUTBOUND,
			logger:    s.Logger,
			upstream:  u,
		}
		upstreamListener := makeListener(opts)
		upstreamListener.BalanceConnections = balanceConnections[cfg.BalanceOutboundConnections]

		upstreamRouter, err := s.makeUpstreamRouter(routerOpts{
			// TODO (SNI partition) add partition for upstream SNI
			//accessLogs:  &cfgSnap.Proxy.AccessLogs,
			clusterName: connect.UpstreamSNI(u, "", cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain),
			filterName:  uid.EnvoyID(),
			routeName:   uid.EnvoyID(),
			protocol:    cfg.Protocol,
			//tracing:     tracing,
		})
		if err != nil {
			return err
		}
		upstreamListener.Routers = []*pbproxystate.Router{
			upstreamRouter,
		}
		listeners = append(listeners, upstreamListener)
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
			return err
		}
		listeners = append(listeners, l)
	}

	// Set listeners on the proxy state.
	s.proxyState.Listeners = listeners

	return nil
}

func makeRouterMatchFromAddrs(addrs map[string]struct{}) *pbproxystate.Match {
	ranges := make([]*pbproxystate.CidrRange, 0)

	for addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}

		pfxLen := uint32(32)
		if ip.To4() == nil {
			pfxLen = 128
		}
		ranges = append(ranges, &pbproxystate.CidrRange{
			AddressPrefix: addr,
			PrefixLen:     &wrapperspb.UInt32Value{Value: pfxLen},
		})
	}

	return &pbproxystate.Match{
		PrefixRanges: ranges,
	}
}

func makeRouterMatchFromAddressWithPort(address string, port int) *pbproxystate.Match {
	ranges := make([]*pbproxystate.CidrRange, 0)

	ip := net.ParseIP(address)
	if ip == nil {
		if address != "" {
			return &pbproxystate.Match{
				ServerNames:     []string{address},
				DestinationPort: &wrapperspb.UInt32Value{Value: uint32(port)},
			}
		}
		return &pbproxystate.Match{
			DestinationPort: &wrapperspb.UInt32Value{Value: uint32(port)},
		}
	}

	pfxLen := uint32(32)
	if ip.To4() == nil {
		pfxLen = 128
	}
	ranges = append(ranges, &pbproxystate.CidrRange{
		AddressPrefix: address,
		PrefixLen:     &wrapperspb.UInt32Value{Value: pfxLen},
	})

	return &pbproxystate.Match{
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

// TODO(proxystate): Gateway support will be added in the future.
// Functions to add from agent/xds/listeners.go:
// func listenersFromSnapshotGateway

// makeListener returns a listener with name and bind details set. Routers and destinations
// must be added before it's useful.
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
	addr string
	//accessLogs structs.AccessLogsConfig
	logger    hclog.Logger
	mode      string
	name      string
	path      string
	port      int
	direction pbproxystate.Direction
	upstream  *structs.Upstream
}

func makeListener(opts makeListenerOpts) *pbproxystate.Listener {
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

func makeListenerWithDefault(opts makeListenerOpts) *pbproxystate.Listener {
	if opts.addr == "" {
		opts.addr = "127.0.0.1"
	}
	// TODO(proxystate): Access logs will be added in the future. It will be added to top level IR, and used by xds code generation.
	//accessLog, err := accesslogs.MakeAccessLogs(&opts.accessLogs, true)
	//if err != nil && opts.logger != nil {
	//	// Since access logging is non-essential for routing, warn and move on
	//	opts.logger.Warn("error generating access log xds", err)
	//}
	return &pbproxystate.Listener{
		Name: fmt.Sprintf("%s:%s:%d", opts.name, opts.addr, opts.port),
		//AccessLog:        accessLog,
		BindAddress: &pbproxystate.Listener_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: opts.addr,
				Port: uint32(opts.port),
			},
		},
		Direction: opts.direction,
	}
}

func makePipeListener(opts makeListenerOpts) *pbproxystate.Listener {
	// TODO(proxystate): Access logs will be added in the future. It will be added to top level IR, and used by xds code generation.
	//accessLog, err := accesslogs.MakeAccessLogs(&opts.accessLogs, true)
	//if err != nil && opts.logger != nil {
	//	// Since access logging is non-essential for routing, warn and move on
	//	opts.logger.Warn("error generating access log xds", err)
	//}
	return &pbproxystate.Listener{
		Name: fmt.Sprintf("%s:%s", opts.name, opts.path),
		//AccessLog:        accessLog,
		BindAddress: &pbproxystate.Listener_UnixSocket{
			UnixSocket: &pbproxystate.UnixSocketAddress{Path: opts.path, Mode: opts.mode},
		},
		Direction: opts.direction,
	}
}

// TODO(proxystate): Escape hatches will be added in the future.
// Functions to add from agent/xds/listeners.go:
// func makeListenerFromUserConfig

// TODO(proxystate): Intentions will be added in the future
// Functions to add from agent/xds/listeners.go:
// func injectConnectFilters

// TODO(proxystate): httpConnectionManager constants will need to be added when used for listeners L7 in the future.
// Constants to add from agent/xds/listeners.go:
// const httpConnectionManagerOldName
// const httpConnectionManagerNewName

// TODO(proxystate): Extracting RDS resource names will be used when wiring up xds v2 server in the future.
// Functions to add from agent/xds/listeners.go:
// func extractRdsResourceNames

// TODO(proxystate): Intentions will be added in the future.
// Functions to add from agent/xds/listeners.go:
// func injectHTTPFilterOnFilterChains

// NOTE: This method MUST only be used for connect proxy public listeners,
// since TLS validation will be done against root certs for all peers
// that might dial this proxy.
func (s *Converter) injectConnectTLSForPublicListener(cfgSnap *proxycfg.ConfigSnapshot, listener *pbproxystate.Listener) error {
	transportSocket, err := s.createInboundMeshMTLS(cfgSnap)
	if err != nil {
		return err
	}

	for idx := range listener.Routers {
		listener.Routers[idx].InboundTls = transportSocket
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

func (s *Converter) createInboundMeshMTLS(cfgSnap *proxycfg.ConfigSnapshot) (*pbproxystate.TransportSocket, error) {
	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
	case structs.ServiceKindMeshGateway:
	default:
		return nil, fmt.Errorf("cannot inject peering trust bundles for kind %q", cfgSnap.Kind)
	}

	cfg, err := config.ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// Add all trust bundle peer names, including local.
	trustBundlePeerNames := []string{"local"}
	for _, tb := range cfgSnap.PeeringTrustBundles() {
		trustBundlePeerNames = append(trustBundlePeerNames, tb.PeerName)
	}
	// Arbitrary UUID to reference the identity by.
	uuid, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}
	// Create the transport socket
	ts := &pbproxystate.TransportSocket{}
	ts.ConnectionTls = &pbproxystate.TransportSocket_InboundMesh{
		InboundMesh: &pbproxystate.InboundMeshMTLS{
			IdentityKey: uuid,
			ValidationContext: &pbproxystate.MeshInboundValidationContext{
				TrustBundlePeerNameKeys: trustBundlePeerNames,
			},
		},
	}
	s.proxyState.LeafCertificates[uuid] = &pbproxystate.LeafCertificate{
		Cert: cfgSnap.Leaf().CertPEM,
		Key:  cfgSnap.Leaf().PrivateKeyPEM,
	}
	ts.TlsParameters = makeTLSParametersFromProxyTLSConfig(cfgSnap.MeshConfigTLSIncoming())
	ts.AlpnProtocols = getAlpnProtocols(cfg.Protocol)

	return ts, nil
}

func (s *Converter) makeInboundListener(cfgSnap *proxycfg.ConfigSnapshot, name string) (*pbproxystate.Listener, error) {
	l := &pbproxystate.Listener{}
	l.Routers = make([]*pbproxystate.Router, 0)
	var err error

	cfg, err := config.ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// This controls if we do L4 or L7 intention checks.
	useHTTPFilter := structs.IsProtocolHTTPLike(cfg.Protocol)

	// TODO(proxystate): Escape hatches will be added in the future. This one is a top level escape hatch.
	// Generate and return custom public listener from config if one was provided.
	//if cfg.PublicListenerJSON != "" {
	//	l, err = makeListenerFromUserConfig(cfg.PublicListenerJSON)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	// For HTTP-like services attach an RBAC http filter and do a best-effort insert
	//	if useHTTPFilter {
	//		httpAuthzFilter, err := makeRBACHTTPFilter(
	//			cfgSnap.ConnectProxy.Intentions,
	//			cfgSnap.IntentionDefaultAllow,
	//			rbacLocalInfo{
	//				trustDomain: cfgSnap.Roots.TrustDomain,
	//				datacenter:  cfgSnap.Datacenter,
	//				partition:   cfgSnap.ProxyID.PartitionOrDefault(),
	//			},
	//			cfgSnap.ConnectProxy.InboundPeerTrustBundles,
	//		)
	//		if err != nil {
	//			return nil, err
	//		}
	//
	//		// Try our best to inject the HTTP RBAC filter.
	//		if err := injectHTTPFilterOnFilterChains(l, httpAuthzFilter); err != nil {
	//			s.Logger.Warn(
	//				"could not inject the HTTP RBAC filter to enforce intentions on user-provided "+
	//					"'envoy_public_listener_json' config; falling back on the RBAC network filter instead",
	//				"proxy", cfgSnap.ProxyID,
	//				"error", err,
	//			)
	//
	//			// If we get an error inject the RBAC network filter instead.
	//			useHTTPFilter = false
	//		}
	//	}
	//
	//	err := s.finalizePublicListenerFromConfig(l, cfgSnap, useHTTPFilter)
	//	if err != nil {
	//		return nil, fmt.Errorf("failed to attach Consul filters and TLS context to custom public listener: %v", err)
	//	}
	//	return l, nil
	//}

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
		name: name,
		//accessLogs: cfgSnap.Proxy.AccessLogs,
		addr:      addr,
		port:      port,
		direction: pbproxystate.Direction_DIRECTION_INBOUND,
		logger:    s.Logger,
	}
	l = makeListener(opts)
	l.BalanceConnections = balanceConnections[cfg.BalanceInboundConnections]

	// TODO(proxystate): Escape hatches will be added in the future. This one is a top level escape hatch.
	//var tracing *envoy_http_v3.HttpConnectionManager_Tracing
	//if cfg.ListenerTracingJSON != "" {
	//	if tracing, err = makeTracingFromUserConfig(cfg.ListenerTracingJSON); err != nil {
	//		s.Logger.Warn("failed to parse ListenerTracingJSON config", "error", err)
	//	}
	//}

	// make local app cluster router
	localAppRouter := &pbproxystate.Router{}

	destOpts := destinationOpts{
		protocol:         cfg.Protocol,
		filterName:       name,
		routeName:        name,
		cluster:          xdscommon.LocalAppClusterName,
		requestTimeoutMs: cfg.LocalRequestTimeoutMs,
		idleTimeoutMs:    cfg.LocalIdleTimeoutMs,
		//tracing:          tracing,
		//accessLogs:       &cfgSnap.Proxy.AccessLogs,
		logger: s.Logger,
	}

	err = s.addRouterDestination(destOpts, localAppRouter)
	if err != nil {
		return nil, err
	}

	if useHTTPFilter {
		l7Dest := localAppRouter.GetL7()
		if l7Dest == nil {
			return nil, fmt.Errorf("l7 destination on inbound listener should not be empty")
		}
		l7Dest.AddEmptyIntention = true

		// TODO(proxystate): L7 Intentions and JWT Auth will be added in the future.
		//jwtFilter, jwtFilterErr := makeJWTAuthFilter(cfgSnap.JWTProviders, cfgSnap.ConnectProxy.Intentions)
		//if jwtFilterErr != nil {
		//	return nil, jwtFilterErr
		//}
		//rbacFilter, err := makeRBACHTTPFilter(
		//	cfgSnap.ConnectProxy.Intentions,
		//	cfgSnap.IntentionDefaultAllow,
		//	rbacLocalInfo{
		//		trustDomain: cfgSnap.Roots.TrustDomain,
		//		datacenter:  cfgSnap.Datacenter,
		//		partition:   cfgSnap.ProxyID.PartitionOrDefault(),
		//	},
		//	cfgSnap.ConnectProxy.InboundPeerTrustBundles,
		//)
		//if err != nil {
		//	return nil, err
		//}
		//
		//filterOpts.httpAuthzFilters = []*envoy_http_v3.HttpFilter{rbacFilter}
		//
		//if jwtFilter != nil {
		//	filterOpts.httpAuthzFilters = append(filterOpts.httpAuthzFilters, jwtFilter)
		//}

		meshConfig := cfgSnap.MeshConfig()
		includeXFCC := meshConfig == nil || meshConfig.HTTP == nil || !meshConfig.HTTP.SanitizeXForwardedClientCert
		l7Dest.IncludeXfcc = includeXFCC
		l7Dest.Protocol = l7Protocols[cfg.Protocol]
		if cfg.MaxInboundConnections > 0 {
			l7Dest.MaxInboundConnections = uint64(cfg.MaxInboundConnections)
		}
	} else {
		l4Dest := localAppRouter.GetL4()
		if l4Dest == nil {
			return nil, fmt.Errorf("l4 destination on inbound listener should not be empty")
		}

		if cfg.MaxInboundConnections > 0 {
			l4Dest.MaxInboundConnections = uint64(cfg.MaxInboundConnections)
		}

		// TODO(proxystate): Intentions will be added to l4 destination in the future. This is currently done in finalizePublicListenerFromConfig.
		l4Dest.AddEmptyIntention = true
	}
	l.Routers = append(l.Routers, localAppRouter)

	err = s.finalizePublicListenerFromConfig(l, cfgSnap)
	if err != nil {
		return nil, fmt.Errorf("failed to attach Consul filters and TLS context to custom public listener: %v", err)
	}

	// When permissive mTLS mode is enabled, include an additional router
	// that matches on the `destination_port == <service port>`. Traffic sent
	// directly to the service port is passed through to the application
	// unmodified.
	if cfgSnap.Proxy.Mode == structs.ProxyModeTransparent &&
		cfgSnap.Proxy.MutualTLSMode == structs.MutualTLSModePermissive {
		router, err := makePermissiveRouter(cfgSnap, destOpts)
		if err != nil {
			return nil, fmt.Errorf("unable to add permissive mtls router: %w", err)
		}
		if router == nil {
			s.Logger.Debug("no service port defined for service in permissive mTLS mode; not adding filter chain for non-mTLS traffic")
		} else {
			l.Routers = append(l.Routers, router)

			// With tproxy, the REDIRECT iptables target rewrites the destination ip/port
			// to the proxy ip/port (e.g. 127.0.0.1:20000) for incoming packets.
			// We need the original_dst filter to recover the original destination address.
			l.Capabilities = append(l.Capabilities, pbproxystate.Capability_CAPABILITY_TRANSPARENT)
		}
	}
	return l, err
}

func makePermissiveRouter(cfgSnap *proxycfg.ConfigSnapshot, opts destinationOpts) (*pbproxystate.Router, error) {
	servicePort := cfgSnap.Proxy.LocalServicePort
	if servicePort <= 0 {
		// No service port means the service does not accept incoming traffic, so
		// the connect proxy does not need to listen for incoming non-mTLS traffic.
		return nil, nil
	}

	opts.statPrefix += "permissive_"
	dest, err := makeL4Destination(opts)
	if err != nil {
		return nil, err
	}

	router := &pbproxystate.Router{
		Match: &pbproxystate.Match{
			DestinationPort: &wrapperspb.UInt32Value{Value: uint32(servicePort)},
		},
		Destination: &pbproxystate.Router_L4{L4: dest},
	}
	return router, nil
}

// finalizePublicListenerFromConfig is used for best-effort injection of L4 intentions and TLS onto listeners.
func (s *Converter) finalizePublicListenerFromConfig(l *pbproxystate.Listener, cfgSnap *proxycfg.ConfigSnapshot) error {
	// TODO(proxystate): L4 intentions will be added in the future.
	//if !useHTTPFilter {
	//	// Best-effort injection of L4 intentions
	//	if err := s.injectConnectFilters(cfgSnap, l); err != nil {
	//		return nil
	//	}
	//}

	// Always apply TLS certificates
	if err := s.injectConnectTLSForPublicListener(cfgSnap, l); err != nil {
		return nil
	}

	return nil
}

func (s *Converter) makeExposedCheckListener(cfgSnap *proxycfg.ConfigSnapshot, cluster string, path structs.ExposePath) (*pbproxystate.Listener, error) {
	cfg, err := config.ParseProxyConfig(cfgSnap.Proxy.Config)
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
		name: listenerName,
		//accessLogs: cfgSnap.Proxy.AccessLogs,
		addr:      addr,
		port:      path.ListenerPort,
		direction: pbproxystate.Direction_DIRECTION_INBOUND,
		logger:    s.Logger,
	}
	l := makeListener(listenerOpts)

	filterName := fmt.Sprintf("exposed_path_filter_%s_%d", strippedPath, path.ListenerPort)

	destOpts := destinationOpts{
		useRDS:           false,
		protocol:         path.Protocol,
		filterName:       filterName,
		routeName:        filterName,
		cluster:          cluster,
		statPrefix:       "",
		routePath:        path.Path,
		httpAuthzFilters: nil,
		//accessLogs:       &cfgSnap.Proxy.AccessLogs,
		logger: s.Logger,
		// in the exposed check listener we don't set the tracing configuration
	}

	router := &pbproxystate.Router{}
	err = s.addRouterDestination(destOpts, router)
	if err != nil {
		return nil, err
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

		ranges := make([]*pbproxystate.CidrRange, 0, 3)
		ranges = append(ranges,
			&pbproxystate.CidrRange{AddressPrefix: "127.0.0.1", PrefixLen: &wrapperspb.UInt32Value{Value: 8}},
			&pbproxystate.CidrRange{AddressPrefix: advertise, PrefixLen: &wrapperspb.UInt32Value{Value: uint32(advertiseLen)}},
		)

		if ok, err := platform.SupportsIPv6(); err != nil {
			return nil, err
		} else if ok {
			ranges = append(ranges,
				&pbproxystate.CidrRange{AddressPrefix: "::1", PrefixLen: &wrapperspb.UInt32Value{Value: 128}},
			)
		}

		router.Match = &pbproxystate.Match{
			SourcePrefixRanges: ranges,
		}
	}

	l.Routers = []*pbproxystate.Router{router}

	return l, err
}

// TODO(proxystate): Gateway support will be added in the future.
// Functions and types to convert from agent/xds/listeners.go:
// func makeTerminatingGatewayListener
// type terminatingGatewayFilterChainOpts
// func makeFilterChainTerminatingGateway
// func makeMeshGatewayListener
// func makeMeshGatewayPeerFilterChain

type routerOpts struct {
	//accessLogs           *structs.AccessLogsConfig
	routeName   string
	clusterName string
	filterName  string
	protocol    string
	useRDS      bool
	statPrefix  string
	//forwardClientDetails bool
	//forwardClientPolicy  envoy_http_v3.HttpConnectionManager_ForwardClientCertDetails
	//tracing              *envoy_http_v3.HttpConnectionManager_Tracing
}

func (g *Converter) makeUpstreamRouter(opts routerOpts) (*pbproxystate.Router, error) {
	if opts.statPrefix == "" {
		opts.statPrefix = "upstream."
	}

	router := &pbproxystate.Router{}

	err := g.addRouterDestination(destinationOpts{
		useRDS:     opts.useRDS,
		protocol:   opts.protocol,
		filterName: opts.filterName,
		routeName:  opts.routeName,
		cluster:    opts.clusterName,
		statPrefix: opts.statPrefix,
		//forwardClientDetails: opts.forwardClientDetails,
		//forwardClientPolicy:  opts.forwardClientPolicy,
		//tracing:              opts.tracing,
		//accessLogs:           opts.accessLogs,
		logger: g.Logger,
	}, router)
	if err != nil {
		return nil, err
	}

	return router, nil
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

func (s *Converter) getAndModifyUpstreamConfigForListener(
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

func (s *Converter) getAndModifyUpstreamConfigForPeeredListener(
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

type destinationOpts struct {
	// All listener filters
	// TODO(proxystate): access logs support will be added later
	//accessLogs *structs.AccessLogsConfig
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
	// TODO(proxystate): tracing support will be added later
	//tracing              *envoy_http_v3.HttpConnectionManager_Tracing
	useRDS bool
}

func (g *Converter) addRouterDestination(opts destinationOpts, router *pbproxystate.Router) error {
	switch opts.protocol {
	case "grpc", "http2", "http":
		dest, err := g.makeL7Destination(opts)
		if err != nil {
			return err
		}
		router.Destination = &pbproxystate.Router_L7{
			L7: dest,
		}
		return nil
	case "tcp":
		fallthrough
	default:
		if opts.useRDS {
			return fmt.Errorf("RDS is not compatible with the tcp proxy filter")
		} else if opts.cluster == "" {
			return fmt.Errorf("cluster name is required for a tcp proxy filter")
		}
		dest, err := makeL4Destination(opts)
		if err != nil {
			return err
		}
		router.Destination = &pbproxystate.Router_L4{
			L4: dest,
		}
		return nil
	}
}

func makeL4Destination(opts destinationOpts) (*pbproxystate.L4Destination, error) {
	// TODO(proxystate): implement access logs at top level
	//accessLogs, err := accesslogs.MakeAccessLogs(opts.accessLogs, false)
	//if err != nil && opts.logger != nil {
	//	opts.logger.Warn("could not make access log xds for tcp proxy", err)
	//}

	l4Dest := &pbproxystate.L4Destination{
		//AccessLog:        accessLogs,
		Name:       opts.cluster,
		StatPrefix: makeStatPrefix(opts.statPrefix, opts.filterName),
	}
	return l4Dest, nil
}

func makeStatPrefix(prefix, filterName string) string {
	// Replace colons here because Envoy does that in the metrics for the actual
	// clusters but doesn't in the stat prefix here while dashboards assume they
	// will match.
	return fmt.Sprintf("%s%s", prefix, strings.Replace(filterName, ":", "_", -1))
}

func (g *Converter) makeL7Destination(opts destinationOpts) (*pbproxystate.L7Destination, error) {
	dest := &pbproxystate.L7Destination{}

	// TODO(proxystate) access logs will be added to proxystate top level and in xds generation
	//accessLogs, err := accesslogs.MakeAccessLogs(opts.accessLogs, false)
	//if err != nil && opts.logger != nil {
	//	opts.logger.Warn("could not make access log xds for http connection manager", err)
	//}

	// An L7 Destination's name will be the route name, so during xds generation the route can be looked up.
	dest.Name = opts.routeName
	dest.StatPrefix = makeStatPrefix(opts.statPrefix, opts.filterName)

	// TODO(proxystate) tracing will be added at the top level proxystate and xds generation
	//if opts.tracing != nil {
	//	cfg.Tracing = opts.tracing
	//}

	if opts.useRDS {
		if opts.cluster != "" {
			return nil, fmt.Errorf("cannot specify cluster name when using RDS")
		}
	} else {
		dest.StaticRoute = true

		if opts.cluster == "" {
			return nil, fmt.Errorf("must specify cluster name when not using RDS")
		}

		routeRule := &pbproxystate.RouteRule{
			Match: &pbproxystate.RouteMatch{
				PathMatch: &pbproxystate.PathMatch{
					PathMatch: &pbproxystate.PathMatch_Prefix{
						Prefix: "/",
					},
				},
				// TODO(banks) Envoy supports matching only valid GRPC
				// requests which might be nice to add here for gRPC services
				// but it's not supported in our current envoy SDK version
				// although docs say it was supported by 1.8.0. Going to defer
				// that until we've updated the deps.
			},
			Destination: &pbproxystate.RouteDestination{
				Destination: &pbproxystate.RouteDestination_Cluster{
					Cluster: &pbproxystate.DestinationCluster{
						Name: opts.cluster,
					},
				},
			},
		}

		var timeoutCfg *pbproxystate.TimeoutConfig
		r := routeRule.GetDestination()

		if opts.requestTimeoutMs != nil {
			if timeoutCfg == nil {
				timeoutCfg = &pbproxystate.TimeoutConfig{}
			}
			timeoutCfg.Timeout = durationpb.New(time.Duration(*opts.requestTimeoutMs) * time.Millisecond)
		}

		if opts.idleTimeoutMs != nil {
			if timeoutCfg == nil {
				timeoutCfg = &pbproxystate.TimeoutConfig{}
			}
			timeoutCfg.IdleTimeout = durationpb.New(time.Duration(*opts.idleTimeoutMs) * time.Millisecond)
		}
		r.DestinationConfiguration = &pbproxystate.DestinationConfiguration{
			TimeoutConfig: timeoutCfg,
		}

		// If a path is provided, do not match on a catch-all prefix
		if opts.routePath != "" {
			routeRule.Match.PathMatch = &pbproxystate.PathMatch{
				PathMatch: &pbproxystate.PathMatch_Exact{
					Exact: opts.routePath,
				},
			}
		}

		// Create static route object
		route := &pbproxystate.Route{
			VirtualHosts: []*pbproxystate.VirtualHost{
				{
					Name:    opts.filterName,
					Domains: []string{"*"},
					RouteRules: []*pbproxystate.RouteRule{
						routeRule,
					},
				},
			},
		}
		// Save the route to proxy state.
		g.proxyState.Routes[opts.routeName] = route
	}

	dest.Protocol = l7Protocols[opts.protocol]

	// TODO(proxystate) need to include xfcc policy in future L7 task
	//// Note the default leads to setting HttpConnectionManager_SANITIZE
	//if opts.forwardClientDetails {
	//	cfg.ForwardClientCertDetails = opts.forwardClientPolicy
	//	cfg.SetCurrentClientCertDetails = &envoy_http_v3.HttpConnectionManager_SetCurrentClientCertDetails{
	//		Subject: &wrapperspb.BoolValue{Value: true},
	//		Cert:    true,
	//		Chain:   true,
	//		Dns:     true,
	//		Uri:     true,
	//	}
	//}

	// Like injectConnectFilters for L4, here we ensure that the first filter
	// (other than the "envoy.grpc_http1_bridge" filter) in the http filter
	// chain of a public listener is the authz filter to prevent unauthorized
	// access and that every filter chain uses our TLS certs.
	if len(opts.httpAuthzFilters) > 0 {
		// TODO(proxystate) support intentions in the future
		dest.Intentions = make([]*pbproxystate.L7Intention, 0)
		//cfg.HttpFilters = append(opts.httpAuthzFilters, cfg.HttpFilters...)
	}

	// TODO(proxystate) add grpc http filters in xds in future L7 task
	//if opts.protocol == "grpc" {
	//	grpcHttp1Bridge, err := makeEnvoyHTTPFilter(
	//		"envoy.filters.http.grpc_http1_bridge",
	//		&envoy_grpc_http1_bridge_v3.Config{},
	//	)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	// In envoy 1.14.x the default value "stats_for_all_methods=true" was
	//	// deprecated, and was changed to "false" in 1.18.x. Avoid using the
	//	// default. TODO: we may want to expose this to users somehow easily.
	//	grpcStatsFilter, err := makeEnvoyHTTPFilter(
	//		"envoy.filters.http.grpc_stats",
	//		&envoy_grpc_stats_v3.FilterConfig{
	//			PerMethodStatSpecifier: &envoy_grpc_stats_v3.FilterConfig_StatsForAllMethods{
	//				StatsForAllMethods: makeBoolValue(true),
	//			},
	//		},
	//	)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	// Add grpc bridge before router and authz, and the stats in front of that.
	//	cfg.HttpFilters = append([]*envoy_http_v3.HttpFilter{
	//		grpcStatsFilter,
	//		grpcHttp1Bridge,
	//	}, cfg.HttpFilters...)
	//}

	return dest, nil
}

var tlsVersionsWithConfigurableCipherSuites = map[types.TLSVersion]struct{}{
	// Remove these two if Envoy ever sets TLS 1.3 as default minimum
	types.TLSVersionUnspecified: {},
	types.TLSVersionAuto:        {},

	types.TLSv1_0: {},
	types.TLSv1_1: {},
	types.TLSv1_2: {},
}

func makeTLSParametersFromProxyTLSConfig(tlsConf *structs.MeshDirectionalTLSConfig) *pbproxystate.TLSParameters {
	if tlsConf == nil {
		return &pbproxystate.TLSParameters{}
	}

	return makeTLSParametersFromTLSConfig(tlsConf.TLSMinVersion, tlsConf.TLSMaxVersion, tlsConf.CipherSuites)
}

func makeTLSParametersFromTLSConfig(
	tlsMinVersion types.TLSVersion,
	tlsMaxVersion types.TLSVersion,
	cipherSuites []types.TLSCipherSuite,
) *pbproxystate.TLSParameters {
	tlsParams := pbproxystate.TLSParameters{}

	if tlsMinVersion != types.TLSVersionUnspecified {
		tlsParams.MinVersion = tlsVersions[tlsMinVersion]
	}
	if tlsMaxVersion != types.TLSVersionUnspecified {
		tlsParams.MaxVersion = tlsVersions[tlsMaxVersion]
	}
	if len(cipherSuites) != 0 {
		var suites []pbproxystate.TLSCipherSuite
		for _, cs := range cipherSuites {
			suites = append(suites, tlsCipherSuites[cs])
		}
		tlsParams.CipherSuites = suites
	}

	return &tlsParams
}

var tlsVersions = map[types.TLSVersion]pbproxystate.TLSVersion{
	types.TLSVersionAuto: pbproxystate.TLSVersion_TLS_VERSION_AUTO,
	types.TLSv1_0:        pbproxystate.TLSVersion_TLS_VERSION_1_0,
	types.TLSv1_1:        pbproxystate.TLSVersion_TLS_VERSION_1_1,
	types.TLSv1_2:        pbproxystate.TLSVersion_TLS_VERSION_1_2,
	types.TLSv1_3:        pbproxystate.TLSVersion_TLS_VERSION_1_3,
}

var tlsCipherSuites = map[types.TLSCipherSuite]pbproxystate.TLSCipherSuite{
	types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:       pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_AES128_GCM_SHA256,
	types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256: pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_CHACHA20_POLY1305,
	types.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:         pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_AES128_GCM_SHA256,
	types.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256:   pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_CHACHA20_POLY1305,
	types.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:          pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_AES128_SHA,
	types.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:            pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_AES128_SHA,
	types.TLS_RSA_WITH_AES_128_GCM_SHA256:               pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_AES128_GCM_SHA256,
	types.TLS_RSA_WITH_AES_128_CBC_SHA:                  pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_AES128_SHA,
	types.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:       pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_AES256_GCM_SHA384,
	types.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:         pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_AES256_GCM_SHA384,
	types.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:          pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_AES256_SHA,
	types.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:            pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_AES256_SHA,
	types.TLS_RSA_WITH_AES_256_GCM_SHA384:               pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_AES256_GCM_SHA384,
	types.TLS_RSA_WITH_AES_256_CBC_SHA:                  pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_AES256_SHA,
}

var l7Protocols = map[string]pbproxystate.L7Protocol{
	"http":  pbproxystate.L7Protocol_L7_PROTOCOL_HTTP,
	"http2": pbproxystate.L7Protocol_L7_PROTOCOL_HTTP2,
	"grpc":  pbproxystate.L7Protocol_L7_PROTOCOL_GRPC,
}

var balanceConnections = map[string]pbproxystate.BalanceConnections{
	"":                             pbproxystate.BalanceConnections_BALANCE_CONNECTIONS_DEFAULT,
	structs.ConnectionExactBalance: pbproxystate.BalanceConnections_BALANCE_CONNECTIONS_EXACT,
}
