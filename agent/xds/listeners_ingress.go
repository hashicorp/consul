// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"fmt"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
)

func (s *ResourceGenerator) makeIngressGatewayListeners(address string, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message

	for listenerKey, upstreams := range cfgSnap.IngressGateway.Upstreams {
		listenerCfg, ok := cfgSnap.IngressGateway.Listeners[listenerKey]
		if !ok {
			return nil, fmt.Errorf("no listener config found for listener on proto/port %s/%d", listenerKey.Protocol, listenerKey.Port)
		}

		tlsContext, err := makeDownstreamTLSContextFromSnapshotListenerConfig(cfgSnap, listenerCfg)
		if err != nil {
			return nil, err
		}

		if listenerKey.Protocol == "tcp" {
			// We rely on the invariant of upstreams slice always having at least 1
			// member, because this key/value pair is created only when a
			// GatewayService is returned in the RPC
			u := upstreams[0]

			uid := proxycfg.NewUpstreamID(&u)

			chain := cfgSnap.IngressGateway.DiscoveryChain[uid]
			if chain == nil {
				// Wait until a chain is present in the snapshot.
				continue
			}

			cfg := s.getAndModifyUpstreamConfigForListener(uid, &u, chain)

			// RDS, Envoy's Route Discovery Service, is only used for HTTP services with a customized discovery chain.
			// TODO(freddy): Why can the protocol of the listener be overridden here?
			useRDS := cfg.Protocol != "tcp" && !chain.Default

			var clusterName string
			if !useRDS {
				// When not using RDS we must generate a cluster name to attach to the filter chain.
				// With RDS, cluster names get attached to the dynamic routes instead.
				target, err := simpleChainTarget(chain)
				if err != nil {
					return nil, err
				}
				clusterName = CustomizeClusterName(target.Name, chain)
			}

			filterName := fmt.Sprintf("%s.%s.%s.%s", chain.ServiceName, chain.Namespace, chain.Partition, chain.Datacenter)

			opts := makeListenerOpts{
				name:       uid.EnvoyID(),
				accessLogs: cfgSnap.Proxy.AccessLogs,
				addr:       address,
				port:       u.LocalBindPort,
				direction:  envoy_core_v3.TrafficDirection_OUTBOUND,
				logger:     s.Logger,
			}
			l := makeListener(opts)

			filterChain, err := s.makeUpstreamFilterChain(filterChainOpts{
				accessLogs:  &cfgSnap.Proxy.AccessLogs,
				routeName:   uid.EnvoyID(),
				useRDS:      useRDS,
				clusterName: clusterName,
				filterName:  filterName,
				protocol:    cfg.Protocol,
				tlsContext:  tlsContext,
			})
			if err != nil {
				return nil, err
			}
			l.FilterChains = []*envoy_listener_v3.FilterChain{
				filterChain,
			}

			resources = append(resources, l)
		} else {
			// If multiple upstreams share this port, make a special listener for the protocol.
			listenerOpts := makeListenerOpts{
				name:       listenerKey.Protocol,
				accessLogs: cfgSnap.Proxy.AccessLogs,
				addr:       address,
				port:       listenerKey.Port,
				direction:  envoy_core_v3.TrafficDirection_OUTBOUND,
				logger:     s.Logger,
			}

			listener := makeListener(listenerOpts)

			filterOpts := listenerFilterOpts{
				useRDS:           true,
				protocol:         listenerKey.Protocol,
				filterName:       listenerKey.RouteName(),
				routeName:        listenerKey.RouteName(),
				cluster:          "",
				statPrefix:       "ingress_upstream_",
				routePath:        "",
				httpAuthzFilters: nil,
				accessLogs:       &cfgSnap.Proxy.AccessLogs,
				logger:           s.Logger,
			}

			// Generate any filter chains needed for services with custom TLS certs
			// via SDS.
			sniFilterChains, err := makeSDSOverrideFilterChains(cfgSnap, listenerKey, filterOpts)
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
				defaultFilter, err := makeListenerFilter(filterOpts)
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

func makeDownstreamTLSContextFromSnapshotListenerConfig(cfgSnap *proxycfg.ConfigSnapshot, listenerCfg structs.IngressListener) (*envoy_tls_v3.DownstreamTlsContext, error) {
	var downstreamContext *envoy_tls_v3.DownstreamTlsContext

	tlsContext, err := makeCommonTLSContextFromSnapshotListenerConfig(cfgSnap, listenerCfg)
	if err != nil {
		return nil, err
	}

	if tlsContext != nil {
		// Configure alpn protocols on TLSContext
		tlsContext.AlpnProtocols = getAlpnProtocols(listenerCfg.Protocol)

		downstreamContext = &envoy_tls_v3.DownstreamTlsContext{
			CommonTlsContext:         tlsContext,
			RequireClientCertificate: &wrapperspb.BoolValue{Value: false},
		}
	}

	return downstreamContext, nil
}

func makeCommonTLSContextFromSnapshotListenerConfig(cfgSnap *proxycfg.ConfigSnapshot, listenerCfg structs.IngressListener) (*envoy_tls_v3.CommonTlsContext, error) {
	var tlsContext *envoy_tls_v3.CommonTlsContext

	// Enable connect TLS if it is enabled at the Gateway or specific listener
	// level.
	gatewayTLSCfg := cfgSnap.IngressGateway.TLSConfig

	// It is not possible to explicitly _disable_ TLS on a listener if it's
	// enabled on the gateway, because false is the zero-value of the struct field
	// and therefore indistinguishable from it being unspecified.
	connectTLSEnabled := gatewayTLSCfg.Enabled ||
		(listenerCfg.TLS != nil && listenerCfg.TLS.Enabled)

	tlsCfg, err := resolveListenerTLSConfig(&gatewayTLSCfg, listenerCfg)
	if err != nil {
		return nil, err
	}

	if tlsCfg.SDS != nil {
		// Set up listener TLS from SDS
		tlsContext = makeCommonTLSContextFromGatewayTLSConfig(*tlsCfg)
	} else if connectTLSEnabled {
		tlsContext = makeCommonTLSContext(cfgSnap.Leaf(), cfgSnap.RootPEMs(), makeTLSParametersFromGatewayTLSConfig(*tlsCfg))
	}

	return tlsContext, nil
}

func resolveListenerTLSConfig(gatewayTLSCfg *structs.GatewayTLSConfig, listenerCfg structs.IngressListener) (*structs.GatewayTLSConfig, error) {
	var mergedCfg structs.GatewayTLSConfig

	resolvedSDSCfg, err := resolveListenerSDSConfig(gatewayTLSCfg.SDS, listenerCfg.TLS, listenerCfg.Port)
	if err != nil {
		return nil, err
	}

	mergedCfg.SDS = resolvedSDSCfg

	if gatewayTLSCfg != nil {
		mergedCfg.TLSMinVersion = gatewayTLSCfg.TLSMinVersion
		mergedCfg.TLSMaxVersion = gatewayTLSCfg.TLSMaxVersion
		mergedCfg.CipherSuites = gatewayTLSCfg.CipherSuites
	}

	if listenerCfg.TLS != nil {
		if listenerCfg.TLS.TLSMinVersion != types.TLSVersionUnspecified {
			mergedCfg.TLSMinVersion = listenerCfg.TLS.TLSMinVersion
		}
		if listenerCfg.TLS.TLSMaxVersion != types.TLSVersionUnspecified {
			mergedCfg.TLSMaxVersion = listenerCfg.TLS.TLSMaxVersion
		}
		if len(listenerCfg.TLS.CipherSuites) != 0 {
			mergedCfg.CipherSuites = listenerCfg.TLS.CipherSuites
		}
	}

	if err := validateListenerTLSConfig(mergedCfg.TLSMinVersion, mergedCfg.CipherSuites); err != nil {
		return nil, err
	}

	return &mergedCfg, nil
}

func resolveListenerSDSConfig(gatewaySDSCfg *structs.GatewayTLSSDSConfig, listenerTLSCfg *structs.GatewayTLSConfig, listenerPort int) (*structs.GatewayTLSSDSConfig, error) {
	var mergedCfg structs.GatewayTLSSDSConfig

	if gatewaySDSCfg != nil {
		mergedCfg.ClusterName = gatewaySDSCfg.ClusterName
		mergedCfg.CertResource = gatewaySDSCfg.CertResource
	}

	if listenerTLSCfg != nil && listenerTLSCfg.SDS != nil {
		if listenerTLSCfg.SDS.ClusterName != "" {
			mergedCfg.ClusterName = listenerTLSCfg.SDS.ClusterName
		}
		if listenerTLSCfg.SDS.CertResource != "" {
			mergedCfg.CertResource = listenerTLSCfg.SDS.CertResource
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
		return nil, fmt.Errorf("missing SDS cluster name for listener on port %d", listenerPort)

	case mergedCfg.ClusterName != "" && mergedCfg.CertResource == "":
		return nil, fmt.Errorf("missing SDS cert resource for listener on port %d", listenerPort)
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
	// too. We need this to be consistent between CE and Enterprise to avoid xDS
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

		commonTlsContext := makeCommonTLSContextFromGatewayServiceTLSConfig(*svc.TLS)
		if commonTlsContext != nil {
			// Configure alpn protocols on TLSContext
			commonTlsContext.AlpnProtocols = getAlpnProtocols(listenerCfg.Protocol)
		}

		tlsContext := &envoy_tls_v3.DownstreamTlsContext{
			CommonTlsContext:         commonTlsContext,
			RequireClientCertificate: &wrapperspb.BoolValue{Value: false},
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

func makeTLSParametersFromGatewayTLSConfig(tlsCfg structs.GatewayTLSConfig) *envoy_tls_v3.TlsParameters {
	return makeTLSParametersFromTLSConfig(tlsCfg.TLSMinVersion, tlsCfg.TLSMaxVersion, tlsCfg.CipherSuites)
}

func makeInlineTLSContextFromGatewayTLSConfig(tlsCfg structs.GatewayTLSConfig, cert structs.InlineCertificateConfigEntry) *envoy_tls_v3.CommonTlsContext {
	return &envoy_tls_v3.CommonTlsContext{
		TlsParams: makeTLSParametersFromGatewayTLSConfig(tlsCfg),
		TlsCertificates: []*envoy_tls_v3.TlsCertificate{{
			CertificateChain: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_InlineString{
					InlineString: lib.EnsureTrailingNewline(cert.Certificate),
				},
			},
			PrivateKey: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_InlineString{
					InlineString: lib.EnsureTrailingNewline(cert.PrivateKey),
				},
			},
		}},
	}
}

func makeCommonTLSContextFromGatewayTLSConfig(tlsCfg structs.GatewayTLSConfig) *envoy_tls_v3.CommonTlsContext {
	return &envoy_tls_v3.CommonTlsContext{
		TlsParams:                      makeTLSParametersFromGatewayTLSConfig(tlsCfg),
		TlsCertificateSdsSecretConfigs: makeTLSCertificateSdsSecretConfigsFromSDS(*tlsCfg.SDS),
	}
}

func makeCommonTLSContextFromGatewayServiceTLSConfig(tlsCfg structs.GatewayServiceTLSConfig) *envoy_tls_v3.CommonTlsContext {
	return &envoy_tls_v3.CommonTlsContext{
		TlsParams:                      &envoy_tls_v3.TlsParameters{},
		TlsCertificateSdsSecretConfigs: makeTLSCertificateSdsSecretConfigsFromSDS(*tlsCfg.SDS),
	}
}

func makeTLSCertificateSdsSecretConfigsFromSDS(sdsCfg structs.GatewayTLSSDSConfig) []*envoy_tls_v3.SdsSecretConfig {
	return []*envoy_tls_v3.SdsSecretConfig{
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
