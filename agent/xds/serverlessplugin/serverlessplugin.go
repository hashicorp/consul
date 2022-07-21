package serverlessplugin

import (
	"fmt"
	"strings"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/golang/protobuf/proto"
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

	for _, indexType := range []string{
		xdscommon.ClusterType,
		xdscommon.ListenerType,
		xdscommon.RouteType,
	} {
		for nameOrSNI, msg := range resources.Index[indexType] {
			switch resource := msg.(type) {
			case *envoy_cluster_v3.Cluster:
				patcher := getPatcherBySNI(config, nameOrSNI)
				if patcher == nil {
					continue
				}

				newCluster, patched, err := patcher.PatchCluster(resource)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching cluster: %w", err))
					continue
				}
				if patched {
					resources.Index[xdscommon.ClusterType][nameOrSNI] = newCluster
				}

			case *envoy_listener_v3.Listener:
				newListener, patched, err := patchListener(config, resource)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener: %w", err))
					continue
				}
				if patched {
					resources.Index[xdscommon.ListenerType][nameOrSNI] = newListener
				}

			case *envoy_route_v3.RouteConfiguration:
				patcher := getPatcherBySNI(config, nameOrSNI)
				if patcher == nil {
					continue
				}

				newRoute, patched, err := patcher.PatchRoute(resource)
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

func patchListener(config xdscommon.PluginConfiguration, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	switch config.Kind {
	case api.ServiceKindTerminatingGateway:
		return patchTerminatingGatewayListener(config, l)
	case api.ServiceKindConnectProxy:
		return patchConnectProxyListener(config, l)
	}
	return l, false, nil
}

func patchTerminatingGatewayListener(config xdscommon.PluginConfiguration, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	var resultErr error
	patched := false
	for _, filterChain := range l.FilterChains {
		sni := getSNI(filterChain)

		if sni == "" {
			continue
		}

		patcher := getPatcherBySNI(config, sni)

		if patcher == nil {
			continue
		}

		var filters []*envoy_listener_v3.Filter

		for _, filter := range filterChain.Filters {
			newFilter, ok, err := patcher.PatchFilter(filter)

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

func patchConnectProxyListener(config xdscommon.PluginConfiguration, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	var resultErr error

	envoyID := ""
	if i := strings.IndexByte(l.Name, ':'); i != -1 {
		envoyID = l.Name[:i]
	}

	patcher := getPatcherByEnvoyID(config, envoyID)
	if patcher == nil {
		return l, false, nil
	}

	var patched bool

	for _, filterChain := range l.FilterChains {
		var filters []*envoy_listener_v3.Filter

		for _, filter := range filterChain.Filters {
			newFilter, ok, err := patcher.PatchFilter(filter)
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
