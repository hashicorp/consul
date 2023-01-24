package builtinextensiontemplate

import (
	"fmt"
	"strings"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

type EnvoyExtension struct {
	Constructor PluginConstructor
	Plugin      Plugin
	ready       bool
}

var _ xdscommon.EnvoyExtension = (*EnvoyExtension)(nil)

// Validate ensures the data in ExtensionConfiguration can successfuly be used
// to apply the specified Envoy extension.
func (envoyExtension *EnvoyExtension) Validate(config xdscommon.ExtensionConfiguration) error {
	plugin, err := envoyExtension.Constructor(config)

	envoyExtension.Plugin = plugin
	envoyExtension.ready = err == nil

	return err
}

// Extend updates indexed xDS structures to include patches for
// built-in extensions. It is responsible for applying Plugins to
// the the appropriate xDS resources. If any portion of this function fails,
// it will attempt to continue and return an error. The caller can then determine
// if it is better to use a partially applied extension or error out.
func (envoyExtension *EnvoyExtension) Extend(resources *xdscommon.IndexedResources, config xdscommon.ExtensionConfiguration) (*xdscommon.IndexedResources, error) {
	if !envoyExtension.ready {
		panic("envoy extension used without being properly constructed")
	}

	var resultErr error

	switch config.Kind {
	case api.ServiceKindTerminatingGateway, api.ServiceKindConnectProxy:
	default:
		return resources, nil
	}

	if !envoyExtension.Plugin.CanApply(config) {
		return resources, nil
	}

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
				if config.IsUpstream() && !config.MatchesUpstreamServiceSNI(nameOrSNI) {
					continue
				}

				// If the extension's config is for an an inbound listener, the Cluster's name
				// must be xdscommon.LocalAppClusterName.
				if !config.IsUpstream() && nameOrSNI == xdscommon.LocalAppClusterName {
					continue
				}

				newCluster, patched, err := envoyExtension.Plugin.PatchCluster(resource)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching cluster: %w", err))
					continue
				}
				if patched {
					resources.Index[xdscommon.ClusterType][nameOrSNI] = newCluster
				}

			case *envoy_listener_v3.Listener:
				newListener, patched, err := envoyExtension.patchListener(config, resource)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener: %w", err))
					continue
				}
				if patched {
					resources.Index[xdscommon.ListenerType][nameOrSNI] = newListener
				}

			case *envoy_route_v3.RouteConfiguration:
				// If the Envoy extension configuration is for an upstream service, the route's
				// name must match the upstream service's Envoy ID.
				matchesEnvoyID := config.EnvoyID() == nameOrSNI
				if config.IsUpstream() && !config.MatchesUpstreamServiceSNI(nameOrSNI) && !matchesEnvoyID {
					continue
				}

				// There aren't routes for inbound services.
				if !config.IsUpstream() {
					continue
				}

				newRoute, patched, err := envoyExtension.Plugin.PatchRoute(resource)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching route: %w", err))
					continue
				}
				if patched {
					resources.Index[xdscommon.RouteType][nameOrSNI] = newRoute
				}
			default:
				resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported type was skipped: %T", resource))
			}
		}
	}

	return resources, resultErr
}

func (envoyExtension EnvoyExtension) patchListener(config xdscommon.ExtensionConfiguration, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	switch config.Kind {
	case api.ServiceKindTerminatingGateway:
		return envoyExtension.patchTerminatingGatewayListener(config, l)
	case api.ServiceKindConnectProxy:
		return envoyExtension.patchConnectProxyListener(config, l)
	}
	return l, false, nil
}

func (envoyExtension EnvoyExtension) patchTerminatingGatewayListener(config xdscommon.ExtensionConfiguration, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	// We don't support directly targeting terminating gateways with extensions.
	if !config.IsUpstream() {
		return l, false, nil
	}

	var resultErr error
	patched := false
	for _, filterChain := range l.FilterChains {
		sni := getSNI(filterChain)

		if sni == "" {
			continue
		}

		// The filter chain's SNI must match the upstream service's SNI.
		if !config.MatchesUpstreamServiceSNI(sni) {
			continue
		}

		var filters []*envoy_listener_v3.Filter

		for _, filter := range filterChain.Filters {
			newFilter, ok, err := envoyExtension.Plugin.PatchFilter(filter)

			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter: %w", err))
				filters = append(filters, filter)
			}
			if ok {
				filters = append(filters, newFilter)
				patched = true
			}
		}
		filterChain.Filters = filters
	}

	return l, patched, resultErr
}

func (envoyExtension EnvoyExtension) patchConnectProxyListener(config xdscommon.ExtensionConfiguration, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	var resultErr error

	envoyID := ""
	if i := strings.IndexByte(l.Name, ':'); i != -1 {
		envoyID = l.Name[:i]
	}

	if config.IsUpstream() && envoyID == xdscommon.OutboundListenerName {
		return envoyExtension.patchTProxyListener(config, l)
	}

	// If the Envoy extension configuration is for an upstream service, the listener's
	// name must match the upstream service's EnvoyID or be the outbound listener.
	if config.IsUpstream() && envoyID != config.EnvoyID() {
		return l, false, nil
	}

	// If the Envoy extension configuration is for inbound resources, the
	// listener must be named xdscommon.PublicListenerName.
	if !config.IsUpstream() && envoyID != xdscommon.PublicListenerName {
		return l, false, nil
	}

	var patched bool

	for _, filterChain := range l.FilterChains {
		var filters []*envoy_listener_v3.Filter

		for _, filter := range filterChain.Filters {
			newFilter, ok, err := envoyExtension.Plugin.PatchFilter(filter)
			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter: %w", err))
				filters = append(filters, filter)
			}

			if ok {
				filters = append(filters, newFilter)
				patched = true
			}
		}
		filterChain.Filters = filters
	}

	return l, patched, resultErr
}

func (envoyExtension EnvoyExtension) patchTProxyListener(config xdscommon.ExtensionConfiguration, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	var resultErr error
	patched := false

	vip := config.Upstreams[config.ServiceName].VIP

	for _, filterChain := range l.FilterChains {
		var filters []*envoy_listener_v3.Filter

		match := filterChainTProxyMatch(vip, filterChain)
		if !match {
			continue
		}

		for _, filter := range filterChain.Filters {
			newFilter, ok, err := envoyExtension.Plugin.PatchFilter(filter)
			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter: %w", err))
				filters = append(filters, filter)
			}

			if ok {
				filters = append(filters, newFilter)
				patched = true
			}
		}
		filterChain.Filters = filters
	}

	return l, patched, resultErr
}

func filterChainTProxyMatch(vip string, filterChain *envoy_listener_v3.FilterChain) bool {
	for _, prefixRange := range filterChain.FilterChainMatch.PrefixRanges {
		// Since we always set the address prefix as the full VIP (rather than a prefix), we can just check if they are
		// equal to find the matching filter chain.
		if vip == prefixRange.AddressPrefix {
			return true
		}
	}

	return false
}

func FilterClusterNames(filter *envoy_listener_v3.Filter) map[string]struct{} {
	clusterNames := make(map[string]struct{})
	if filter == nil {
		return clusterNames
	}

	if config := envoy_resource_v3.GetHTTPConnectionManager(filter); config != nil {
		// If it's using RDS, the cluster names will be in the route, rather than in the http filter's route config, so
		// we don't return any cluster names in this case. They can be gathered from the route.
		if config.GetRds() != nil {
			return clusterNames
		}

		cfg := config.GetRouteConfig()

		clusterNames = RouteClusterNames(cfg)
	}

	if config := GetTCPProxy(filter); config != nil {
		clusterNames[config.GetCluster()] = struct{}{}
	}

	return clusterNames
}

func RouteClusterNames(route *envoy_route_v3.RouteConfiguration) map[string]struct{} {
	if route == nil {
		return nil
	}

	clusterNames := make(map[string]struct{})

	for _, virtualHost := range route.VirtualHosts {
		for _, route := range virtualHost.Routes {
			r := route.GetRoute()
			if r == nil {
				continue
			}
			if c := r.GetCluster(); c != "" {
				clusterNames[r.GetCluster()] = struct{}{}
			}

			if wc := r.GetWeightedClusters(); wc != nil {
				for _, c := range wc.GetClusters() {
					if c.Name != "" {
						clusterNames[c.Name] = struct{}{}
					}
				}
			}
		}
	}
	return clusterNames
}

func GetTCPProxy(filter *envoy_listener_v3.Filter) *envoy_tcp_proxy_v3.TcpProxy {
	if typedConfig := filter.GetTypedConfig(); typedConfig != nil {
		config := &envoy_tcp_proxy_v3.TcpProxy{}
		if err := anypb.UnmarshalTo(typedConfig, config, proto.UnmarshalOptions{}); err == nil {
			return config
		}
	}

	return nil
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
