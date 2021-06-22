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
	envoy_grpc_stats_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_stats/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/iptables"
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
	case structs.ServiceKindTerminatingGateway:
		return s.listenersFromSnapshotGateway(cfgSnap)
	case structs.ServiceKindMeshGateway:
		return s.listenersFromSnapshotGateway(cfgSnap)
	case structs.ServiceKindIngressGateway:
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
	resources[0], err = s.makeInboundListener(cfgSnap, PublicListenerName)
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

		outboundListener = makePortListener(OutboundListenerName, "127.0.0.1", port, envoy_core_v3.TrafficDirection_OUTBOUND)
		outboundListener.FilterChains = make([]*envoy_listener_v3.FilterChain, 0)
		outboundListener.ListenerFilters = []*envoy_listener_v3.ListenerFilter{
			{
				// The original_dst filter is a listener filter that recovers the original destination
				// address before the iptables redirection. This filter is needed for transparent
				// proxies because they route to upstreams using filter chains that match on the
				// destination IP address. If the filter is not present, no chain will match.
				//
				// TODO(tproxy): Hard-coded until we upgrade the go-control-plane library
				Name: "envoy.filters.listener.original_dst",
			},
		}
	}

	for id, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		upstreamCfg := cfgSnap.ConnectProxy.UpstreamConfig[id]
		cfg := s.getAndModifyUpstreamConfigForListener(id, upstreamCfg, chain)

		// If escape hatch is present, create a listener from it and move on to the next
		if cfg.EnvoyListenerJSON != "" {
			upstreamListener, err := makeListenerFromUserConfig(cfg.EnvoyListenerJSON)
			if err != nil {
				return nil, err
			}
			resources = append(resources, upstreamListener)
			continue
		}

		// Generate the upstream listeners for when they are explicitly set with a local bind port or socket path
		if outboundListener == nil || (upstreamCfg != nil && upstreamCfg.HasLocalPortOrSocket()) {
			filterChain, err := s.makeUpstreamFilterChainForDiscoveryChain(
				id,
				"",
				cfg.Protocol,
				upstreamCfg,
				chain,
				cfgSnap,
				nil,
			)
			if err != nil {
				return nil, err
			}

			upstreamListener := makeListener(id, upstreamCfg, envoy_core_v3.TrafficDirection_OUTBOUND)
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
		filterChain, err := s.makeUpstreamFilterChainForDiscoveryChain(
			id,
			"",
			cfg.Protocol,
			upstreamCfg,
			chain,
			cfgSnap,
			nil,
		)
		if err != nil {
			return nil, err
		}

		endpoints := cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[id][chain.ID()]
		uniqueAddrs := make(map[string]struct{})

		// Match on the virtual IP for the upstream service (identified by the chain's ID).
		// We do not match on all endpoints here since it would lead to load balancing across
		// all instances when any instance address is dialed.
		for _, e := range endpoints {
			if vip := e.Service.TaggedAddresses[virtualIPTag]; vip.Address != "" {
				uniqueAddrs[vip.Address] = struct{}{}
			}
		}
		if len(uniqueAddrs) > 1 {
			s.Logger.Warn("detected multiple virtual IPs for an upstream, all will be used to match traffic",
				"upstream", id)
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

		for svc, passthrough := range cfgSnap.ConnectProxy.PassthroughUpstreams {
			sn := structs.ServiceNameFromString(svc)
			u := structs.Upstream{
				DestinationName:      sn.Name,
				DestinationNamespace: sn.NamespaceOrDefault(),
			}

			filterChain, err := s.makeUpstreamFilterChainForDiscoveryChain(
				"",
				"passthrough~"+passthrough.SNI,

				// TODO(tproxy) This should use the protocol configured on the upstream's config entry
				"tcp",
				&u,
				nil,
				cfgSnap,
				nil,
			)
			if err != nil {
				return nil, err
			}
			filterChain.FilterChainMatch = makeFilterChainMatchFromAddrs(passthrough.Addrs)

			passthroughChains = append(passthroughChains, filterChain)
		}

		outboundListener.FilterChains = append(outboundListener.FilterChains, passthroughChains...)

		// Filter chains are stable sorted to avoid draining if the list is provided out of order
		sort.SliceStable(outboundListener.FilterChains, func(i, j int) bool {
			return outboundListener.FilterChains[i].FilterChainMatch.PrefixRanges[0].AddressPrefix <
				outboundListener.FilterChains[j].FilterChainMatch.PrefixRanges[0].AddressPrefix
		})

		// Add a catch-all filter chain that acts as a TCP proxy to destinations outside the mesh
		if cfgSnap.ConnectProxy.MeshConfig == nil ||
			!cfgSnap.ConnectProxy.MeshConfig.TransparentProxy.MeshDestinationsOnly {

			filterChain, err := s.makeUpstreamFilterChainForDiscoveryChain(
				"",
				OriginalDestinationClusterName,
				"tcp",
				nil,
				nil,
				cfgSnap,
				nil,
			)
			if err != nil {
				return nil, err
			}
			outboundListener.FilterChains = append(outboundListener.FilterChains, filterChain)
		}

		// Only add the outbound listener if configured.
		if len(outboundListener.FilterChains) > 0 {
			resources = append(resources, outboundListener)
		}
	}

	// Looping over explicit upstreams is only needed for prepared queries because they do not have discovery chains
	for id, u := range cfgSnap.ConnectProxy.UpstreamConfig {
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			continue
		}

		cfg, err := structs.ParseUpstreamConfig(u.Config)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn("failed to parse", "upstream", u.Identifier(), "error", err)
		}
		upstreamListener := makeListener(id, u, envoy_core_v3.TrafficDirection_OUTBOUND)

		filterChain, err := s.makeUpstreamFilterChainForDiscoveryChain(
			id,
			"",
			cfg.Protocol,
			u,
			nil,
			cfgSnap,
			nil,
		)
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
		for _, check := range s.CheckFetcher.ServiceHTTPBasedChecks(psid) {
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
		clusterName := LocalAppClusterName
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
			PrefixLen:     &wrappers.UInt32Value{Value: pfxLen},
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

func (s *ResourceGenerator) makeIngressGatewayListeners(address string, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message

	for listenerKey, upstreams := range cfgSnap.IngressGateway.Upstreams {
		var tlsContext *envoy_tls_v3.DownstreamTlsContext
		if cfgSnap.IngressGateway.TLSEnabled {
			tlsContext = &envoy_tls_v3.DownstreamTlsContext{
				CommonTlsContext:         makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.Leaf()),
				RequireClientCertificate: &wrappers.BoolValue{Value: false},
			}
		}

		if listenerKey.Protocol == "tcp" {
			// We rely on the invariant of upstreams slice always having at least 1
			// member, because this key/value pair is created only when a
			// GatewayService is returned in the RPC
			u := upstreams[0]
			id := u.Identifier()

			chain := cfgSnap.IngressGateway.DiscoveryChain[id]

			var upstreamListener proto.Message
			upstreamListener, err := s.makeUpstreamListenerForDiscoveryChain(
				&u,
				address,
				chain,
				cfgSnap,
				tlsContext,
			)
			if err != nil {
				return nil, err
			}
			resources = append(resources, upstreamListener)
		} else {
			// If multiple upstreams share this port, make a special listener for the protocol.
			listener := makePortListener(listenerKey.Protocol, address, listenerKey.Port, envoy_core_v3.TrafficDirection_OUTBOUND)
			opts := listenerFilterOpts{
				useRDS:          true,
				protocol:        listenerKey.Protocol,
				filterName:      listenerKey.RouteName(),
				routeName:       listenerKey.RouteName(),
				cluster:         "",
				statPrefix:      "ingress_upstream_",
				routePath:       "",
				httpAuthzFilter: nil,
			}
			filter, err := makeListenerFilter(opts)
			if err != nil {
				return nil, err
			}

			transportSocket, err := makeDownstreamTLSTransportSocket(tlsContext)
			if err != nil {
				return nil, err
			}

			listener.FilterChains = []*envoy_listener_v3.FilterChain{
				{
					Filters: []*envoy_listener_v3.Filter{
						filter,
					},
					TransportSocket: transportSocket,
				},
			}
			resources = append(resources, listener)
		}
	}

	return resources, nil
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
func makeListener(name string, upstream *structs.Upstream, trafficDirection envoy_core_v3.TrafficDirection) *envoy_listener_v3.Listener {
	if upstream.LocalBindPort == 0 && upstream.LocalBindSocketPath != "" {
		return makePipeListener(name, upstream.LocalBindSocketPath, upstream.LocalBindSocketMode, trafficDirection)
	}

	return makePortListenerWithDefault(name, upstream.LocalBindAddress, upstream.LocalBindPort, trafficDirection)
}

func makePortListener(name, addr string, port int, trafficDirection envoy_core_v3.TrafficDirection) *envoy_listener_v3.Listener {
	return &envoy_listener_v3.Listener{
		Name:             fmt.Sprintf("%s:%s:%d", name, addr, port),
		Address:          makeAddress(addr, port),
		TrafficDirection: trafficDirection,
	}
}

func makePortListenerWithDefault(name, addr string, port int, trafficDirection envoy_core_v3.TrafficDirection) *envoy_listener_v3.Listener {
	if addr == "" {
		addr = "127.0.0.1"
	}
	return makePortListener(name, addr, port, trafficDirection)
}

func makePipeListener(name, path string, mode_str string, trafficDirection envoy_core_v3.TrafficDirection) *envoy_listener_v3.Listener {
	// We've already validated this, so it should not fail.
	mode, err := strconv.ParseUint(mode_str, 0, 32)
	if err != nil {
		mode = 0
	}
	return &envoy_listener_v3.Listener{
		Name:             fmt.Sprintf("%s:%s", name, path),
		Address:          makePipeAddress(path, uint32(mode)),
		TrafficDirection: trafficDirection,
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
	// Type field is present so decode it as a any.Any
	var any any.Any
	if err := jsonpb.UnmarshalString(configJSON, &any); err != nil {
		return nil, err
	}
	var l envoy_listener_v3.Listener
	if err := proto.Unmarshal(any.Value, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

// Ensure that the first filter in each filter chain of a public listener is
// the authz filter to prevent unauthorized access.
func (s *ResourceGenerator) injectConnectFilters(cfgSnap *proxycfg.ConfigSnapshot, listener *envoy_listener_v3.Listener) error {
	authzFilter, err := makeRBACNetworkFilter(
		cfgSnap.ConnectProxy.Intentions,
		cfgSnap.IntentionDefaultAllow,
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
			if err := ptypes.UnmarshalAny(tc.TypedConfig, &hcm); err != nil {
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

		var (
			hcm envoy_http_v3.HttpConnectionManager
		)
		tc, ok := hcmFilter.ConfigType.(*envoy_listener_v3.Filter_TypedConfig)
		if !ok {
			return fmt.Errorf(
				"filter chain %d has a %q filter with an unsupported config type: %T",
				chainIdx,
				hcmFilter.Name,
				hcmFilter.ConfigType,
			)
		}

		if err := ptypes.UnmarshalAny(tc.TypedConfig, &hcm); err != nil {
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

// Ensure every filter chain uses our TLS certs. We might allow users to work
// around this later if there is a good use case but this is actually a feature
// for now as it allows them to specify custom listener params in config but
// still get our certs delivered dynamically and intentions enforced without
// coming up with some complicated templating/merging solution.
func (s *ResourceGenerator) injectConnectTLSOnFilterChains(cfgSnap *proxycfg.ConfigSnapshot, listener *envoy_listener_v3.Listener) error {
	for idx := range listener.FilterChains {
		tlsContext := &envoy_tls_v3.DownstreamTlsContext{
			CommonTlsContext:         makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.Leaf()),
			RequireClientCertificate: &wrappers.BoolValue{Value: true},
		}
		transportSocket, err := makeDownstreamTLSTransportSocket(tlsContext)
		if err != nil {
			return err
		}
		listener.FilterChains[idx].TransportSocket = transportSocket
	}
	return nil
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

	l = makePortListener(name, addr, port, envoy_core_v3.TrafficDirection_INBOUND)

	filterOpts := listenerFilterOpts{
		protocol:         cfg.Protocol,
		filterName:       name,
		routeName:        name,
		cluster:          LocalAppClusterName,
		requestTimeoutMs: cfg.LocalRequestTimeoutMs,
	}
	if useHTTPFilter {
		filterOpts.httpAuthzFilter, err = makeRBACHTTPFilter(
			cfgSnap.ConnectProxy.Intentions,
			cfgSnap.IntentionDefaultAllow,
		)
		if err != nil {
			return nil, err
		}
	}
	filter, err := makeListenerFilter(filterOpts)
	if err != nil {
		return nil, err
	}
	l.FilterChains = []*envoy_listener_v3.FilterChain{
		{
			Filters: []*envoy_listener_v3.Filter{
				filter,
			},
		},
	}

	err = s.finalizePublicListenerFromConfig(l, cfgSnap, useHTTPFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to attach Consul filters and TLS context to custom public listener: %v", err)
	}

	return l, err
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
	if err := s.injectConnectTLSOnFilterChains(cfgSnap, l); err != nil {
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

	l := makePortListener(listenerName, addr, path.ListenerPort, envoy_core_v3.TrafficDirection_INBOUND)

	filterName := fmt.Sprintf("exposed_path_filter_%s_%d", strippedPath, path.ListenerPort)

	opts := listenerFilterOpts{
		useRDS:          false,
		protocol:        path.Protocol,
		filterName:      filterName,
		routeName:       filterName,
		cluster:         cluster,
		statPrefix:      "",
		routePath:       path.Path,
		httpAuthzFilter: nil,
	}
	f, err := makeListenerFilter(opts)
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
			&envoy_core_v3.CidrRange{AddressPrefix: "127.0.0.1", PrefixLen: &wrappers.UInt32Value{Value: 8}},
			&envoy_core_v3.CidrRange{AddressPrefix: advertise, PrefixLen: &wrappers.UInt32Value{Value: uint32(advertiseLen)}},
		)

		if ok, err := kernelSupportsIPv6(); err != nil {
			return nil, err
		} else if ok {
			ranges = append(ranges,
				&envoy_core_v3.CidrRange{AddressPrefix: "::1", PrefixLen: &wrappers.UInt32Value{Value: 128}},
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
	l := makePortListener(name, addr, port, envoy_core_v3.TrafficDirection_INBOUND)

	tlsInspector, err := makeTLSInspectorListenerFilter()
	if err != nil {
		return nil, err
	}
	l.ListenerFilters = []*envoy_listener_v3.ListenerFilter{tlsInspector}

	// Make a FilterChain for each linked service
	// Match on the cluster name,
	for _, svc := range cfgSnap.TerminatingGateway.ValidServices() {
		clusterName := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

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

		clusterChain, err := s.makeFilterChainTerminatingGateway(
			cfgSnap,
			clusterName,
			svc,
			intentions,
			cfg.Protocol,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to make filter chain for cluster %q: %v", clusterName, err)
		}
		l.FilterChains = append(l.FilterChains, clusterChain)

		// if there is a service-resolver for this service then also setup subset filter chains for it
		if hasResolver {
			// generate 1 filter chain for each service subset
			for subsetName := range resolver.Subsets {
				subsetClusterName := connect.ServiceSNI(svc.Name, subsetName, svc.NamespaceOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

				subsetClusterChain, err := s.makeFilterChainTerminatingGateway(
					cfgSnap,
					subsetClusterName,
					svc,
					intentions,
					cfg.Protocol,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to make filter chain for cluster %q: %v", subsetClusterName, err)
				}
				l.FilterChains = append(l.FilterChains, subsetClusterChain)
			}
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
	tcpProxy, err := makeTCPProxyFilter(name, "", "terminating_gateway.")
	if err != nil {
		return nil, err
	}
	fallback := &envoy_listener_v3.FilterChain{
		Filters: []*envoy_listener_v3.Filter{
			{Name: "envoy.filters.network.sni_cluster"},
			tcpProxy,
		},
	}
	l.FilterChains = append(l.FilterChains, fallback)

	return l, nil
}

func (s *ResourceGenerator) makeFilterChainTerminatingGateway(
	cfgSnap *proxycfg.ConfigSnapshot,
	cluster string,
	service structs.ServiceName,
	intentions structs.Intentions,
	protocol string,
) (*envoy_listener_v3.FilterChain, error) {
	tlsContext := &envoy_tls_v3.DownstreamTlsContext{
		CommonTlsContext:         makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.TerminatingGateway.ServiceLeaves[service]),
		RequireClientCertificate: &wrappers.BoolValue{Value: true},
	}
	transportSocket, err := makeDownstreamTLSTransportSocket(tlsContext)
	if err != nil {
		return nil, err
	}

	filterChain := &envoy_listener_v3.FilterChain{
		FilterChainMatch: makeSNIFilterChainMatch(cluster),
		Filters:          make([]*envoy_listener_v3.Filter, 0, 3),
		TransportSocket:  transportSocket,
	}

	// This controls if we do L4 or L7 intention checks.
	useHTTPFilter := structs.IsProtocolHTTPLike(protocol)

	// If this is L4, the first filter we setup is to do intention checks.
	if !useHTTPFilter {
		authFilter, err := makeRBACNetworkFilter(
			intentions,
			cfgSnap.IntentionDefaultAllow,
		)
		if err != nil {
			return nil, err
		}
		filterChain.Filters = append(filterChain.Filters, authFilter)
	}

	// Lastly we setup the actual proxying component. For L4 this is a straight
	// tcp proxy. For L7 this is a very hands-off HTTP proxy just to inject an
	// HTTP filter to do intention checks here instead.
	opts := listenerFilterOpts{
		protocol:   protocol,
		filterName: fmt.Sprintf("%s.%s.%s", service.Name, service.NamespaceOrDefault(), cfgSnap.Datacenter),
		routeName:  cluster, // Set cluster name for route config since each will have its own
		cluster:    cluster,
		statPrefix: "upstream.",
		routePath:  "",
	}

	if useHTTPFilter {
		var err error
		opts.httpAuthzFilter, err = makeRBACHTTPFilter(
			intentions,
			cfgSnap.IntentionDefaultAllow,
		)
		if err != nil {
			return nil, err
		}

		opts.cluster = ""
		opts.useRDS = true
	}

	filter, err := makeListenerFilter(opts)
	if err != nil {
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
	tcpProxy, err := makeTCPProxyFilter(name, "", "mesh_gateway_local.")
	if err != nil {
		return nil, err
	}

	sniClusterChain := &envoy_listener_v3.FilterChain{
		Filters: []*envoy_listener_v3.Filter{
			sniCluster,
			tcpProxy,
		},
	}

	l := makePortListener(name, addr, port, envoy_core_v3.TrafficDirection_UNSPECIFIED)
	l.ListenerFilters = []*envoy_listener_v3.ListenerFilter{tlsInspector}

	// TODO (mesh-gateway) - Do we need to create clusters for all the old trust domains as well?
	// We need 1 Filter Chain per datacenter
	datacenters := cfgSnap.MeshGateway.Datacenters()
	for _, dc := range datacenters {
		if dc == cfgSnap.Datacenter {
			continue // skip local
		}
		clusterName := connect.DatacenterSNI(dc, cfgSnap.Roots.TrustDomain)
		filterName := fmt.Sprintf("%s.%s", name, dc)
		dcTCPProxy, err := makeTCPProxyFilter(filterName, clusterName, "mesh_gateway_remote.")
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

	if cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" && cfgSnap.ServerSNIFn != nil {
		for _, dc := range datacenters {
			if dc == cfgSnap.Datacenter {
				continue // skip local
			}
			clusterName := cfgSnap.ServerSNIFn(dc, "")
			filterName := fmt.Sprintf("%s.%s", name, dc)
			dcTCPProxy, err := makeTCPProxyFilter(filterName, clusterName, "mesh_gateway_remote.")
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
		for _, srv := range cfgSnap.MeshGateway.ConsulServers {
			clusterName := cfgSnap.ServerSNIFn(cfgSnap.Datacenter, srv.Node.Node)

			filterName := fmt.Sprintf("%s.%s", name, cfgSnap.Datacenter)
			dcTCPProxy, err := makeTCPProxyFilter(filterName, clusterName, "mesh_gateway_local_server.")
			if err != nil {
				return nil, err
			}

			l.FilterChains = append(l.FilterChains, &envoy_listener_v3.FilterChain{
				FilterChainMatch: &envoy_listener_v3.FilterChainMatch{
					ServerNames: []string{fmt.Sprintf("%s", clusterName)},
				},
				Filters: []*envoy_listener_v3.Filter{
					dcTCPProxy,
				},
			})
		}
	}

	// This needs to get tacked on at the end as it has no
	// matching and will act as a catch all
	l.FilterChains = append(l.FilterChains, sniClusterChain)

	return l, nil
}

func (s *ResourceGenerator) makeUpstreamFilterChainForDiscoveryChain(
	id string,
	overrideCluster string,
	protocol string,
	u *structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
	tlsContext *envoy_tls_v3.DownstreamTlsContext,
) (*envoy_listener_v3.FilterChain, error) {
	// TODO (freddy) Make this actually legible
	useRDS := true

	var (
		clusterName                        string
		destination, datacenter, namespace string
	)

	if chain != nil {
		destination, datacenter, namespace = chain.ServiceName, chain.Datacenter, chain.Namespace
	}
	if (chain == nil || chain.IsDefault()) && u != nil {
		useRDS = false

		if datacenter == "" {
			datacenter = u.Datacenter
		}
		if datacenter == "" {
			datacenter = cfgSnap.Datacenter
		}
		if destination == "" {
			destination = u.DestinationName
		}
		if namespace == "" {
			namespace = u.DestinationNamespace
		}

		sni := connect.UpstreamSNI(u, "", datacenter, cfgSnap.Roots.TrustDomain)
		clusterName = CustomizeClusterName(sni, chain)

	} else {
		if protocol == "tcp" && chain != nil {
			useRDS = false

			startNode := chain.Nodes[chain.StartNode]
			if startNode == nil {
				return nil, fmt.Errorf("missing first node in compiled discovery chain for: %s", chain.ServiceName)
			}
			if startNode.Type != structs.DiscoveryGraphNodeTypeResolver {
				return nil, fmt.Errorf("unexpected first node in discovery chain using protocol=%q: %s", protocol, startNode.Type)
			}
			targetID := startNode.Resolver.Target
			target := chain.Targets[targetID]

			clusterName = CustomizeClusterName(target.Name, chain)
		}
	}

	// Default the namespace to match how SNIs are generated
	if namespace == "" {
		namespace = structs.IntentionDefaultNamespace
	}

	filterName := fmt.Sprintf("%s.%s.%s", destination, namespace, datacenter)
	if u != nil && u.DestinationType == structs.UpstreamDestTypePreparedQuery {
		// Avoid encoding dc and namespace for prepared queries.
		// Those are defined in the query itself and are not available here.
		filterName = id
	}
	if overrideCluster != "" {
		useRDS = false
		clusterName = overrideCluster

		if destination == "" {
			filterName = overrideCluster
		}
	}

	opts := listenerFilterOpts{
		useRDS:          useRDS,
		protocol:        protocol,
		filterName:      filterName,
		routeName:       id,
		cluster:         clusterName,
		statPrefix:      "upstream.",
		routePath:       "",
		ingressGateway:  false,
		httpAuthzFilter: nil,
	}
	filter, err := makeListenerFilter(opts)
	if err != nil {
		return nil, err
	}
	transportSocket, err := makeDownstreamTLSTransportSocket(tlsContext)
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

// TODO(freddy) Replace in favor of new function above. Currently in use for ingress gateways.
func (s *ResourceGenerator) makeUpstreamListenerForDiscoveryChain(
	u *structs.Upstream,
	address string,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
	tlsContext *envoy_tls_v3.DownstreamTlsContext,
) (proto.Message, error) {

	// Best understanding is this only makes sense for port listeners....
	if u.LocalBindSocketPath != "" {
		return nil, fmt.Errorf("makeUpstreamListenerForDiscoveryChain not supported for unix domain sockets %s %+v",
			address, u)
	}

	upstreamID := u.Identifier()
	l := makePortListenerWithDefault(upstreamID, address, u.LocalBindPort, envoy_core_v3.TrafficDirection_OUTBOUND)
	cfg := s.getAndModifyUpstreamConfigForListener(upstreamID, u, chain)
	if cfg.EnvoyListenerJSON != "" {
		return makeListenerFromUserConfig(cfg.EnvoyListenerJSON)
	}

	useRDS := true
	var (
		clusterName                        string
		destination, datacenter, namespace string
	)
	if chain == nil || chain.IsDefault() {
		useRDS = false

		dc := u.Datacenter
		if dc == "" {
			dc = cfgSnap.Datacenter
		}
		destination, datacenter, namespace = u.DestinationName, dc, u.DestinationNamespace

		sni := connect.UpstreamSNI(u, "", dc, cfgSnap.Roots.TrustDomain)
		clusterName = CustomizeClusterName(sni, chain)

	} else {
		destination, datacenter, namespace = chain.ServiceName, chain.Datacenter, chain.Namespace

		if cfg.Protocol == "tcp" {
			useRDS = false

			startNode := chain.Nodes[chain.StartNode]
			if startNode == nil {
				return nil, fmt.Errorf("missing first node in compiled discovery chain for: %s", chain.ServiceName)
			}
			if startNode.Type != structs.DiscoveryGraphNodeTypeResolver {
				return nil, fmt.Errorf("unexpected first node in discovery chain using protocol=%q: %s", cfg.Protocol, startNode.Type)
			}
			targetID := startNode.Resolver.Target
			target := chain.Targets[targetID]

			clusterName = CustomizeClusterName(target.Name, chain)
		}
	}

	// Default the namespace to match how SNIs are generated
	if namespace == "" {
		namespace = structs.IntentionDefaultNamespace
	}
	filterName := fmt.Sprintf("%s.%s.%s", destination, namespace, datacenter)

	if u.DestinationType == structs.UpstreamDestTypePreparedQuery {
		// Avoid encoding dc and namespace for prepared queries.
		// Those are defined in the query itself and are not available here.
		filterName = upstreamID
	}

	opts := listenerFilterOpts{
		useRDS:          useRDS,
		protocol:        cfg.Protocol,
		filterName:      filterName,
		routeName:       upstreamID,
		cluster:         clusterName,
		statPrefix:      "upstream.",
		routePath:       "",
		httpAuthzFilter: nil,
	}
	filter, err := makeListenerFilter(opts)
	if err != nil {
		return nil, err
	}

	transportSocket, err := makeDownstreamTLSTransportSocket(tlsContext)
	if err != nil {
		return nil, err
	}

	l.FilterChains = []*envoy_listener_v3.FilterChain{
		{
			Filters: []*envoy_listener_v3.Filter{
				filter,
			},
			TransportSocket: transportSocket,
		},
	}
	return l, nil
}

func (s *ResourceGenerator) getAndModifyUpstreamConfigForListener(id string, u *structs.Upstream, chain *structs.CompiledDiscoveryChain) structs.UpstreamConfig {
	var (
		cfg structs.UpstreamConfig
		err error
	)

	configMap := make(map[string]interface{})
	if u != nil {
		configMap = u.Config
	}
	if chain == nil || chain.IsDefault() {
		cfg, err = structs.ParseUpstreamConfig(configMap)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn("failed to parse", "upstream", id, "error", err)
		}
	} else {
		// Use NoDefaults here so that we can set the protocol to the chain
		// protocol if necessary
		cfg, err = structs.ParseUpstreamConfigNoDefaults(configMap)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			s.Logger.Warn("failed to parse", "upstream", id, "error", err)
		}

		if cfg.EnvoyListenerJSON != "" {
			s.Logger.Warn("ignoring escape hatch setting because already configured for",
				"discovery chain", chain.ServiceName, "upstream", id, "config", "envoy_listener_json")

			// Remove from config struct so we don't use it later on
			cfg.EnvoyListenerJSON = ""
		}

		protocol := cfg.Protocol
		if protocol == "" {
			protocol = chain.Protocol
		}
		if protocol == "" {
			protocol = "tcp"
		}

		// set back on the config so that we can use it from return value
		cfg.Protocol = protocol
	}

	return cfg
}

type listenerFilterOpts struct {
	useRDS           bool
	protocol         string
	filterName       string
	routeName        string
	cluster          string
	statPrefix       string
	routePath        string
	requestTimeoutMs *int
	ingressGateway   bool
	httpAuthzFilter  *envoy_http_v3.HttpFilter
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
		return makeTCPProxyFilter(opts.filterName, opts.cluster, opts.statPrefix)
	}
}

func makeTLSInspectorListenerFilter() (*envoy_listener_v3.ListenerFilter, error) {
	return &envoy_listener_v3.ListenerFilter{Name: "envoy.filters.listener.tls_inspector"}, nil
}

func makeSNIFilterChainMatch(sniMatch string) *envoy_listener_v3.FilterChainMatch {
	return &envoy_listener_v3.FilterChainMatch{
		ServerNames: []string{sniMatch},
	}
}

func makeSNIClusterFilter() (*envoy_listener_v3.Filter, error) {
	// This filter has no config which is why we are not calling make
	return &envoy_listener_v3.Filter{Name: "envoy.filters.network.sni_cluster"}, nil
}

func makeTCPProxyFilter(filterName, cluster, statPrefix string) (*envoy_listener_v3.Filter, error) {
	cfg := &envoy_tcp_proxy_v3.TcpProxy{
		StatPrefix:       makeStatPrefix(statPrefix, filterName),
		ClusterSpecifier: &envoy_tcp_proxy_v3.TcpProxy_Cluster{Cluster: cluster},
	}
	return makeFilter("envoy.filters.network.tcp_proxy", cfg)
}

func makeStatPrefix(prefix, filterName string) string {
	// Replace colons here because Envoy does that in the metrics for the actual
	// clusters but doesn't in the stat prefix here while dashboards assume they
	// will match.
	return fmt.Sprintf("%s%s", prefix, strings.Replace(filterName, ":", "_", -1))
}

func makeHTTPFilter(opts listenerFilterOpts) (*envoy_listener_v3.Filter, error) {
	cfg := &envoy_http_v3.HttpConnectionManager{
		StatPrefix: makeStatPrefix(opts.statPrefix, opts.filterName),
		CodecType:  envoy_http_v3.HttpConnectionManager_AUTO,
		HttpFilters: []*envoy_http_v3.HttpFilter{
			{
				Name: "envoy.filters.http.router",
			},
		},
		Tracing: &envoy_http_v3.HttpConnectionManager_Tracing{
			// Don't trace any requests by default unless the client application
			// explicitly propagates trace headers that indicate this should be
			// sampled.
			RandomSampling: &envoy_type_v3.Percent{Value: 0.0},
		},
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
			r.Timeout = ptypes.DurationProto(time.Duration(*opts.requestTimeoutMs) * time.Millisecond)
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

	// Like injectConnectFilters for L4, here we ensure that the first filter
	// (other than the "envoy.grpc_http1_bridge" filter) in the http filter
	// chain of a public listener is the authz filter to prevent unauthorized
	// access and that every filter chain uses our TLS certs.
	if opts.httpAuthzFilter != nil {
		cfg.HttpFilters = append([]*envoy_http_v3.HttpFilter{opts.httpAuthzFilter}, cfg.HttpFilters...)
	}

	if opts.protocol == "grpc" {
		// Add grpc bridge before router and authz
		cfg.HttpFilters = append([]*envoy_http_v3.HttpFilter{{
			Name: "envoy.filters.http.grpc_http1_bridge",
		}}, cfg.HttpFilters...)

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
		cfg.HttpFilters = append([]*envoy_http_v3.HttpFilter{
			grpcStatsFilter,
		}, cfg.HttpFilters...)
	}

	return makeFilter("envoy.filters.network.http_connection_manager", cfg)
}

func makeFilter(name string, cfg proto.Message) (*envoy_listener_v3.Filter, error) {
	any, err := ptypes.MarshalAny(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_listener_v3.Filter{
		Name:       name,
		ConfigType: &envoy_listener_v3.Filter_TypedConfig{TypedConfig: any},
	}, nil
}

func makeEnvoyHTTPFilter(name string, cfg proto.Message) (*envoy_http_v3.HttpFilter, error) {
	any, err := ptypes.MarshalAny(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_http_v3.HttpFilter{
		Name:       name,
		ConfigType: &envoy_http_v3.HttpFilter_TypedConfig{TypedConfig: any},
	}, nil
}

func makeCommonTLSContextFromLeaf(cfgSnap *proxycfg.ConfigSnapshot, leaf *structs.IssuedCert) *envoy_tls_v3.CommonTlsContext {
	// Concatenate all the root PEMs into one.
	if cfgSnap.Roots == nil {
		return nil
	}

	// TODO(banks): verify this actually works with Envoy (docs are not clear).
	rootPEMS := ""
	for _, root := range cfgSnap.Roots.Roots {
		rootPEMS += root.RootCert
	}

	return &envoy_tls_v3.CommonTlsContext{
		TlsParams: &envoy_tls_v3.TlsParameters{},
		TlsCertificates: []*envoy_tls_v3.TlsCertificate{
			{
				CertificateChain: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: leaf.CertPEM,
					},
				},
				PrivateKey: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: leaf.PrivateKeyPEM,
					},
				},
			},
		},
		ValidationContextType: &envoy_tls_v3.CommonTlsContext_ValidationContext{
			ValidationContext: &envoy_tls_v3.CertificateValidationContext{
				// TODO(banks): later for L7 support we may need to configure ALPN here.
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: rootPEMS,
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
	any, err := ptypes.MarshalAny(config)
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
