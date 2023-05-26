// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package extensioncommon

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

// BasicExtensionAdapter is an adapter that provides default implementations for all of the EnvoyExtension
// interface functions. Extension implementations can extend the adapter and implement only the functions
// they are interested in. At a minimum, extensions must override the adapter's CanApply and Validate
// functions.
type BasicExtensionAdapter struct{}

// CanApply provides a default implementation of the CanApply interface that always returns false.
func (BasicExtensionAdapter) CanApply(_ *RuntimeConfig) bool { return false }

// PatchCluster provides a default implementation of the PatchCluster interface that does nothing.
func (BasicExtensionAdapter) PatchCluster(_ *RuntimeConfig, c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	return c, false, nil
}

// PatchClusters provides a default implementation of the PatchClusters interface that does nothing.
func (BasicExtensionAdapter) PatchClusters(_ *RuntimeConfig, c ClusterMap) (ClusterMap, error) {
	return c, nil
}

// PatchListener provides a default implementation of the PatchListener interface that does nothing.
func (BasicExtensionAdapter) PatchListener(_ *RuntimeConfig, l *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	return l, false, nil
}

// PatchListeners provides a default implementation of the PatchListeners interface that does nothing.
func (BasicExtensionAdapter) PatchListeners(_ *RuntimeConfig, l ListenerMap) (ListenerMap, error) {
	return l, nil
}

// PatchFilter provides a default implementation of the PatchFilter interface that does nothing.
func (BasicExtensionAdapter) PatchFilter(_ *RuntimeConfig, f *envoy_listener_v3.Filter, _ bool) (*envoy_listener_v3.Filter, bool, error) {
	return f, false, nil
}

// PatchFilters provides a default implementation of the PatchFilters interface that does nothing.
func (BasicExtensionAdapter) PatchFilters(_ *RuntimeConfig, f []*envoy_listener_v3.Filter, _ bool) ([]*envoy_listener_v3.Filter, error) {
	return f, nil
}

// PatchRoute provides a default implementation of the PatchRoute interface that does nothing.
func (BasicExtensionAdapter) PatchRoute(_ *RuntimeConfig, r *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	return r, false, nil
}

// PatchRoutes provides a default implementation of the PatchRoutes interface that does nothing.
func (BasicExtensionAdapter) PatchRoutes(_ *RuntimeConfig, r RouteMap) (RouteMap, error) {
	return r, nil
}

// Validate provides a default implementation of the Validate interface that always returns nil.
func (BasicExtensionAdapter) Validate(_ *RuntimeConfig) error { return nil }
