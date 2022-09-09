package serverlessplugin

import (
	"fmt"
	"strings"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

// MutateIndexedResources updates indexed xDS structures to include patches for
// serverless integrations. It is responsible for constructing all of the
// patchers and forwarding xDS structs onto the appropriate patcher. If any
// portion of this function fails, it will record the error and continue. The
// behavior is appropriate since the unpatched xDS structures this receives are
// typically invalid.
func MutateIndexedResources(resources *xdscommon.IndexedResources, config xdscommon.PluginConfiguration) (*xdscommon.IndexedResources, error) {
	var resultErr error

	switch config.Kind {
	case api.ServiceKindTerminatingGateway, api.ServiceKindConnectProxy:
	default:
		return resources, resultErr
	}

	// Patch clusters
	for sni, msg := range resources.Index[xdscommon.ClusterType] {
		patched := false
		cluster, ok := msg.(*envoy_cluster_v3.Cluster)
		if !ok {
			resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported type was skipped. Not a cluster: %T", msg))
			continue
		}
		for _, patcher := range getPatchersBySNI(config, sni) {
			c, ok, err := patcher.PatchCluster(cluster)
			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching cluster: %w", err))
				continue
			}
			cluster = c
			patched = patched || ok

		}
		if patched {
			resources.Index[xdscommon.ClusterType][sni] = cluster
		}
	}

	// Patch listeners and filters
	for name, msg := range resources.Index[xdscommon.ListenerType] {
		listener, ok := msg.(*envoy_listener_v3.Listener)
		if !ok {
			resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported type was skipped. Not a listener: %T", msg))
			continue
		}

		// Patch listeners
		l, ok, err := patchListener(config, listener)
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener: %w", err))
		}
		if ok {
			listener = l
			resources.Index[xdscommon.ListenerType][name] = listener
		}

		// Patch filters
		l, ok, err = patchFilters(config, listener)
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching filter: %w", err))
		}
		if ok {
			listener = l
			resources.Index[xdscommon.ListenerType][name] = listener
		}
	}

	// Patch routes
	for sni, msg := range resources.Index[xdscommon.RouteType] {
		patched := false
		route, ok := msg.(*envoy_route_v3.RouteConfiguration)
		if !ok {
			resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported type was skipped. Not a route: %T", msg))
			continue
		}
		for _, patcher := range getPatchersBySNI(config, sni) {
			r, ok, err := patcher.PatchRoute(route)
			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching route: %w", err))
				continue
			}
			route = r
			patched = patched || ok
		}
		if patched {
			resources.Index[xdscommon.RouteType][sni] = route
		}
	}

	return resources, resultErr
}

func patchListener(config xdscommon.PluginConfiguration, l *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	switch config.Kind {
	case api.ServiceKindTerminatingGateway:
		return patchTerminatingGatewayListener(config, l)
	case api.ServiceKindConnectProxy:
		return patchConnectProxyListener(config, l)
	}
	return l, false, nil
}

func patchFilters(config xdscommon.PluginConfiguration, l *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	switch config.Kind {
	case api.ServiceKindTerminatingGateway:
		return patchTerminatingGatewayFilters(config, l)
	case api.ServiceKindConnectProxy:
		return patchConnectProxyFilters(config, l)
	}
	return l, false, nil
}

func patchConnectProxyListener(config xdscommon.PluginConfiguration, listener *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	var resultErr error
	var patched bool

	envoyID := getEnvoyIDFromListenerName(listener)
	for _, patcher := range getPatchersByEnvoyID(config, envoyID) {
		l, ok, err := patcher.PatchListener(listener)
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener: %w", err))
			continue
		}
		if ok {
			patched = true
			listener = l
		}
	}
	return listener, patched, resultErr
}

func patchTerminatingGatewayListener(config xdscommon.PluginConfiguration, listener *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	var resultErr error
	var patched bool

	for _, filterChain := range listener.FilterChains {
		sni := getSNI(filterChain)
		if sni == "" {
			continue
		}

		patchers := getPatchersBySNI(config, sni)
		if len(patchers) == 0 {
			continue
		}

		for _, patcher := range patchers {
			l, ok, err := patcher.PatchListener(listener)
			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener: %w", err))
			}
			if ok {
				patched = true
				listener = l
			}
		}
	}

	return listener, patched, resultErr
}

func patchConnectProxyFilters(config xdscommon.PluginConfiguration, listener *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	var resultErr error
	var patched bool

	envoyID := getEnvoyIDFromListenerName(listener)
	for _, patcher := range getPatchersByEnvoyID(config, envoyID) {
		for _, filterChain := range listener.FilterChains {
			var filters []*envoy_listener_v3.Filter

			for _, filter := range filterChain.Filters {
				newFilter, ok, err := patcher.PatchFilter(filter)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter: %w", err))
				}
				if ok {
					patched = true
					filters = append(filters, newFilter)
				} else {
					filters = append(filters, filter)
				}
			}
			filterChain.Filters = filters
		}
	}

	return listener, patched, resultErr
}

func patchTerminatingGatewayFilters(config xdscommon.PluginConfiguration, listener *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	var resultErr error
	var patched bool

	for _, filterChain := range listener.FilterChains {
		sni := getSNI(filterChain)
		if sni == "" {
			continue
		}

		patchers := getPatchersBySNI(config, sni)
		if len(patchers) == 0 {
			continue
		}

		for _, patcher := range patchers {
			var filters []*envoy_listener_v3.Filter
			for _, filter := range filterChain.Filters {
				newFilter, ok, err := patcher.PatchFilter(filter)

				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter: %w", err))
				}
				if ok {
					patched = true
					filters = append(filters, newFilter)
				} else {
					filters = append(filters, filter)
				}
			}
			filterChain.Filters = filters
		}
	}

	return listener, patched, resultErr
}

func getSNI(chain *envoy_listener_v3.FilterChain) string {
	var sni string

	if chain == nil {
		return sni
	}

	if chain.FilterChainMatch == nil {
		return sni
	}

	if len(chain.FilterChainMatch.ServerNames) == 0 {
		return sni
	}

	return chain.FilterChainMatch.ServerNames[0]
}

func getEnvoyIDFromListenerName(l *envoy_listener_v3.Listener) string {
	if i := strings.IndexByte(l.Name, ':'); i != -1 {
		return l.Name[:i]
	}
	return ""
}
