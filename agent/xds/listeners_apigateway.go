// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"sort"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_http_jwt_authn_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/naming"
	"github.com/hashicorp/consul/types"
)

func (s *ResourceGenerator) makeAPIGatewayListeners(address string, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message

	readyListeners := getReadyListeners(cfgSnap)

	proxyConfig := cfgSnap.GetProxyConfig(s.Logger)

	for _, readyListener := range readyListeners {
		listenerCfg := readyListener.listenerCfg
		listenerKey := readyListener.listenerKey
		boundListener := readyListener.boundListenerCfg

		// Collect the referenced certificate config entries
		var certs []structs.ConfigEntry
		for _, certRef := range boundListener.Certificates {
			switch certRef.Kind {
			case structs.InlineCertificate:
				if cert, ok := cfgSnap.APIGateway.InlineCertificates.Get(certRef); ok {
					certs = append(certs, cert)
				}
			case structs.FileSystemCertificate:
				if cert, ok := cfgSnap.APIGateway.FileSystemCertificates.Get(certRef); ok {
					certs = append(certs, cert)
				}
			}
		}

		effectiveTLSCfg, err := resolveAPIListenerTLSConfig(cfgSnap.APIGateway.TLSConfig, listenerCfg.TLS)
		if err != nil {
			return nil, err
		}

		routeSDSOverrides, err := collectAPIGatewayServiceSDSOverridesWithResolvedTLS(cfgSnap, readyListener, effectiveTLSCfg)
		if err != nil {
			return nil, err
		}

		isAPIGatewayWithTLS := len(boundListener.Certificates) > 0 || hasSDSCert(effectiveTLSCfg.SDS) || len(routeSDSOverrides) > 0

		tlsContext, err := makeDownstreamTLSContextFromSnapshotAPIListenerConfig(cfgSnap, listenerCfg)
		if err != nil {
			return nil, err
		}

		if listenerCfg.Protocol == structs.ListenerProtocolTCP {
			// Find the upstream matching this listener

			// We rely on the invariant of upstreams slice always having at least 1
			// member, because this key/value pair is created only when a
			// GatewayService is returned in the RPC
			u := readyListener.upstreams[0]
			uid := proxycfg.NewUpstreamID(&u)

			chain := cfgSnap.APIGateway.DiscoveryChain[uid]
			if chain == nil {
				// Wait until a chain is present in the snapshot.
				continue
			}

			cfg := s.getAndModifyUpstreamConfigForListener(uid, &u, chain)
			useRDS := cfg.Protocol != "tcp" && !chain.Default

			var clusterName string
			if !useRDS {
				// When not using RDS we must generate a cluster name to attach to the filter chain.
				// With RDS, cluster names get attached to the dynamic routes instead.
				target, err := simpleChainTarget(chain)
				if err != nil {
					return nil, err
				}
				clusterName = naming.CustomizeClusterName(target.Name, chain)
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
				accessLogs:      &cfgSnap.Proxy.AccessLogs,
				routeName:       uid.EnvoyID(),
				useRDS:          useRDS,
				fetchTimeoutRDS: cfgSnap.GetXDSCommonConfig(s.Logger).GetXDSFetchTimeout(),
				clusterName:     clusterName,
				filterName:      filterName,
				protocol:        cfg.Protocol,
				tlsContext:      tlsContext,
			})
			if err != nil {
				return nil, err
			}
			l.FilterChains = []*envoy_listener_v3.FilterChain{
				filterChain,
			}

			if isAPIGatewayWithTLS {

				maxRequestHeadersKb := proxyConfig.MaxRequestHeadersKB
				if listenerCfg.MaxRequestHeadersKB != nil {
					maxRequestHeadersKb = listenerCfg.MaxRequestHeadersKB
				}

				// construct SNI filter chains
				l.FilterChains, err = s.makeInlineOverrideFilterChains(
					cfgSnap,
					*effectiveTLSCfg,
					routeSDSOverrides,
					listenerKey.Protocol, listenerFilterOpts{
						useRDS:              useRDS,
						fetchTimeoutRDS:     cfgSnap.GetXDSCommonConfig(s.Logger).GetXDSFetchTimeout(),
						protocol:            listenerKey.Protocol,
						routeName:           listenerKey.RouteName(),
						cluster:             clusterName,
						statPrefix:          "ingress_upstream_",
						accessLogs:          &cfgSnap.Proxy.AccessLogs,
						logger:              s.Logger,
						maxRequestHeadersKb: maxRequestHeadersKb,
					},
					certs,
				)
				if err != nil {
					return nil, err
				}

				// add the tls inspector to do SNI introspection
				tlsInspector, err := makeTLSInspectorListenerFilter()
				if err != nil {
					return nil, err
				}
				l.ListenerFilters = []*envoy_listener_v3.ListenerFilter{tlsInspector}
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

			routes := make([]*structs.HTTPRouteConfigEntry, 0, len(readyListener.routeReferences))
			for _, routeRef := range maps.Keys(readyListener.routeReferences) {
				route, ok := cfgSnap.APIGateway.HTTPRoutes.Get(routeRef)
				if !ok {
					return nil, fmt.Errorf("missing route for routeRef %s:%s", routeRef.Kind, routeRef.Name)
				}

				routes = append(routes, route)
			}
			consolidatedRoutes := discoverychain.ConsolidateHTTPRoutes(cfgSnap.APIGateway.GatewayConfig, &readyListener.listenerCfg, routes...)
			routesWithJWT := []*structs.HTTPRouteConfigEntry{}
			for _, routeCfgEntry := range consolidatedRoutes {
				routeCfgEntry := routeCfgEntry
				route := &routeCfgEntry

				if listenerCfg.Override != nil && listenerCfg.Override.JWT != nil {
					routesWithJWT = append(routesWithJWT, route)
					continue
				}

				if listenerCfg.Default != nil && listenerCfg.Default.JWT != nil {
					routesWithJWT = append(routesWithJWT, route)
					continue
				}

				for _, rule := range route.Rules {
					if rule.Filters.JWT != nil {
						routesWithJWT = append(routesWithJWT, route)
						continue
					}
					for _, svc := range rule.Services {
						if svc.Filters.JWT != nil {
							routesWithJWT = append(routesWithJWT, route)
							continue
						}
					}
				}

			}

			var authFilters []*envoy_http_v3.HttpFilter
			if len(routesWithJWT) > 0 {
				builder := &GatewayAuthFilterBuilder{
					listener:       listenerCfg,
					routes:         routesWithJWT,
					providers:      cfgSnap.JWTProviders,
					envoyProviders: make(map[string]*envoy_http_jwt_authn_v3.JwtProvider, len(cfgSnap.JWTProviders)),
				}
				authFilters, err = builder.makeGatewayAuthFilters()
				if err != nil {
					return nil, err
				}
			}
			maxRequestHeadersKb := proxyConfig.MaxRequestHeadersKB
			if listenerCfg.MaxRequestHeadersKB != nil {
				maxRequestHeadersKb = listenerCfg.MaxRequestHeadersKB
			}
			filterOpts := listenerFilterOpts{
				useRDS:              true,
				fetchTimeoutRDS:     cfgSnap.GetXDSCommonConfig(s.Logger).GetXDSFetchTimeout(),
				protocol:            listenerKey.Protocol,
				filterName:          listenerKey.RouteName(),
				routeName:           listenerKey.RouteName(),
				cluster:             "",
				statPrefix:          "ingress_upstream_",
				routePath:           "",
				httpAuthzFilters:    authFilters,
				accessLogs:          &cfgSnap.Proxy.AccessLogs,
				logger:              s.Logger,
				maxRequestHeadersKb: maxRequestHeadersKb,
			}

			// Generate any filter chains needed for services with custom TLS certs
			// via SDS.
			sniFilterChains := []*envoy_listener_v3.FilterChain{}

			if isAPIGatewayWithTLS {
				sniFilterChains, err = s.makeInlineOverrideFilterChains(cfgSnap, *effectiveTLSCfg, routeSDSOverrides, listenerKey.Protocol, filterOpts, certs)
				if err != nil {
					return nil, err
				}
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
			if len(sniFilterChains) < len(readyListener.upstreams) && !isAPIGatewayWithTLS {
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

// helper struct to persist upstream parent information when ready upstream list is built out
type readyListener struct {
	listenerKey      proxycfg.APIGatewayListenerKey
	listenerCfg      structs.APIGatewayListener
	boundListenerCfg structs.BoundAPIGatewayListener
	routeReferences  map[structs.ResourceReference]struct{}
	upstreams        []structs.Upstream
}

type apiGatewayServiceSDSOverride struct {
	Hosts []string
	SDS   structs.GatewayTLSSDSConfig
}

func collectAPIGatewayServiceSDSOverrides(cfgSnap *proxycfg.ConfigSnapshot, ready readyListener) ([]apiGatewayServiceSDSOverride, error) {
	resolvedTLSCfg, err := resolveAPIListenerTLSConfig(cfgSnap.APIGateway.TLSConfig, ready.listenerCfg.TLS)
	if err != nil {
		return nil, err
	}

	return collectAPIGatewayServiceSDSOverridesWithResolvedTLS(cfgSnap, ready, resolvedTLSCfg)
}

func collectAPIGatewayServiceSDSOverridesWithResolvedTLS(
	cfgSnap *proxycfg.ConfigSnapshot,
	ready readyListener,
	resolvedTLSCfg *structs.GatewayTLSConfig,
) ([]apiGatewayServiceSDSOverride, error) {

	var defaultSDS *structs.GatewayTLSSDSConfig
	if resolvedTLSCfg != nil && resolvedTLSCfg.SDS != nil && resolvedTLSCfg.SDS.ClusterName != "" {
		defaultSDS = resolvedTLSCfg.SDS
	}

	switch ready.listenerCfg.Protocol {
	case structs.ListenerProtocolHTTP:
		byKey := make(map[string]*apiGatewayServiceSDSOverride)
		hostToKey := make(map[string]string)

		for routeRef := range ready.routeReferences {
			route, ok := cfgSnap.APIGateway.HTTPRoutes.Get(routeRef)
			if !ok {
				continue
			}

			for _, rule := range route.Rules {
				for _, service := range rule.Services {
					sds := service.SDSConfigOrNil()
					if sds == nil {
						continue
					}

					effectiveSDS := *sds
					if effectiveSDS.ClusterName == "" && defaultSDS != nil {
						effectiveSDS.ClusterName = defaultSDS.ClusterName
					}
					if effectiveSDS.ClusterName == "" {
						return nil, fmt.Errorf("route %q service %q sets TLS.SDS without ClusterName and no listener or gateway TLS.SDS.ClusterName is available", route.Name, service.Name)
					}

					key := fmt.Sprintf("%s|%s", effectiveSDS.ClusterName, effectiveSDS.CertResource)
					override, ok := byKey[key]
					if !ok {
						override = &apiGatewayServiceSDSOverride{SDS: effectiveSDS}
						byKey[key] = override
					}

					for _, host := range route.Hostnames {
						if prev, seen := hostToKey[host]; seen && prev != key {
							return nil, fmt.Errorf("host %q maps to multiple TLS.SDS configs on listener %q", host, ready.listenerCfg.Name)
						}
						hostToKey[host] = key
						if !containsString(override.Hosts, host) {
							override.Hosts = append(override.Hosts, host)
						}
					}
				}
			}
		}

		keys := make([]string, 0, len(byKey))
		for k := range byKey {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		result := make([]apiGatewayServiceSDSOverride, 0, len(keys))
		for _, key := range keys {
			override := byKey[key]
			sort.Strings(override.Hosts)
			result = append(result, *override)
		}

		return result, nil

	case structs.ListenerProtocolTCP:
		var selected *structs.GatewayTLSSDSConfig

		for routeRef := range ready.routeReferences {
			route, ok := cfgSnap.APIGateway.TCPRoutes.Get(routeRef)
			if !ok {
				continue
			}

			for _, service := range route.Services {
				sds := service.SDSConfigOrNil()
				if sds == nil {
					continue
				}

				effectiveSDS := *sds
				if effectiveSDS.ClusterName == "" && defaultSDS != nil {
					effectiveSDS.ClusterName = defaultSDS.ClusterName
				}
				if effectiveSDS.ClusterName == "" {
					return nil, fmt.Errorf("route %q service %q sets TLS.SDS without ClusterName and no listener or gateway TLS.SDS.ClusterName is available", route.Name, service.Name)
				}

				if selected == nil {
					selected = &effectiveSDS
					continue
				}

				if selected.ClusterName != effectiveSDS.ClusterName || selected.CertResource != effectiveSDS.CertResource {
					return nil, fmt.Errorf("listener %q has multiple TCP route TLS.SDS overrides; found both %q/%q and %q/%q",
						ready.listenerCfg.Name,
						selected.ClusterName, selected.CertResource,
						effectiveSDS.ClusterName, effectiveSDS.CertResource,
					)
				}
			}
		}

		if selected == nil {
			return nil, nil
		}

		return []apiGatewayServiceSDSOverride{{SDS: *selected}}, nil
	}

	return nil, nil
}

func containsString(items []string, val string) bool {
	for _, item := range items {
		if item == val {
			return true
		}
	}
	return false
}

// getReadyListeners returns a map containing the list of upstreams for each listener that is ready
func getReadyListeners(cfgSnap *proxycfg.ConfigSnapshot) map[string]readyListener {
	ready := map[string]readyListener{}
	for _, l := range cfgSnap.APIGateway.Listeners {
		// Only include upstreams for listeners that are ready
		if !cfgSnap.APIGateway.GatewayConfig.ListenerIsReady(l.Name) {
			continue
		}

		// For each route bound to the listener
		boundListener := cfgSnap.APIGateway.BoundListeners[l.Name]
		for _, routeRef := range boundListener.Routes {
			// Get all upstreams for the route
			routeUpstreams, ok := cfgSnap.APIGateway.Upstreams[routeRef]
			if !ok {
				continue
			}

			// Filter to upstreams that attach to this specific listener since
			// a route can bind to + have upstreams for multiple listeners
			listenerKey := proxycfg.APIGatewayListenerKeyFromListener(l)
			routeUpstreamsForListener, ok := routeUpstreams[listenerKey]
			if !ok {
				continue
			}

			for _, upstream := range routeUpstreamsForListener {
				// Insert or update readyListener for the listener to include this upstream
				r, ok := ready[l.Name]
				if !ok {
					r = readyListener{
						listenerKey:      listenerKey,
						listenerCfg:      l,
						routeReferences:  map[structs.ResourceReference]struct{}{},
						boundListenerCfg: boundListener,
					}
				}
				r.routeReferences[routeRef] = struct{}{}
				r.upstreams = append(r.upstreams, upstream)
				ready[l.Name] = r
			}
		}
	}
	return ready
}

func makeDownstreamTLSContextFromSnapshotAPIListenerConfig(
	cfgSnap *proxycfg.ConfigSnapshot,
	listenerCfg structs.APIGatewayListener,
) (*envoy_tls_v3.DownstreamTlsContext, error) {
	var downstreamContext *envoy_tls_v3.DownstreamTlsContext

	tlsContext, err := makeCommonTLSContextFromSnapshotAPIGatewayListenerConfig(cfgSnap, listenerCfg)
	if err != nil {
		return nil, err
	}

	if tlsContext != nil {
		// Configure alpn protocols on TLSContext
		tlsContext.AlpnProtocols = getAlpnProtocols(string(listenerCfg.Protocol))

		downstreamContext = &envoy_tls_v3.DownstreamTlsContext{
			CommonTlsContext:         tlsContext,
			RequireClientCertificate: &wrapperspb.BoolValue{Value: false},
		}
	}

	return downstreamContext, nil
}

func makeCommonTLSContextFromSnapshotAPIGatewayListenerConfig(
	cfgSnap *proxycfg.ConfigSnapshot,
	listenerCfg structs.APIGatewayListener,
) (*envoy_tls_v3.CommonTlsContext, error) {
	var tlsContext *envoy_tls_v3.CommonTlsContext

	// API Gateway TLS config is resolved from gateway defaults plus listener overrides.
	tlsCfg, err := resolveAPIListenerTLSConfig(cfgSnap.APIGateway.TLSConfig, listenerCfg.TLS)
	if err != nil {
		return nil, err
	}

	connectTLSEnabled := !isGatewayTLSConfigEmpty(*tlsCfg) || len(listenerCfg.TLS.Certificates) > 0

	if connectTLSEnabled {
		tlsContext = makeCommonTLSContext(cfgSnap.Leaf(), cfgSnap.RootPEMs(), makeTLSParametersFromGatewayTLSConfig(*tlsCfg))
	}

	return tlsContext, nil
}

func resolveAPIListenerTLSConfig(gatewayTLSCfg structs.GatewayTLSConfig, listenerTLSCfg structs.APIGatewayTLSConfiguration) (*structs.GatewayTLSConfig, error) {
	mergedCfg := gatewayTLSCfg
	if gatewayTLSCfg.SDS != nil {
		sds := *gatewayTLSCfg.SDS
		mergedCfg.SDS = &sds
	}

	if listenerTLSCfg.SDS != nil {
		sds := *listenerTLSCfg.SDS
		if sds.ClusterName == "" && mergedCfg.SDS != nil {
			sds.ClusterName = mergedCfg.SDS.ClusterName
		}
		if sds.CertResource == "" && mergedCfg.SDS != nil {
			sds.CertResource = mergedCfg.SDS.CertResource
		}
		mergedCfg.SDS = &sds
	}
	if listenerTLSCfg.MinVersion != types.TLSVersionUnspecified {
		mergedCfg.TLSMinVersion = listenerTLSCfg.MinVersion
	}
	if listenerTLSCfg.MaxVersion != types.TLSVersionUnspecified {
		mergedCfg.TLSMaxVersion = listenerTLSCfg.MaxVersion
	}
	if len(listenerTLSCfg.CipherSuites) != 0 {
		mergedCfg.CipherSuites = listenerTLSCfg.CipherSuites
	}

	if mergedCfg.SDS != nil {
		hasCluster := mergedCfg.SDS.ClusterName != ""
		hasCertResource := mergedCfg.SDS.CertResource != ""
		if hasCertResource && !hasCluster {
			return nil, fmt.Errorf("invalid TLS.SDS configuration: ClusterName is required when CertResource is set")
		}
		if listenerTLSCfg.SDS != nil && hasCluster && !hasCertResource {
			return nil, fmt.Errorf("invalid TLS.SDS configuration: CertResource is required when ClusterName is set")
		}
	}

	if err := validateListenerTLSConfig(mergedCfg.TLSMinVersion, mergedCfg.CipherSuites); err != nil {
		return nil, err
	}

	return &mergedCfg, nil
}

func isGatewayTLSConfigEmpty(cfg structs.GatewayTLSConfig) bool {
	return !hasSDSCert(cfg.SDS) && cfg.TLSMinVersion == "" && cfg.TLSMaxVersion == "" && len(cfg.CipherSuites) == 0
}

func hasSDSCert(sds *structs.GatewayTLSSDSConfig) bool {
	return sds != nil && sds.CertResource != ""
}

// when we have multiple certificates on a single listener, we need
// to duplicate the filter chains with multiple TLS contexts
func (s *ResourceGenerator) makeInlineOverrideFilterChains(cfgSnap *proxycfg.ConfigSnapshot,
	tlsCfg structs.GatewayTLSConfig,
	serviceSDSOverrides []apiGatewayServiceSDSOverride,
	protocol string,
	filterOpts listenerFilterOpts,
	certs []structs.ConfigEntry,
) ([]*envoy_listener_v3.FilterChain, error) {
	var chains []*envoy_listener_v3.FilterChain

	constructChain := func(name string, hosts []string, tlsContext *envoy_tls_v3.CommonTlsContext) error {
		filterOpts.filterName = name
		filter, err := makeListenerFilter(filterOpts)
		if err != nil {
			return err
		}

		// Configure alpn protocols on TLSContext
		tlsContext.AlpnProtocols = getAlpnProtocols(protocol)
		transportSocket, err := makeDownstreamTLSTransportSocket(&envoy_tls_v3.DownstreamTlsContext{
			CommonTlsContext:         tlsContext,
			RequireClientCertificate: &wrapperspb.BoolValue{Value: false},
		})
		if err != nil {
			return err
		}

		chains = append(chains, &envoy_listener_v3.FilterChain{
			FilterChainMatch: makeSNIFilterChainMatch(hosts...),
			Filters: []*envoy_listener_v3.Filter{
				filter,
			},
			TransportSocket: transportSocket,
		})

		return nil
	}

	for i, override := range serviceSDSOverrides {
		overrideCfg := tlsCfg
		overrideCfg.SDS = &override.SDS
		if err := constructChain(fmt.Sprintf("service-sds-%d", i), override.Hosts, makeCommonTLSContextFromGatewayTLSConfig(overrideCfg)); err != nil {
			return nil, err
		}
	}

	for _, override := range serviceSDSOverrides {
		if len(override.Hosts) == 0 {
			// A catch-all service override (used by TCP routes) supersedes all
			// listener default certificate chains for this listener.
			return chains, nil
		}
	}

	if hasSDSCert(tlsCfg.SDS) {
		if err := constructChain("sds", nil, makeCommonTLSContextFromGatewayTLSConfig(tlsCfg)); err != nil {
			return nil, err
		}
		return chains, nil
	}

	// Separate file-system and inline certificates
	var fileSystemCerts []*structs.FileSystemCertificateConfigEntry
	var inlineCerts []*structs.InlineCertificateConfigEntry

	for _, cert := range certs {
		switch tce := cert.(type) {
		case *structs.FileSystemCertificateConfigEntry:
			fileSystemCerts = append(fileSystemCerts, tce)
		case *structs.InlineCertificateConfigEntry:
			inlineCerts = append(inlineCerts, tce)
		}
	}

	// Handle file-system certificates: consolidate into ONE filter chain with multiple SDS configs
	// This prevents duplicate empty filter chain matchers that Envoy rejects
	if len(fileSystemCerts) > 0 {
		var sdsConfigs []*envoy_tls_v3.SdsSecretConfig
		for _, cert := range fileSystemCerts {
			sdsConfigs = append(sdsConfigs, &envoy_tls_v3.SdsSecretConfig{
				// Reference the secret returned in xds/secrets.go by name
				Name: cert.GetName(),
				SdsConfig: &envoy_core_v3.ConfigSource{
					// Use ADS (Aggregated Discovery Service) to fetch secrets from Consul
					ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
						Ads: &envoy_core_v3.AggregatedConfigSource{},
					},
					ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
				},
			})
		}

		tlsContext := &envoy_tls_v3.CommonTlsContext{
			TlsParams:                      makeTLSParametersFromGatewayTLSConfig(tlsCfg),
			TlsCertificateSdsSecretConfigs: sdsConfigs,
		}

		// Create a single filter chain for all file-system certificates
		// Envoy will automatically select the correct certificate based on SNI
		if err := constructChain("file-system-certificates", nil, tlsContext); err != nil {
			return nil, err
		}

		// If we only have file-system certs, return early
		if len(inlineCerts) == 0 {
			return chains, nil
		}
	}

	// Handle inline certificates with the existing logic
	// When we have file-system certs, we need to treat single inline cert as multiple
	// to avoid duplicate catch-all filter chains
	multipleCerts := len(inlineCerts) > 1 || len(fileSystemCerts) > 0

	allCertHosts := map[string]struct{}{}
	overlappingHosts := map[string]struct{}{}

	if multipleCerts {
		// we only need to prune out overlapping hosts if we have more than
		// one certificate
		for _, cert := range inlineCerts {
			hosts, err := cert.Hosts()
			if err != nil {
				return nil, fmt.Errorf("unable to parse hosts from x509 certificate: %v", hosts)
			}
			for _, host := range hosts {
				if _, ok := allCertHosts[host]; ok {
					overlappingHosts[host] = struct{}{}
				}
				allCertHosts[host] = struct{}{}
			}
		}
	}

	for _, cert := range inlineCerts {
		var hosts []string

		// if we only have one cert, we just use it for all ingress
		if multipleCerts {
			certHosts, err := cert.Hosts()
			if err != nil {
				return nil, fmt.Errorf("unable to parse hosts from x509 certificate: %v", hosts)
			}
			// filter out any overlapping hosts so we don't have collisions in our filter chains
			for _, host := range certHosts {
				if _, ok := overlappingHosts[host]; !ok {
					hosts = append(hosts, host)
				}
			}

			if len(hosts) == 0 {
				// all of our hosts are overlapping, so we just skip this filter and it'll be
				// handled by the default filter chain
				continue
			}
		}

		tlsContext := makeInlineTLSContextFromGatewayTLSConfig(tlsCfg, cert)

		if err := constructChain(cert.GetName(), hosts, tlsContext); err != nil {
			return nil, err
		}
	}

	// Only add default chain if we have multiple inline certs AND no file-system certs
	// If we have file-system certs, they already provide the catch-all behavior
	if len(inlineCerts) > 1 && len(fileSystemCerts) == 0 {
		// if we have more than one cert, add a default handler that uses the leaf cert from connect
		if err := constructChain("default", nil, makeCommonTLSContext(cfgSnap.Leaf(), cfgSnap.RootPEMs(), makeTLSParametersFromGatewayTLSConfig(tlsCfg))); err != nil {
			return nil, err
		}
	}

	return chains, nil
}
