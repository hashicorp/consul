// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package extensioncommon

import (
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
)

// ClusterMap is a map of clusters indexed by name.
type ClusterMap map[string]*envoy_cluster_v3.Cluster

// ClusterLoadAssignmentMap is a map of cluster load assignments indexed by name.
type ClusterLoadAssignmentMap map[string]*envoy_endpoint_v3.ClusterLoadAssignment

// ListenerMap is a map of listeners indexed by name.
type ListenerMap map[string]*envoy_listener_v3.Listener

// RouteMap is a map of routes indexed by name.
type RouteMap map[string]*envoy_route_v3.RouteConfiguration

// BasicExtension is the interface that each user of BasicEnvoyExtender must implement. It
// is responsible for modifying the xDS structures based on only the state of
// the extension.
type BasicExtension interface {
	// CanApply determines if the extension can mutate resources for the given runtime configuration.
	CanApply(*RuntimeConfig) bool

	// PatchRoute patches a route to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	// See also PatchRoutes.
	PatchRoute(RoutePayload) (*envoy_route_v3.RouteConfiguration, bool, error)

	// PatchRoutes patches routes to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	// This allows extensions to operate on a collection of routes.
	// For extensions that implement both PatchRoute and PatchRoutes,
	// PatchRoutes is always called first with the entire collection of routes.
	// Then PatchRoute is called for each individual route.
	PatchRoutes(*RuntimeConfig, RouteMap) (RouteMap, error)

	// PatchCluster patches a cluster to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	// See also PatchClusters.
	PatchCluster(ClusterPayload) (*envoy_cluster_v3.Cluster, bool, error)

	// PatchClusters patches clusters to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	// This allows extensions to operate on a collection of clusters.
	// For extensions that implement both PatchCluster and PatchClusters,
	// PatchClusters is always called first with the entire collection of clusters.
	// Then PatchClusters is called for each individual cluster.
	PatchClusters(*RuntimeConfig, ClusterMap) (ClusterMap, error)

	// PatchClusterLoadAssignment patches a cluster load assignment to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	PatchClusterLoadAssignment(ClusterLoadAssignmentPayload) (*envoy_endpoint_v3.ClusterLoadAssignment, bool, error)

	// PatchListener patches a listener to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	// See also PatchListeners.
	PatchListener(ListenerPayload) (*envoy_listener_v3.Listener, bool, error)

	// PatchListeners patches listeners to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	// This allows extensions to operate on a collection of listeners.
	// For extensions that implement both PatchListener and PatchListeners,
	// PatchListeners is always called first with the entire collection of listeners.
	// Then PatchListeners is called for each individual listener.
	PatchListeners(*RuntimeConfig, ListenerMap) (ListenerMap, error)

	// PatchFilter patches an Envoy filter to include the custom Envoy
	// configuration required to integrate with the built in extension template.
	// See also PatchFilters.
	PatchFilter(FilterPayload) (*envoy_listener_v3.Filter, bool, error)

	// PatchFilters patches Envoy filters to include the custom Envoy
	// configuration required to integrate with the built in extension template.
	// This allows extensions to operate on a collection of filters.
	// For extensions that implement both PatchFilter and PatchFilters,
	// PatchFilters is always called first with the entire collection of filters.
	// Then PatchFilter is called for each individual filter.
	PatchFilters(cfg *RuntimeConfig, f []*envoy_listener_v3.Filter, isInboundListener bool) ([]*envoy_listener_v3.Filter, error)

	// Validate determines if the runtime configuration provided is valid for the extension.
	Validate(*RuntimeConfig) error
}

var _ EnvoyExtender = (*BasicEnvoyExtender)(nil)

// BasicEnvoyExtender provides convenience functions for iterating and applying modifications
// to Envoy resources.
type BasicEnvoyExtender struct {
	Extension BasicExtension
}

func (b *BasicEnvoyExtender) CanApply(config *RuntimeConfig) bool {
	return b.Extension.CanApply(config)
}

func (b *BasicEnvoyExtender) Validate(config *RuntimeConfig) error {
	return b.Extension.Validate(config)
}

func (b *BasicEnvoyExtender) Extend(resources *xdscommon.IndexedResources, config *RuntimeConfig) (*xdscommon.IndexedResources, error) {
	var resultErr error

	// We don't support patching the local proxy with an upstream's config except in special
	// cases supported by UpstreamEnvoyExtender.
	if config.IsSourcedFromUpstream {
		return nil, fmt.Errorf("%q extension applied as local config but is sourced from an upstream of the local service", config.EnvoyExtension.Name)
	}

	switch config.Kind {
	// Currently we only support extensions for terminating gateways and connect proxies.
	case api.ServiceKindTerminatingGateway, api.ServiceKindConnectProxy:
	default:
		return resources, nil
	}

	clusters := make(ClusterMap)
	clusterLoadAssignments := make(ClusterLoadAssignmentMap)
	routes := make(RouteMap)
	listeners := make(ListenerMap)

	for _, indexType := range []string{
		xdscommon.ListenerType,
		xdscommon.RouteType,
		xdscommon.ClusterType,
		xdscommon.EndpointType,
	} {
		for nameOrSNI, msg := range resources.Index[indexType] {
			switch resource := msg.(type) {
			case *envoy_cluster_v3.Cluster:
				clusters[nameOrSNI] = resource
			case *envoy_endpoint_v3.ClusterLoadAssignment:
				clusterLoadAssignments[nameOrSNI] = resource
			case *envoy_listener_v3.Listener:
				listeners[nameOrSNI] = resource
			case *envoy_route_v3.RouteConfiguration:
				routes[nameOrSNI] = resource
			default:
				resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported type was skipped: %T", resource))
			}
		}
	}

	if patchedClusters, err := b.patchClusters(config, clusters); err == nil {
		for k, v := range patchedClusters {
			resources.Index[xdscommon.ClusterType][k] = v
		}
	} else {
		resultErr = multierror.Append(resultErr, err)
	}

	if patchedClusterLoadAssignments, err := b.patchClusterLoadAssignments(config, clusterLoadAssignments); err == nil {
		for k, v := range patchedClusterLoadAssignments {
			resources.Index[xdscommon.EndpointType][k] = v
		}
	} else {
		resultErr = multierror.Append(resultErr, err)
	}

	if patchedListeners, err := b.patchListeners(config, listeners); err == nil {
		for k, v := range patchedListeners {
			resources.Index[xdscommon.ListenerType][k] = v
		}
	} else {
		resultErr = multierror.Append(resultErr, err)
	}

	if patchedRoutes, err := b.patchRoutes(config, routes); err == nil {
		for k, v := range patchedRoutes {
			resources.Index[xdscommon.RouteType][k] = v
		}
	} else {
		resultErr = multierror.Append(resultErr, err)
	}

	return resources, resultErr
}

func (b *BasicEnvoyExtender) patchClusters(config *RuntimeConfig, clusters ClusterMap) (ClusterMap, error) {
	var resultErr error

	patchedClusters, err := b.Extension.PatchClusters(config, clusters)
	if err != nil {
		return clusters, fmt.Errorf("error patching clusters: %w", err)
	}
	for nameOrSNI, cluster := range clusters {
		patchedCluster, patched, err := b.Extension.PatchCluster(config.GetClusterPayload(cluster))
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching cluster %q: %w", nameOrSNI, err))
		}
		if !patched {
			patchedCluster = cluster
		}

		// We patch cluster load assignments directly above for EDS, but also here for CDS,
		// since updates can come from either.
		if patchedCluster.LoadAssignment != nil {
			patchedClusterLoadAssignment, patched, err := b.Extension.PatchClusterLoadAssignment(config.GetClusterLoadAssignmentPayload(patchedCluster.LoadAssignment))
			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching load assignment for cluster %q: %w", nameOrSNI, err))
			} else if patched {
				patchedCluster.LoadAssignment = patchedClusterLoadAssignment
			}
		}

		patchedClusters[nameOrSNI] = patchedCluster
	}
	return patchedClusters, resultErr
}

func (b *BasicEnvoyExtender) patchClusterLoadAssignments(config *RuntimeConfig, clusterLoadAssignments ClusterLoadAssignmentMap) (ClusterLoadAssignmentMap, error) {
	var resultErr error

	for nameOrSNI, clusterLoadAssignment := range clusterLoadAssignments {
		patchedClusterLoadAssignment, patched, err := b.Extension.PatchClusterLoadAssignment(config.GetClusterLoadAssignmentPayload(clusterLoadAssignment))
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching cluster load assignment %q: %w", nameOrSNI, err))
		}
		if patched {
			clusterLoadAssignments[nameOrSNI] = patchedClusterLoadAssignment
		} else {
			clusterLoadAssignments[nameOrSNI] = clusterLoadAssignment
		}
	}
	return clusterLoadAssignments, resultErr
}

func (b *BasicEnvoyExtender) patchRoutes(config *RuntimeConfig, routes RouteMap) (RouteMap, error) {
	var resultErr error

	patchedRoutes, err := b.Extension.PatchRoutes(config, routes)
	if err != nil {
		return routes, fmt.Errorf("error patching routes: %w", err)
	}
	for nameOrSNI, route := range patchedRoutes {
		patchedRoute, patched, err := b.Extension.PatchRoute(config.GetRoutePayload(route))
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching route %q: %w", nameOrSNI, err))
		}
		if patched {
			patchedRoutes[nameOrSNI] = patchedRoute
		} else {
			patchedRoutes[nameOrSNI] = route
		}
	}
	return patchedRoutes, resultErr
}

func (b *BasicEnvoyExtender) patchListeners(config *RuntimeConfig, listeners ListenerMap) (ListenerMap, error) {
	var resultErr error

	patchedListeners, err := b.Extension.PatchListeners(config, listeners)
	if err != nil {
		return listeners, fmt.Errorf("error patching listeners: %w", err)
	}
	for nameOrSNI, listener := range listeners {
		patchedListener, patched, err := b.Extension.PatchListener(config.GetListenerPayload(listener))
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener %q: %w", nameOrSNI, err))
		}
		if !patched {
			patchedListener = listener
		}

		if patchedListener, err = b.patchSupportedListenerFilterChains(config, patchedListener, nameOrSNI); err == nil {
			patchedListeners[nameOrSNI] = patchedListener
		} else {
			resultErr = multierror.Append(resultErr, err)
			patchedListeners[nameOrSNI] = listener
		}
	}
	return patchedListeners, resultErr
}

func (b *BasicEnvoyExtender) patchSupportedListenerFilterChains(config *RuntimeConfig, l *envoy_listener_v3.Listener, nameOrSNI string) (*envoy_listener_v3.Listener, error) {
	switch config.Kind {
	case api.ServiceKindTerminatingGateway, api.ServiceKindConnectProxy:
		return b.patchListenerFilterChains(config, l, nameOrSNI)
	}
	return l, nil
}

func (b *BasicEnvoyExtender) patchListenerFilterChains(config *RuntimeConfig, l *envoy_listener_v3.Listener, nameOrSNI string) (*envoy_listener_v3.Listener, error) {
	var resultErr error

	for idx, filterChain := range l.FilterChains {
		if patchedFilterChain, err := b.patchFilterChain(config, filterChain, l); err == nil {
			l.FilterChains[idx] = patchedFilterChain
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter chain %q: %w", nameOrSNI, err))
		}
	}

	return l, resultErr
}

func (b *BasicEnvoyExtender) patchFilterChain(config *RuntimeConfig, filterChain *envoy_listener_v3.FilterChain, l *envoy_listener_v3.Listener) (*envoy_listener_v3.FilterChain, error) {
	var resultErr error
	inbound := IsInboundPublicListener(l)
	patchedFilters, err := b.Extension.PatchFilters(config, filterChain.Filters, inbound)
	if err != nil {
		return filterChain, fmt.Errorf("error patching filters: %w", err)
	}
	for idx, filter := range patchedFilters {
		patchedFilter, patched, err := b.Extension.PatchFilter(config.GetFilterPayload(filter, l))
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching filter: %w", err))
		}
		if patched {
			patchedFilters[idx] = patchedFilter
		} else {
			patchedFilters[idx] = filter
		}
	}
	filterChain.Filters = patchedFilters
	return filterChain, err
}
