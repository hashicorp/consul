// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"github.com/hashicorp/consul/lib"

	"golang.org/x/exp/maps"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_http_jwt_authn_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"

	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/xds/naming"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

func (s *ResourceGenerator) makeAPIGatewayListeners(address string, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message

	readyListeners := getReadyListeners(cfgSnap)

	for _, readyListener := range readyListeners {
		listenerCfg := readyListener.listenerCfg
		listenerKey := readyListener.listenerKey
		boundListener := readyListener.boundListenerCfg

		// Collect the referenced certificate config entries
		var certs []structs.ConfigEntry
		for _, certRef := range boundListener.Certificates {
			switch certRef.Kind {
			case structs.InlineCertificate:
				if cert, ok := cfgSnap.APIGateway.Certificates.Get(certRef); ok {
					certs = append(certs, cert)
				}
			case structs.FileSystemCertificate:
				if cert, ok := cfgSnap.APIGateway.FSCertificates.Get(certRef); ok {
					certs = append(certs, cert)
				}
			}
		}

		isAPIGatewayWithTLS := len(boundListener.Certificates) > 0

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
				// construct SNI filter chains
				l.FilterChains, err = makeInlineOverrideFilterChains(
					cfgSnap,
					listenerCfg.TLS,
					listenerKey.Protocol,
					listenerFilterOpts{
						useRDS:          useRDS,
						fetchTimeoutRDS: cfgSnap.GetXDSCommonConfig(s.Logger).GetXDSFetchTimeout(),
						protocol:        listenerKey.Protocol,
						routeName:       listenerKey.RouteName(),
						cluster:         clusterName,
						statPrefix:      "ingress_upstream_",
						accessLogs:      &cfgSnap.Proxy.AccessLogs,
						logger:          s.Logger,
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

			filterOpts := listenerFilterOpts{
				useRDS:           true,
				fetchTimeoutRDS:  cfgSnap.GetXDSCommonConfig(s.Logger).GetXDSFetchTimeout(),
				protocol:         listenerKey.Protocol,
				filterName:       listenerKey.RouteName(),
				routeName:        listenerKey.RouteName(),
				cluster:          "",
				statPrefix:       "ingress_upstream_",
				routePath:        "",
				httpAuthzFilters: authFilters,
				accessLogs:       &cfgSnap.Proxy.AccessLogs,
				logger:           s.Logger,
			}

			// Generate any filter chains needed for services with custom TLS certs
			// via SDS.
			sniFilterChains := []*envoy_listener_v3.FilterChain{}

			if isAPIGatewayWithTLS {
				sniFilterChains, err = makeInlineOverrideFilterChains(cfgSnap, listenerCfg.TLS, listenerKey.Protocol, filterOpts, certs)
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

	// API Gateway TLS config is per listener
	tlsCfg, err := resolveAPIListenerTLSConfig(listenerCfg.TLS)
	if err != nil {
		return nil, err
	}

	connectTLSEnabled := (!listenerCfg.TLS.IsEmpty())

	if connectTLSEnabled {
		tlsContext = makeCommonTLSContext(cfgSnap.Leaf(), cfgSnap.RootPEMs(), makeTLSParametersFromGatewayTLSConfig(*tlsCfg))
	}

	return tlsContext, nil
}

func resolveAPIListenerTLSConfig(listenerTLSCfg structs.APIGatewayTLSConfiguration) (*structs.GatewayTLSConfig, error) {
	var mergedCfg structs.GatewayTLSConfig

	if !listenerTLSCfg.IsEmpty() {
		if listenerTLSCfg.MinVersion != types.TLSVersionUnspecified {
			mergedCfg.TLSMinVersion = listenerTLSCfg.MinVersion
		}
		if listenerTLSCfg.MaxVersion != types.TLSVersionUnspecified {
			mergedCfg.TLSMaxVersion = listenerTLSCfg.MaxVersion
		}
		if len(listenerTLSCfg.CipherSuites) != 0 {
			mergedCfg.CipherSuites = listenerTLSCfg.CipherSuites
		}
	}

	if err := validateListenerTLSConfig(mergedCfg.TLSMinVersion, mergedCfg.CipherSuites); err != nil {
		return nil, err
	}

	return &mergedCfg, nil
}

// when we have multiple certificates on a single listener, we need
// to duplicate the filter chains with multiple TLS contexts
func makeInlineOverrideFilterChains(cfgSnap *proxycfg.ConfigSnapshot,
	tlsCfg structs.APIGatewayTLSConfiguration,
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

	constructTLSContext := func(tlsConfig structs.APIGatewayTLSConfiguration, certConfig structs.ConfigEntry) (*envoy_tls_v3.CommonTlsContext, error) {
		switch tce := certConfig.(type) {
		case *structs.InlineCertificateConfigEntry:
			return &envoy_tls_v3.CommonTlsContext{
				TlsParams: makeTLSParametersFromGatewayTLSConfig(tlsCfg.ToGatewayTLSConfig()),
				TlsCertificates: []*envoy_tls_v3.TlsCertificate{
					{
						CertificateChain: &envoy_core_v3.DataSource{
							Specifier: &envoy_core_v3.DataSource_InlineString{
								InlineString: lib.EnsureTrailingNewline(tce.Certificate),
							},
						},
						PrivateKey: &envoy_core_v3.DataSource{
							Specifier: &envoy_core_v3.DataSource_InlineString{
								InlineString: lib.EnsureTrailingNewline(tce.PrivateKey),
							},
						},
					},
				},
			}, nil
		case *structs.FileSystemCertificateConfigEntry:
			ctx := makeCommonTLSContextFromFiles("", tce.Certificate, tce.PrivateKey)
			ctx.TlsParams = makeTLSParametersFromGatewayTLSConfig(tlsCfg.ToGatewayTLSConfig())
			return ctx, nil
		default:
			return nil, fmt.Errorf("unsupported config entry kind %s", tce.GetKind())
		}
	}

	for _, cert := range certs {
		// TODO Delivering via SDS is semi-required in order to get file system watches today.
		//   https://github.com/envoyproxy/envoy/issues/10387
		tlsContext, err := constructTLSContext(tlsCfg, cert)
		if err != nil {
			continue
		}

		if err := constructChain(cert.GetName(), nil, tlsContext); err != nil {
			return nil, err
		}
	}

	if len(certs) > 1 {
		// if we have more than one cert, add a default handler that uses the leaf cert from connect
		if err := constructChain("default", nil, makeCommonTLSContext(cfgSnap.Leaf(), cfgSnap.RootPEMs(), makeTLSParametersFromGatewayTLSConfig(tlsCfg.ToGatewayTLSConfig()))); err != nil {
			return nil, err
		}
	}

	return chains, nil
}
