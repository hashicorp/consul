package xds

import (
	"fmt"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func (s *ResourceGenerator) makeIngressGatewayListeners(address string, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message

	for listenerKey, upstreams := range cfgSnap.IngressGateway.Upstreams {
		var tlsContext *envoy_tls_v3.DownstreamTlsContext

		listenerCfg, ok := cfgSnap.IngressGateway.Listeners[listenerKey]
		if !ok {
			return nil, fmt.Errorf("no listener config found for listener on port %d", listenerKey.Port)
		}
		// Enable connect TLS if it is enabled at the Gateway or specific listener
		// level.
		connectTLSEnabled := cfgSnap.IngressGateway.TLSConfig.Enabled ||
			(listenerCfg.TLS != nil && listenerCfg.TLS.Enabled)

		sdsCfg, err := resolveListenerSDSConfig(cfgSnap, listenerKey)
		if err != nil {
			return nil, err
		}

		if sdsCfg != nil {
			// Set up listener TLS from SDS
			tlsContext = &envoy_tls_v3.DownstreamTlsContext{
				CommonTlsContext:         makeCommonTLSContextFromSDS(*sdsCfg),
				RequireClientCertificate: &wrappers.BoolValue{Value: false},
			}
		} else if connectTLSEnabled {
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

			// Generate any filter chains needed for services with custom TLS certs
			// via SDS.
			sniFilterChains, err := makeSDSOverrideFilterChains(cfgSnap, listenerKey, opts)
			if err != nil {
				return nil, err
			}

			// If there are any sni filter chains, we need a TLS inspector filter!
			if len(sniFilterChains) > 0 {
				tlsInspector, err := makeTLSInspectorListenerFilter()
				if err != nil {
					return nil, err
				}
				listener.ListenerFilters = []*envoy_listener_v3.ListenerFilter{tlsInspector}
			}

			listener.FilterChains = sniFilterChains

			// See if there are other services that didn't have specific SNI-matching
			// filter chains. If so add a default filterchain to serve them.
			if len(sniFilterChains) < len(upstreams) {
				defaultFilter, err := makeListenerFilter(opts)
				if err != nil {
					return nil, err
				}

				transportSocket, err := makeDownstreamTLSTransportSocket(tlsContext)
				if err != nil {
					return nil, err
				}
				listener.FilterChains = append(listener.FilterChains,
					&envoy_listener_v3.FilterChain{
						Filters: []*envoy_listener_v3.Filter{
							defaultFilter,
						},
						TransportSocket: transportSocket,
					})
			}

			resources = append(resources, listener)
		}
	}

	return resources, nil
}

func resolveListenerSDSConfig(cfgSnap *proxycfg.ConfigSnapshot, listenerKey proxycfg.IngressListenerKey) (*structs.GatewayTLSSDSConfig, error) {
	var mergedCfg structs.GatewayTLSSDSConfig

	gwSDS := cfgSnap.IngressGateway.TLSConfig.SDS
	if gwSDS != nil {
		mergedCfg.ClusterName = gwSDS.ClusterName
		mergedCfg.CertResource = gwSDS.CertResource
	}

	listenerCfg, ok := cfgSnap.IngressGateway.Listeners[listenerKey]
	if !ok {
		return nil, fmt.Errorf("no listener config found for listener on port %d", listenerKey.Port)
	}

	if listenerCfg.TLS != nil && listenerCfg.TLS.SDS != nil {
		if listenerCfg.TLS.SDS.ClusterName != "" {
			mergedCfg.ClusterName = listenerCfg.TLS.SDS.ClusterName
		}
		if listenerCfg.TLS.SDS.CertResource != "" {
			mergedCfg.CertResource = listenerCfg.TLS.SDS.CertResource
		}
	}

	// Validate. Either merged should have both fields empty or both set. Other
	// cases shouldn't be possible as we validate them at input but be robust to
	// bugs later.
	switch {
	case mergedCfg.ClusterName == "" && mergedCfg.CertResource == "":
		return nil, nil

	case mergedCfg.ClusterName != "" && mergedCfg.CertResource != "":
		return &mergedCfg, nil

	case mergedCfg.ClusterName == "" && mergedCfg.CertResource != "":
		return nil, fmt.Errorf("missing SDS cluster name for listener on port %d", listenerKey.Port)

	case mergedCfg.ClusterName != "" && mergedCfg.CertResource == "":
		return nil, fmt.Errorf("missing SDS cert resource for listener on port %d", listenerKey.Port)
	}

	return &mergedCfg, nil
}

func routeNameForUpstream(l structs.IngressListener, s structs.IngressService) string {
	key := proxycfg.IngressListenerKeyFromListener(l)

	// If the upstream service doesn't have any TLS overrides then it can just use
	// the combined filterchain with all the merged routes.
	if !ingressServiceHasSDSOverrides(s) {
		return key.RouteName()
	}

	// Return a specific route for this service as it needs a custom FilterChain
	// to serve its custom cert so we should attach its routes to a separate Route
	// too. We need this to be consistent between OSS and Enterprise to avoid xDS
	// config golden files in tests conflicting so we can't use ServiceID.String()
	// which normalizes to included all identifiers in Enterprise.
	sn := s.ToServiceName()
	svcIdentifier := sn.Name
	if !sn.InDefaultPartition() || !sn.InDefaultNamespace() {
		// Non-default partition/namespace, use a full identifier
		svcIdentifier = sn.String()
	}
	return fmt.Sprintf("%s_%s", key.RouteName(), svcIdentifier)
}

func ingressServiceHasSDSOverrides(s structs.IngressService) bool {
	return s.TLS != nil &&
		s.TLS.SDS != nil &&
		s.TLS.SDS.CertResource != ""
}

// ingress services that specify custom TLS certs via SDS overrides need to get
// their own filter chain and routes. This will generate all the extra filter
// chains an ingress listener needs. It may be empty and expects the default
// catch-all chain and route to contain all the other services that share the
// default TLS config.
func makeSDSOverrideFilterChains(cfgSnap *proxycfg.ConfigSnapshot,
	listenerKey proxycfg.IngressListenerKey,
	filterOpts listenerFilterOpts) ([]*envoy_listener_v3.FilterChain, error) {

	listenerCfg, ok := cfgSnap.IngressGateway.Listeners[listenerKey]
	if !ok {
		return nil, fmt.Errorf("no listener config found for listener on port %d", listenerKey.Port)
	}

	var chains []*envoy_listener_v3.FilterChain

	for _, svc := range listenerCfg.Services {
		if !ingressServiceHasSDSOverrides(svc) {
			continue
		}

		if len(svc.Hosts) < 1 {
			// Shouldn't be possible with validation but be careful
			return nil, fmt.Errorf("no hosts specified with SDS certificate (service %q on listener on port %d)",
				svc.ToServiceName().ToServiceID().String(), listenerKey.Port)
		}

		// Service has a certificate resource override. Return a new filter chain
		// with the right TLS cert and a filter that will load only the routes for
		// this service.
		routeName := routeNameForUpstream(listenerCfg, svc)
		filterOpts.filterName = routeName
		filterOpts.routeName = routeName
		filter, err := makeListenerFilter(filterOpts)
		if err != nil {
			return nil, err
		}

		tlsContext := &envoy_tls_v3.DownstreamTlsContext{
			CommonTlsContext:         makeCommonTLSContextFromSDS(*svc.TLS.SDS),
			RequireClientCertificate: &wrappers.BoolValue{Value: false},
		}

		transportSocket, err := makeDownstreamTLSTransportSocket(tlsContext)
		if err != nil {
			return nil, err
		}

		chain := &envoy_listener_v3.FilterChain{
			// Only match traffic for this service's hosts.
			FilterChainMatch: makeSNIFilterChainMatch(svc.Hosts...),
			Filters: []*envoy_listener_v3.Filter{
				filter,
			},
			TransportSocket: transportSocket,
		}

		chains = append(chains, chain)
	}

	return chains, nil
}

func makeCommonTLSContextFromSDS(sdsCfg structs.GatewayTLSSDSConfig) *envoy_tls_v3.CommonTlsContext {
	return &envoy_tls_v3.CommonTlsContext{
		TlsParams: &envoy_tls_v3.TlsParameters{},
		TlsCertificateSdsSecretConfigs: []*envoy_tls_v3.SdsSecretConfig{
			{
				Name: sdsCfg.CertResource,
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
											ClusterName: sdsCfg.ClusterName,
										},
									},
									Timeout: &duration.Duration{Seconds: 5},
								},
							},
						},
					},
					ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
				},
			},
		},
	}
}
