package xdsv2

import (
	"fmt"
	"sort"
	"strconv"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	envoy_grpc_http1_bridge_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_http1_bridge/v3"
	envoy_grpc_stats_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_stats/v3"
	envoy_http_router_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	envoy_extensions_filters_listener_http_inspector_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/http_inspector/v3"
	envoy_original_dst_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/original_dst/v3"
	envoy_tls_inspector_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	envoy_connection_limit_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/connection_limit/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_network_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/rbac/v3"
	envoy_sni_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/sni_cluster/v3"
	envoy_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

const (
	envoyNetworkFilterName                     = "envoy.filters.network.tcp_proxy"
	envoyOriginalDestinationListenerFilterName = "envoy.filters.listener.original_dst"
	envoyTLSInspectorListenerFilterName        = "envoy.filters.listener.tls_inspector"
	envoyHttpInspectorListenerFilterName       = "envoy.filters.listener.http_inspector"
	envoyHttpConnectionManagerFilterName       = "envoy.filters.network.http_connection_manager"
)

func (pr *ProxyResources) makeListener(listener *pbproxystate.Listener) (*envoy_listener_v3.Listener, error) {
	envoyListener := &envoy_listener_v3.Listener{}

	// Listener Address
	var address *envoy_core_v3.Address
	switch listener.BindAddress.(type) {
	case *pbproxystate.Listener_HostPort:
		address = makeIpPortEnvoyAddress(listener.BindAddress.(*pbproxystate.Listener_HostPort))
	case *pbproxystate.Listener_UnixSocket:
		address = makeUnixSocketEnvoyAddress(listener.BindAddress.(*pbproxystate.Listener_UnixSocket))
	default:
		// This should be impossible to reach because we're using protobufs.
		return nil, fmt.Errorf("invalid listener bind address type: %t", listener.BindAddress)
	}
	envoyListener.Address = address

	// Listener Direction
	var direction envoy_core_v3.TrafficDirection
	switch listener.Direction {
	case pbproxystate.Direction_DIRECTION_OUTBOUND:
		direction = envoy_core_v3.TrafficDirection_OUTBOUND
	case pbproxystate.Direction_DIRECTION_INBOUND:
		direction = envoy_core_v3.TrafficDirection_INBOUND
	case pbproxystate.Direction_DIRECTION_UNSPECIFIED:
		direction = envoy_core_v3.TrafficDirection_UNSPECIFIED
	default:
		return nil, fmt.Errorf("no direction for listener %+v", listener.Name)
	}
	envoyListener.TrafficDirection = direction

	// Before creating the filter chains, sort routers by match to avoid draining if the list is provided out of order.
	sortRouters(listener.Routers)

	// Listener filter chains
	for _, r := range listener.Routers {
		filterChain, err := pr.makeEnvoyListenerFilterChain(r)
		if err != nil {
			return nil, fmt.Errorf("could not make filter chain: %w", err)
		}
		envoyListener.FilterChains = append(envoyListener.FilterChains, filterChain)
	}

	if listener.DefaultRouter != nil {
		defaultFilterChain, err := pr.makeEnvoyListenerFilterChain(listener.DefaultRouter)
		if err != nil {
			return nil, fmt.Errorf("could not make filter chain: %w", err)
		}
		envoyListener.DefaultFilterChain = defaultFilterChain
	}

	// Envoy builtin listener filters
	for _, c := range listener.Capabilities {
		listenerFilter, err := makeEnvoyListenerFilter(c)
		if err != nil {
			return nil, fmt.Errorf("could not make listener filter: %w", err)
		}
		envoyListener.ListenerFilters = append(envoyListener.ListenerFilters, listenerFilter)
	}

	err := addEnvoyListenerConnectionBalanceConfig(listener.BalanceConnections, envoyListener)
	if err != nil {
		return nil, err
	}

	envoyListener.Name = listener.Name
	envoyListener.Address = address
	envoyListener.TrafficDirection = direction

	return envoyListener, nil
}

func makeEnvoyConnectionLimitFilter(maxInboundConns uint64) (*envoy_listener_v3.Filter, error) {
	cfg := &envoy_connection_limit_v3.ConnectionLimit{
		StatPrefix:     "inbound_connection_limit",
		MaxConnections: wrapperspb.UInt64(maxInboundConns),
	}

	return makeEnvoyFilter("envoy.filters.network.connection_limit", cfg)
}

func addEnvoyListenerConnectionBalanceConfig(balanceType pbproxystate.BalanceConnections, listener *envoy_listener_v3.Listener) error {
	switch balanceType {
	case pbproxystate.BalanceConnections_BALANCE_CONNECTIONS_DEFAULT:
		// Default with no balancing.
		return nil
	case pbproxystate.BalanceConnections_BALANCE_CONNECTIONS_EXACT:
		listener.ConnectionBalanceConfig = &envoy_listener_v3.Listener_ConnectionBalanceConfig{
			BalanceType: &envoy_listener_v3.Listener_ConnectionBalanceConfig_ExactBalance_{},
		}
		return nil
	default:
		// This should be impossible using protobufs.
		return fmt.Errorf("unsupported connection balance option: %+v", balanceType)
	}
}

func makeIpPortEnvoyAddress(address *pbproxystate.Listener_HostPort) *envoy_core_v3.Address {
	return &envoy_core_v3.Address{
		Address: &envoy_core_v3.Address_SocketAddress{
			SocketAddress: &envoy_core_v3.SocketAddress{
				Address: address.HostPort.Host,
				PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
					PortValue: address.HostPort.Port,
				},
			},
		},
	}
}

func makeUnixSocketEnvoyAddress(address *pbproxystate.Listener_UnixSocket) *envoy_core_v3.Address {
	modeInt, err := strconv.ParseUint(address.UnixSocket.Mode, 0, 32)
	if err != nil {
		modeInt = 0
	}
	return &envoy_core_v3.Address{
		Address: &envoy_core_v3.Address_Pipe{
			Pipe: &envoy_core_v3.Pipe{
				Path: address.UnixSocket.Path,
				Mode: uint32(modeInt),
			},
		},
	}
}

func (pr *ProxyResources) makeEnvoyListenerFilterChain(router *pbproxystate.Router) (*envoy_listener_v3.FilterChain, error) {
	envoyFilterChain := &envoy_listener_v3.FilterChain{}

	if router == nil {
		return nil, fmt.Errorf("no router to create filter chain")
	}

	// Router Match
	match := makeEnvoyFilterChainMatch(router.Match)
	if match != nil {
		envoyFilterChain.FilterChainMatch = match
	}

	// Router Destination
	var envoyFilters []*envoy_listener_v3.Filter
	switch router.Destination.(type) {
	case *pbproxystate.Router_L4:
		l4Filters, err := pr.makeEnvoyResourcesForL4Destination(router.Destination.(*pbproxystate.Router_L4))
		if err != nil {
			return nil, err
		}
		envoyFilters = append(envoyFilters, l4Filters...)
	case *pbproxystate.Router_L7:
		l7 := router.Destination.(*pbproxystate.Router_L7)
		l7Filters, err := pr.makeEnvoyResourcesForL7Destination(l7)
		if err != nil {
			return nil, err
		}

		// Inject ALPN protocols to router's TLS if destination is L7
		if router.InboundTls != nil {
			router.InboundTls.AlpnProtocols = getAlpnProtocols(l7.L7.Protocol)
		}
		envoyFilters = append(envoyFilters, l7Filters...)
	case *pbproxystate.Router_Sni:
		sniFilters, err := pr.makeEnvoyResourcesForSNIDestination(router.Destination.(*pbproxystate.Router_Sni))
		if err != nil {
			return nil, err
		}
		envoyFilters = append(envoyFilters, sniFilters...)
	default:
		// This should be impossible using protobufs.
		return nil, fmt.Errorf("unsupported destination type: %t", router.Destination)

	}

	// Router TLS
	ts, err := pr.makeEnvoyTransportSocket(router.InboundTls)
	if err != nil {
		return nil, err
	}
	envoyFilterChain.TransportSocket = ts

	envoyFilterChain.Filters = envoyFilters
	return envoyFilterChain, err
}

func makeEnvoyFilterChainMatch(routerMatch *pbproxystate.Match) *envoy_listener_v3.FilterChainMatch {
	var envoyFilterChainMatch *envoy_listener_v3.FilterChainMatch
	if routerMatch != nil {
		envoyFilterChainMatch = &envoy_listener_v3.FilterChainMatch{}

		envoyFilterChainMatch.DestinationPort = routerMatch.DestinationPort

		if len(routerMatch.ServerNames) > 0 {
			var serverNames []string
			for _, n := range routerMatch.ServerNames {
				serverNames = append(serverNames, n)
			}
			envoyFilterChainMatch.ServerNames = serverNames
		}
		if len(routerMatch.PrefixRanges) > 0 {
			sortPrefixRanges(routerMatch.PrefixRanges)
			var ranges []*envoy_core_v3.CidrRange
			for _, r := range routerMatch.PrefixRanges {
				cidrRange := &envoy_core_v3.CidrRange{
					PrefixLen:     r.PrefixLen,
					AddressPrefix: r.AddressPrefix,
				}
				ranges = append(ranges, cidrRange)
			}
			envoyFilterChainMatch.PrefixRanges = ranges
		}
		if len(routerMatch.SourcePrefixRanges) > 0 {
			var ranges []*envoy_core_v3.CidrRange
			for _, r := range routerMatch.SourcePrefixRanges {
				cidrRange := &envoy_core_v3.CidrRange{
					PrefixLen:     r.PrefixLen,
					AddressPrefix: r.AddressPrefix,
				}
				ranges = append(ranges, cidrRange)
			}
			envoyFilterChainMatch.SourcePrefixRanges = ranges
		}
	}
	return envoyFilterChainMatch
}

func (pr *ProxyResources) makeEnvoyResourcesForSNIDestination(sni *pbproxystate.Router_Sni) ([]*envoy_listener_v3.Filter, error) {
	var envoyFilters []*envoy_listener_v3.Filter
	sniFilter, err := makeEnvoyFilter("envoy.filters.network.sni_cluster", &envoy_sni_cluster_v3.SniCluster{})
	if err != nil {
		return nil, err
	}
	tcp := &envoy_tcp_proxy_v3.TcpProxy{
		StatPrefix:       sni.Sni.StatPrefix,
		ClusterSpecifier: &envoy_tcp_proxy_v3.TcpProxy_Cluster{Cluster: ""},
	}
	tcpFilter, err := makeEnvoyFilter(envoyNetworkFilterName, tcp)
	if err != nil {
		return nil, err
	}
	envoyFilters = append(envoyFilters, sniFilter, tcpFilter)
	return envoyFilters, err
}

func (pr *ProxyResources) makeEnvoyResourcesForL4Destination(l4 *pbproxystate.Router_L4) ([]*envoy_listener_v3.Filter, error) {
	err := pr.makeCluster(l4.L4.Name)
	if err != nil {
		return nil, err
	}
	envoyFilters, err := makeL4Filters(l4.L4)
	return envoyFilters, err
}

func (pr *ProxyResources) makeEnvoyResourcesForL7Destination(l7 *pbproxystate.Router_L7) ([]*envoy_listener_v3.Filter, error) {
	envoyFilters, err := pr.makeL7Filters(l7.L7)
	if err != nil {
		return nil, err
	}
	return envoyFilters, err
}

func getAlpnProtocols(protocol pbproxystate.L7Protocol) []string {
	var alpnProtocols []string

	switch protocol {
	case pbproxystate.L7Protocol_L7_PROTOCOL_GRPC, pbproxystate.L7Protocol_L7_PROTOCOL_HTTP2:
		alpnProtocols = append(alpnProtocols, "h2", "http/1.1")
	case pbproxystate.L7Protocol_L7_PROTOCOL_HTTP:
		alpnProtocols = append(alpnProtocols, "http/1.1")
	}

	return alpnProtocols
}

func makeL4Filters(l4 *pbproxystate.L4Destination) ([]*envoy_listener_v3.Filter, error) {
	var envoyFilters []*envoy_listener_v3.Filter
	if l4 != nil {
		// Add rbac filter. RBAC filter needs to be added first so any
		// unauthorized connections will get rejected.
		// TODO(proxystate): Intentions will be added in the future.
		if l4.AddEmptyIntention {
			rbacFilter, err := makeEmptyRBACNetworkFilter()
			if err != nil {
				return nil, err
			}
			envoyFilters = append(envoyFilters, rbacFilter)
		}

		if l4.MaxInboundConnections > 0 {
			connectionLimitFilter, err := makeEnvoyConnectionLimitFilter(l4.MaxInboundConnections)
			if err != nil {
				return nil, err
			}
			envoyFilters = append(envoyFilters, connectionLimitFilter)
		}

		// Add tcp proxy filter
		tcp := &envoy_tcp_proxy_v3.TcpProxy{
			ClusterSpecifier: &envoy_tcp_proxy_v3.TcpProxy_Cluster{Cluster: l4.Name},
			StatPrefix:       l4.StatPrefix,
		}
		tcpFilter, err := makeEnvoyFilter(envoyNetworkFilterName, tcp)
		if err != nil {
			return nil, err
		}
		envoyFilters = append(envoyFilters, tcpFilter)
	}
	return envoyFilters, nil

}

func makeEmptyRBACNetworkFilter() (*envoy_listener_v3.Filter, error) {
	cfg := &envoy_network_rbac_v3.RBAC{
		StatPrefix: "connect_authz",
		Rules:      &envoy_rbac_v3.RBAC{},
	}
	filter, err := makeEnvoyFilter("envoy.filters.network.rbac", cfg)
	if err != nil {
		return nil, err
	}
	return filter, nil
}

// TODO: Forward client cert details will be added as part of L7 listeners task.
func (pr *ProxyResources) makeL7Filters(l7 *pbproxystate.L7Destination) ([]*envoy_listener_v3.Filter, error) {
	var envoyFilters []*envoy_listener_v3.Filter
	var httpConnMgr *envoy_http_v3.HttpConnectionManager

	if l7 != nil {
		// TODO: Intentions will be added in the future.
		if l7.MaxInboundConnections > 0 {
			connLimitFilter, err := makeEnvoyConnectionLimitFilter(l7.MaxInboundConnections)
			if err != nil {
				return nil, err
			}
			envoyFilters = append(envoyFilters, connLimitFilter)
		}
		envoyHttpRouter, err := makeEnvoyHTTPFilter("envoy.filters.http.router", &envoy_http_router_v3.Router{})
		if err != nil {
			return nil, err
		}

		httpConnMgr = &envoy_http_v3.HttpConnectionManager{
			StatPrefix: l7.StatPrefix,
			CodecType:  envoy_http_v3.HttpConnectionManager_AUTO,
			HttpFilters: []*envoy_http_v3.HttpFilter{
				envoyHttpRouter,
			},
			Tracing: &envoy_http_v3.HttpConnectionManager_Tracing{
				// Don't trace any requests by default unless the client application
				// explicitly propagates trace headers that indicate this should be
				// sampled.
				RandomSampling: &envoy_type_v3.Percent{Value: 0.0},
			},
			// Explicitly enable WebSocket upgrades for all HTTP listeners
			UpgradeConfigs: []*envoy_http_v3.HttpConnectionManager_UpgradeConfig{
				{UpgradeType: "websocket"},
			},
		}

		routeConfig, err := pr.makeRoute(l7.Name)
		if err != nil {
			return nil, err
		}

		if l7.StaticRoute {
			httpConnMgr.RouteSpecifier = &envoy_http_v3.HttpConnectionManager_RouteConfig{
				RouteConfig: routeConfig,
			}
		} else {
			// Add Envoy route under the route resource since it's not inlined.
			pr.envoyResources[xdscommon.RouteType] = append(pr.envoyResources[xdscommon.RouteType], routeConfig)

			httpConnMgr.RouteSpecifier = &envoy_http_v3.HttpConnectionManager_Rds{
				Rds: &envoy_http_v3.Rds{
					RouteConfigName: l7.Name,
					ConfigSource: &envoy_core_v3.ConfigSource{
						ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
						ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
							Ads: &envoy_core_v3.AggregatedConfigSource{},
						},
					},
				},
			}
		}

		// Add http2 protocol options
		if l7.Protocol == pbproxystate.L7Protocol_L7_PROTOCOL_HTTP2 || l7.Protocol == pbproxystate.L7Protocol_L7_PROTOCOL_GRPC {
			httpConnMgr.Http2ProtocolOptions = &envoy_core_v3.Http2ProtocolOptions{}
		}

		// Add grpc envoy http filters.
		if l7.Protocol == pbproxystate.L7Protocol_L7_PROTOCOL_GRPC {
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
						StatsForAllMethods: &wrapperspb.BoolValue{Value: true},
					},
				},
			)
			if err != nil {
				return nil, err
			}

			// Add grpc bridge before envoyRouter and authz, and the stats in front of that.
			httpConnMgr.HttpFilters = append([]*envoy_http_v3.HttpFilter{
				grpcStatsFilter,
				grpcHttp1Bridge,
			}, httpConnMgr.HttpFilters...)
		}

		httpFilter, err := makeEnvoyFilter(envoyHttpConnectionManagerFilterName, httpConnMgr)
		if err != nil {
			return nil, err
		}
		envoyFilters = append(envoyFilters, httpFilter)
	}
	return envoyFilters, nil
}

func (pr *ProxyResources) makeEnvoyTLSParameters(defaultParams *pbproxystate.TLSParameters, overrideParams *pbproxystate.TLSParameters) *envoy_tls_v3.TlsParameters {
	tlsParams := &envoy_tls_v3.TlsParameters{}

	if overrideParams != nil {
		if overrideParams.MinVersion != pbproxystate.TLSVersion_TLS_VERSION_UNSPECIFIED {
			if minVersion, ok := envoyTLSVersions[overrideParams.MinVersion]; ok {
				tlsParams.TlsMinimumProtocolVersion = minVersion
			}
		}
		if overrideParams.MaxVersion != pbproxystate.TLSVersion_TLS_VERSION_UNSPECIFIED {
			if maxVersion, ok := envoyTLSVersions[overrideParams.MaxVersion]; ok {
				tlsParams.TlsMaximumProtocolVersion = maxVersion
			}
		}
		if len(overrideParams.CipherSuites) != 0 {
			tlsParams.CipherSuites = marshalEnvoyTLSCipherSuiteStrings(overrideParams.CipherSuites)
		}
		return tlsParams
	}

	if defaultParams != nil {
		if defaultParams.MinVersion != pbproxystate.TLSVersion_TLS_VERSION_UNSPECIFIED {
			if minVersion, ok := envoyTLSVersions[defaultParams.MinVersion]; ok {
				tlsParams.TlsMinimumProtocolVersion = minVersion
			}
		}
		if defaultParams.MaxVersion != pbproxystate.TLSVersion_TLS_VERSION_UNSPECIFIED {
			if maxVersion, ok := envoyTLSVersions[defaultParams.MaxVersion]; ok {
				tlsParams.TlsMaximumProtocolVersion = maxVersion
			}
		}
		if len(defaultParams.CipherSuites) != 0 {
			tlsParams.CipherSuites = marshalEnvoyTLSCipherSuiteStrings(defaultParams.CipherSuites)
		}
		return tlsParams
	}

	return tlsParams

}

func (pr *ProxyResources) makeEnvoyTransportSocket(ts *pbproxystate.TransportSocket) (*envoy_core_v3.TransportSocket, error) {
	if ts == nil {
		return nil, nil
	}
	commonTLSContext := &envoy_tls_v3.CommonTlsContext{}

	// Create connection TLS. Listeners should only look at inbound TLS.
	switch ts.ConnectionTls.(type) {
	case *pbproxystate.TransportSocket_InboundMesh:
		downstreamContext := &envoy_tls_v3.DownstreamTlsContext{}
		downstreamContext.CommonTlsContext = commonTLSContext
		// Set TLS Parameters.
		tlsParams := pr.makeEnvoyTLSParameters(pr.proxyState.Tls.InboundTlsParameters, ts.TlsParameters)
		commonTLSContext.TlsParams = tlsParams

		// Set the certificate config on the tls context.
		// For inbound mesh, we need to add the identity certificate
		// and the validation context for the mesh depending on the provided trust bundle names.
		if pr.proxyState.Tls == nil {
			// if tls is nil but connection tls is provided, then the proxy state is misconfigured
			return nil, fmt.Errorf("proxyState.Tls is required to generate router's transport socket")
		}
		im := ts.ConnectionTls.(*pbproxystate.TransportSocket_InboundMesh).InboundMesh
		leaf, ok := pr.proxyState.LeafCertificates[im.IdentityKey]
		if !ok {
			return nil, fmt.Errorf("failed to create transport socket: leaf certificate %q not found", im.IdentityKey)
		}
		err := pr.makeEnvoyCertConfig(commonTLSContext, leaf)
		if err != nil {
			return nil, fmt.Errorf("failed to create transport socket: %w", err)
		}

		// Create validation context.
		// When there's only one trust bundle name, we create a simple validation context
		if len(im.ValidationContext.TrustBundlePeerNameKeys) == 1 {
			peerName := im.ValidationContext.TrustBundlePeerNameKeys[0]
			tb, ok := pr.proxyState.TrustBundles[peerName]
			if !ok {
				return nil, fmt.Errorf("failed to create transport socket: provided trust bundle name does not exist in proxystate trust bundle map: %s", peerName)
			}
			commonTLSContext.ValidationContextType = &envoy_tls_v3.CommonTlsContext_ValidationContext{
				ValidationContext: &envoy_tls_v3.CertificateValidationContext{
					// TODO(banks): later for L7 support we may need to configure ALPN here.
					TrustedCa: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: RootPEMsAsString(tb.Roots),
						},
					},
				},
			}
		} else if len(im.ValidationContext.TrustBundlePeerNameKeys) > 1 {
			cfg := &envoy_tls_v3.SPIFFECertValidatorConfig{
				TrustDomains: make([]*envoy_tls_v3.SPIFFECertValidatorConfig_TrustDomain, 0, len(im.ValidationContext.TrustBundlePeerNameKeys)),
			}

			for _, peerName := range im.ValidationContext.TrustBundlePeerNameKeys {
				// Look up the trust bundle ca in the map.
				tb, ok := pr.proxyState.TrustBundles[peerName]
				if !ok {
					return nil, fmt.Errorf("failed to create transport socket: provided bundle name does not exist in trust bundle map: %s", peerName)
				}
				cfg.TrustDomains = append(cfg.TrustDomains, &envoy_tls_v3.SPIFFECertValidatorConfig_TrustDomain{
					Name: tb.TrustDomain,
					TrustBundle: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: RootPEMsAsString(tb.Roots),
						},
					},
				})
			}
			// Sort the trust domains so the output is stable.
			sortTrustDomains(cfg.TrustDomains)

			spiffeConfig, err := anypb.New(cfg)
			if err != nil {
				return nil, err
			}
			commonTLSContext.ValidationContextType = &envoy_tls_v3.CommonTlsContext_ValidationContext{
				ValidationContext: &envoy_tls_v3.CertificateValidationContext{
					CustomValidatorConfig: &envoy_core_v3.TypedExtensionConfig{
						// The typed config name is hard-coded because it is not available as a wellknown var in the control plane lib.
						Name:        "envoy.tls.cert_validator.spiffe",
						TypedConfig: spiffeConfig,
					},
				},
			}
		}
		// Always require client certificate
		downstreamContext.RequireClientCertificate = &wrapperspb.BoolValue{Value: true}
		transportSocket, err := makeTransportSocket("tls", downstreamContext)
		if err != nil {
			return nil, err
		}
		return transportSocket, nil
	case *pbproxystate.TransportSocket_InboundNonMesh:
		downstreamContext := &envoy_tls_v3.DownstreamTlsContext{}
		downstreamContext.CommonTlsContext = commonTLSContext
		// Set TLS Parameters
		tlsParams := pr.makeEnvoyTLSParameters(pr.proxyState.Tls.InboundTlsParameters, ts.TlsParameters)
		commonTLSContext.TlsParams = tlsParams
		// For non-mesh, we don't care about validation context as currently we don't support mTLS for non-mesh connections.
		nonMeshTLS := ts.ConnectionTls.(*pbproxystate.TransportSocket_InboundNonMesh).InboundNonMesh
		err := pr.addNonMeshCertConfig(commonTLSContext, nonMeshTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to create transport socket: %w", err)
		}
		transportSocket, err := makeTransportSocket("tls", downstreamContext)
		if err != nil {
			return nil, err
		}
		return transportSocket, nil
	case *pbproxystate.TransportSocket_OutboundMesh:
		upstreamContext := &envoy_tls_v3.UpstreamTlsContext{}
		upstreamContext.CommonTlsContext = commonTLSContext
		// Set TLS Parameters
		tlsParams := pr.makeEnvoyTLSParameters(pr.proxyState.Tls.OutboundTlsParameters, ts.TlsParameters)
		commonTLSContext.TlsParams = tlsParams
		// For outbound mesh, we need to insert the mesh identity certificate
		// and the validation context for the mesh depending on the provided trust bundle names.
		if pr.proxyState.Tls == nil {
			// if tls is nil but connection tls is provided, then the proxy state is misconfigured
			return nil, fmt.Errorf("proxyState.Tls is required to generate router's transport socket")
		}
		om := ts.ConnectionTls.(*pbproxystate.TransportSocket_OutboundMesh).OutboundMesh
		leaf, ok := pr.proxyState.LeafCertificates[om.IdentityKey]
		if !ok {
			return nil, fmt.Errorf("leaf %s not found in proxyState", om.IdentityKey)
		}
		err := pr.makeEnvoyCertConfig(commonTLSContext, leaf)
		if err != nil {
			return nil, fmt.Errorf("failed to create transport socket: %w", err)
		}

		// Create validation context
		peerName := om.ValidationContext.TrustBundlePeerNameKey
		tb, ok := pr.proxyState.TrustBundles[peerName]
		if !ok {
			return nil, fmt.Errorf("failed to create transport socket: provided peer name does not exist in trust bundle map: %s", peerName)
		}

		var matchers []*envoy_matcher_v3.StringMatcher
		if len(om.ValidationContext.SpiffeIds) > 0 {
			matchers = make([]*envoy_matcher_v3.StringMatcher, 0)
			for _, m := range om.ValidationContext.SpiffeIds {
				matchers = append(matchers, &envoy_matcher_v3.StringMatcher{
					MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
						Exact: m,
					},
				})
			}
		}
		commonTLSContext.ValidationContextType = &envoy_tls_v3.CommonTlsContext_ValidationContext{
			ValidationContext: &envoy_tls_v3.CertificateValidationContext{
				// TODO(banks): later for L7 support we may need to configure ALPN here.
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: RootPEMsAsString(tb.Roots),
					},
				},
				MatchSubjectAltNames: matchers,
			},
		}

		upstreamContext.Sni = om.Sni
		transportSocket, err := makeTransportSocket("tls", upstreamContext)
		if err != nil {
			return nil, err
		}
		return transportSocket, nil
	default:
		return nil, nil
	}

}

func (pr *ProxyResources) makeEnvoyCertConfig(common *envoy_tls_v3.CommonTlsContext, certificate *pbproxystate.LeafCertificate) error {
	if certificate == nil {
		return fmt.Errorf("no leaf certificate provided")
	}
	common.TlsCertificates = []*envoy_tls_v3.TlsCertificate{
		{
			CertificateChain: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_InlineString{
					InlineString: lib.EnsureTrailingNewline(certificate.Cert),
				},
			},
			PrivateKey: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_InlineString{
					InlineString: lib.EnsureTrailingNewline(certificate.Key),
				},
			},
		},
	}
	return nil
}

func (pr *ProxyResources) makeEnvoySDSCertConfig(common *envoy_tls_v3.CommonTlsContext, certificate *pbproxystate.SDSCertificate) error {
	if certificate == nil {
		return fmt.Errorf("no SDS certificate provided")
	}
	common.TlsCertificateSdsSecretConfigs = []*envoy_tls_v3.SdsSecretConfig{
		{
			Name: certificate.CertResource,
			SdsConfig: &envoy_core_v3.ConfigSource{
				ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_ApiConfigSource{
					ApiConfigSource: &envoy_core_v3.ApiConfigSource{
						ApiType:             envoy_core_v3.ApiConfigSource_GRPC,
						TransportApiVersion: envoy_core_v3.ApiVersion_V3,
						// Note ClusterNames can't be set here - that's only for REST type
						// we need a full GRPC config instead.
						GrpcServices: []*envoy_core_v3.GrpcService{
							{
								TargetSpecifier: &envoy_core_v3.GrpcService_EnvoyGrpc_{
									EnvoyGrpc: &envoy_core_v3.GrpcService_EnvoyGrpc{
										ClusterName: certificate.ClusterName,
									},
								},
								Timeout: &durationpb.Duration{Seconds: 5},
							},
						},
					},
				},
				ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
			},
		},
	}
	return nil
}

func (pr *ProxyResources) addNonMeshCertConfig(common *envoy_tls_v3.CommonTlsContext, tls *pbproxystate.InboundNonMeshTLS) error {
	if tls == nil {
		return fmt.Errorf("no inbound non-mesh TLS provided")
	}

	switch tls.Identity.(type) {
	case *pbproxystate.InboundNonMeshTLS_LeafKey:
		leafKey := tls.Identity.(*pbproxystate.InboundNonMeshTLS_LeafKey).LeafKey
		leaf, ok := pr.proxyState.LeafCertificates[leafKey]
		if !ok {
			return fmt.Errorf("leaf key %s not found in leaf certificate map", leafKey)
		}
		common.TlsCertificates = []*envoy_tls_v3.TlsCertificate{
			{
				CertificateChain: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: lib.EnsureTrailingNewline(leaf.Cert),
					},
				},
				PrivateKey: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: lib.EnsureTrailingNewline(leaf.Key),
					},
				},
			},
		}
	case *pbproxystate.InboundNonMeshTLS_Sds:
		c := tls.Identity.(*pbproxystate.InboundNonMeshTLS_Sds).Sds
		common.TlsCertificateSdsSecretConfigs = []*envoy_tls_v3.SdsSecretConfig{
			{
				Name: c.CertResource,
				SdsConfig: &envoy_core_v3.ConfigSource{
					ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_ApiConfigSource{
						ApiConfigSource: &envoy_core_v3.ApiConfigSource{
							ApiType:             envoy_core_v3.ApiConfigSource_GRPC,
							TransportApiVersion: envoy_core_v3.ApiVersion_V3,
							// Note ClusterNames can't be set here - that's only for REST type
							// we need a full GRPC config instead.
							GrpcServices: []*envoy_core_v3.GrpcService{
								{
									TargetSpecifier: &envoy_core_v3.GrpcService_EnvoyGrpc_{
										EnvoyGrpc: &envoy_core_v3.GrpcService_EnvoyGrpc{
											ClusterName: c.ClusterName,
										},
									},
									Timeout: &durationpb.Duration{Seconds: 5},
								},
							},
						},
					},
					ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
				},
			},
		}
	}

	return nil
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

func makeEnvoyListenerFilter(c pbproxystate.Capability) (*envoy_listener_v3.ListenerFilter, error) {
	var lf proto.Message
	var name string

	switch c {
	case pbproxystate.Capability_CAPABILITY_TRANSPARENT:
		lf = &envoy_original_dst_v3.OriginalDst{}
		name = envoyOriginalDestinationListenerFilterName
	case pbproxystate.Capability_CAPABILITY_L4_TLS_INSPECTION:
		name = envoyTLSInspectorListenerFilterName
		lf = &envoy_tls_inspector_v3.TlsInspector{}
	case pbproxystate.Capability_CAPABILITY_L7_PROTOCOL_INSPECTION:
		name = envoyHttpInspectorListenerFilterName
		lf = &envoy_extensions_filters_listener_http_inspector_v3.HttpInspector{}
	default:
		return nil, fmt.Errorf("unsupported listener captability: %s", c)
	}
	lfAsAny, err := anypb.New(lf)
	if err != nil {
		return nil, err
	}

	return &envoy_listener_v3.ListenerFilter{
		Name:       name,
		ConfigType: &envoy_listener_v3.ListenerFilter_TypedConfig{TypedConfig: lfAsAny},
	}, nil
}

func makeEnvoyFilter(name string, cfg proto.Message) (*envoy_listener_v3.Filter, error) {
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

func RootPEMsAsString(rootPEMs []string) string {
	var rootPEMsString string
	for _, root := range rootPEMs {
		rootPEMsString += lib.EnsureTrailingNewline(root)
	}
	return rootPEMsString
}

func marshalEnvoyTLSCipherSuiteStrings(cipherSuites []pbproxystate.TLSCipherSuite) []string {
	envoyTLSCipherSuiteStrings := map[pbproxystate.TLSCipherSuite]string{
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_AES128_GCM_SHA256: "ECDHE-ECDSA-AES128-GCM-SHA256",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_CHACHA20_POLY1305: "ECDHE-ECDSA-CHACHA20-POLY1305",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_AES128_GCM_SHA256:   "ECDHE-RSA-AES128-GCM-SHA256",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_CHACHA20_POLY1305:   "ECDHE-RSA-CHACHA20-POLY1305",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_AES128_SHA:        "ECDHE-ECDSA-AES128-SHA",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_AES128_SHA:          "ECDHE-RSA-AES128-SHA",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_AES128_GCM_SHA256:             "AES128-GCM-SHA256",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_AES128_SHA:                    "AES128-SHA",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_AES256_GCM_SHA384: "ECDHE-ECDSA-AES256-GCM-SHA384",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_AES256_GCM_SHA384:   "ECDHE-RSA-AES256-GCM-SHA384",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_ECDSA_AES256_SHA:        "ECDHE-ECDSA-AES256-SHA",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_ECDHE_RSA_AES256_SHA:          "ECDHE-RSA-AES256-SHA",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_AES256_GCM_SHA384:             "AES256-GCM-SHA384",
		pbproxystate.TLSCipherSuite_TLS_CIPHER_SUITE_AES256_SHA:                    "AES256-SHA",
	}

	var cipherSuiteStrings []string

	for _, c := range cipherSuites {
		if s, ok := envoyTLSCipherSuiteStrings[c]; ok {
			cipherSuiteStrings = append(cipherSuiteStrings, s)
		}
	}

	return cipherSuiteStrings
}

var envoyTLSVersions = map[pbproxystate.TLSVersion]envoy_tls_v3.TlsParameters_TlsProtocol{
	pbproxystate.TLSVersion_TLS_VERSION_AUTO: envoy_tls_v3.TlsParameters_TLS_AUTO,
	pbproxystate.TLSVersion_TLS_VERSION_1_0:  envoy_tls_v3.TlsParameters_TLSv1_0,
	pbproxystate.TLSVersion_TLS_VERSION_1_1:  envoy_tls_v3.TlsParameters_TLSv1_1,
	pbproxystate.TLSVersion_TLS_VERSION_1_2:  envoy_tls_v3.TlsParameters_TLSv1_2,
	pbproxystate.TLSVersion_TLS_VERSION_1_3:  envoy_tls_v3.TlsParameters_TLSv1_3,
}

// Sort the trust domains so that the output is stable.
// This benefits tests but also prevents Envoy from mistakenly thinking the listener
// changed and needs to be drained only because this ordering is different.
func sortTrustDomains(trustDomains []*envoy_tls_v3.SPIFFECertValidatorConfig_TrustDomain) {
	sort.Slice(trustDomains, func(i int, j int) bool {
		return trustDomains[i].Name < trustDomains[j].Name
	})
}

// sortRouters stable sorts routers with a Match to avoid draining if the list is provided out of order.
// xdsv1 used to sort the filter chains on outbound listeners, so this adds that functionality by sorting routers with matches.
func sortRouters(routers []*pbproxystate.Router) {
	if routers == nil {
		return
	}
	sort.SliceStable(routers, func(i, j int) bool {
		si := ""
		sj := ""
		if routers[i].Match != nil {
			if len(routers[i].Match.PrefixRanges) > 0 {
				si += routers[i].Match.PrefixRanges[0].AddressPrefix +
					"/" + routers[i].Match.PrefixRanges[0].PrefixLen.String() +
					":" + routers[i].Match.DestinationPort.String()
			}
			if len(routers[i].Match.ServerNames) > 0 {
				si += routers[i].Match.ServerNames[0] +
					":" + routers[i].Match.DestinationPort.String()
			} else {
				si += routers[i].Match.DestinationPort.String()
			}
		}

		if routers[j].Match != nil {
			if len(routers[j].Match.PrefixRanges) > 0 {
				sj += routers[j].Match.PrefixRanges[0].AddressPrefix +
					"/" + routers[j].Match.PrefixRanges[0].PrefixLen.String() +
					":" + routers[j].Match.DestinationPort.String()
			}
			if len(routers[j].Match.ServerNames) > 0 {
				sj += routers[j].Match.ServerNames[0] +
					":" + routers[j].Match.DestinationPort.String()
			} else {
				sj += routers[j].Match.DestinationPort.String()
			}
		}

		return si < sj
	})
}

func sortPrefixRanges(prefixRanges []*pbproxystate.CidrRange) {
	if prefixRanges == nil {
		return
	}
	sort.SliceStable(prefixRanges, func(i, j int) bool {
		return prefixRanges[i].AddressPrefix < prefixRanges[j].AddressPrefix
	})
}
