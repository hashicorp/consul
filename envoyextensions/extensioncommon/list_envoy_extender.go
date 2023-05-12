// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package extensioncommon

import (
	"fmt"
	"strings"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
)

type ClusterMap map[string]*envoy_cluster_v3.Cluster
type ListenerMap map[string]*envoy_listener_v3.Listener
type RouteMap map[string]*envoy_route_v3.RouteConfiguration

// ListExtension is the interface that each user of ListEnvoyExtender must implement. It
// is responsible for modifying the xDS structures based on only the state of the extension.
type ListExtension interface {
	// CanApply determines if the extension can mutate resources for the given runtime configuration.
	CanApply(*RuntimeConfig) bool

	// PatchRoutes patches routes to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	PatchRoutes(*RuntimeConfig, RouteMap) (RouteMap, error)

	// PatchClusters patches clusters to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	PatchClusters(*RuntimeConfig, ClusterMap) (ClusterMap, error)

	// PatchFilters patches Envoy filters to include the custom Envoy
	// configuration required to integrate with the built in extension template.
	PatchFilters(*RuntimeConfig, []*envoy_listener_v3.Filter) ([]*envoy_listener_v3.Filter, error)
}

var _ EnvoyExtender = (*ListEnvoyExtender)(nil)

// ListEnvoyExtender provides convenience functions for iterating and applying modifications
// to lists of Envoy resources.
type ListEnvoyExtender struct {
	Extension ListExtension
}

func (*ListEnvoyExtender) Validate(config *RuntimeConfig) error {
	return nil
}

func (e *ListEnvoyExtender) Extend(resources *xdscommon.IndexedResources, config *RuntimeConfig) (*xdscommon.IndexedResources, error) {
	var resultErr error

	switch config.Kind {
	case api.ServiceKindTerminatingGateway, api.ServiceKindConnectProxy:
	default:
		return resources, nil
	}

	if !e.Extension.CanApply(config) {
		return resources, nil
	}

	clusters := make(ClusterMap)
	routes := make(RouteMap)
	listeners := make(ListenerMap)
	isUpstream := config.IsUpstream()

	for _, indexType := range []string{
		xdscommon.ListenerType,
		xdscommon.RouteType,
		xdscommon.ClusterType,
	} {
		for nameOrSNI, msg := range resources.Index[indexType] {
			switch resource := msg.(type) {
			case *envoy_cluster_v3.Cluster:
				// If the Envoy extension configuration is for an upstream service, the Cluster's
				// name must match the upstream service's SNI.
				if isUpstream && !config.MatchesUpstreamServiceSNI(nameOrSNI) {
					continue
				}

				// If the extension's config is for an an inbound listener, the Cluster's name
				// must be xdscommon.LocalAppClusterName.
				if !isUpstream && nameOrSNI == xdscommon.LocalAppClusterName {
					continue
				}

				clusters[nameOrSNI] = resource

			case *envoy_listener_v3.Listener:
				listeners[nameOrSNI] = resource

			case *envoy_route_v3.RouteConfiguration:
				// If the Envoy extension configuration is for an upstream service, the route's
				// name must match the upstream service's Envoy ID.
				matchesEnvoyID := config.EnvoyID() == nameOrSNI
				if isUpstream && !config.MatchesUpstreamServiceSNI(nameOrSNI) && !matchesEnvoyID {
					continue
				}

				// There aren't routes for inbound services.
				if !isUpstream {
					continue
				}

				routes[nameOrSNI] = resource

			default:
				resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported type was skipped: %T", resource))
			}
		}
	}

	patchedClusters, err := e.Extension.PatchClusters(config, clusters)
	if err == nil {
		for nameOrSNI, cluster := range patchedClusters {
			resources.Index[xdscommon.ClusterType][nameOrSNI] = cluster
		}
	} else {
		resultErr = multierror.Append(resultErr, fmt.Errorf("error patching clusters: %w", err))
	}

	patchedListeners, err := e.patchListeners(config, listeners)
	if err == nil {
		for nameOrSNI, listener := range patchedListeners {
			resources.Index[xdscommon.ListenerType][nameOrSNI] = listener
		}
	} else {
		resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listeners: %w", err))
	}

	patchedRoutes, err := e.Extension.PatchRoutes(config, routes)
	if err == nil {
		for nameOrSNI, route := range patchedRoutes {
			resources.Index[xdscommon.RouteType][nameOrSNI] = route
		}
	} else {
		resultErr = multierror.Append(resultErr, fmt.Errorf("error patching routes: %w", err))
	}

	return resources, resultErr
}

func (e ListEnvoyExtender) patchListeners(config *RuntimeConfig, m ListenerMap) (ListenerMap, error) {
	switch config.Kind {
	case api.ServiceKindTerminatingGateway:
		return e.patchTerminatingGatewayListeners(config, m)
	case api.ServiceKindConnectProxy:
		return e.patchConnectProxyListeners(config, m)
	}
	return m, nil
}

func (e ListEnvoyExtender) patchTerminatingGatewayListeners(config *RuntimeConfig, l ListenerMap) (ListenerMap, error) {
	// We don't support directly targeting terminating gateways with extensions.
	if !config.IsUpstream() {
		return l, nil
	}

	var resultErr error
	for _, listener := range l {
		for _, filterChain := range listener.FilterChains {
			sni := getSNI(filterChain)

			if sni == "" {
				continue
			}

			// The filter chain's SNI must match the upstream service's SNI.
			if !config.MatchesUpstreamServiceSNI(sni) {
				continue
			}

			patchedFilters, err := e.Extension.PatchFilters(config, filterChain.Filters)
			if err == nil {
				filterChain.Filters = patchedFilters
			} else {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filters for %q: %w", sni, err))
				continue
			}
		}
	}

	return l, resultErr
}

func (e ListEnvoyExtender) patchConnectProxyListeners(config *RuntimeConfig, l ListenerMap) (ListenerMap, error) {
	var resultErr error

	isUpstream := config.IsUpstream()
	for nameOrSNI, listener := range l {
		envoyID := ""
		if id, _, found := strings.Cut(listener.Name, ":"); found {
			envoyID = id
		}

		if isUpstream && envoyID == xdscommon.OutboundListenerName {
			patchedListener, err := e.patchTProxyListener(config, listener)
			if err == nil {
				l[nameOrSNI] = patchedListener
			} else {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching TProxy listener %q: %w", nameOrSNI, err))
			}
			continue
		}

		// If the Envoy extension configuration is for an upstream service, the listener's
		// name must match the upstream service's EnvoyID or be the outbound listener.
		if isUpstream && envoyID != config.EnvoyID() {
			continue
		}

		// If the Envoy extension configuration is for inbound resources, the
		// listener must be named xdscommon.PublicListenerName.
		if config.IsLocal() && envoyID != xdscommon.PublicListenerName {
			continue
		}

		patchedListener, err := e.patchConnectProxyListener(config, listener)
		if err == nil {
			l[nameOrSNI] = patchedListener
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching connect proxy listener %q: %w", nameOrSNI, err))
		}
	}

	return l, resultErr
}

func (e ListEnvoyExtender) patchConnectProxyListener(config *RuntimeConfig, l *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, error) {
	var resultErr error

	for _, filterChain := range l.FilterChains {
		patchedFilters, err := e.Extension.PatchFilters(config, filterChain.Filters)
		if err == nil {
			filterChain.Filters = patchedFilters
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching filters: %w", err))
		}
	}
	return l, resultErr
}

func (e ListEnvoyExtender) patchTProxyListener(config *RuntimeConfig, l *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, error) {
	var resultErr error

	vip := config.Upstreams[config.ServiceName].VIP

	for _, filterChain := range l.FilterChains {
		match := filterChainTProxyMatch(vip, filterChain)
		if !match {
			continue
		}

		patchedFilters, err := e.Extension.PatchFilters(config, filterChain.Filters)
		if err == nil {
			filterChain.Filters = patchedFilters
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching filters: %w", err))
		}
	}

	return l, resultErr
}
