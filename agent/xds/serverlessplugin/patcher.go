package serverlessplugin

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

// patcher is the interface that each serverless integration must implement. It
// is responsible for modifying the xDS structures based on only the state of
// the patcher.
type patcher interface {
	// CanPatch determines if the patcher can mutate resources for the given api.ServiceKind
	CanPatch(api.ServiceKind) bool

	// patchRoute patches a route to include the custom Envoy configuration
	// PatchCluster patches a cluster to include the custom Envoy configuration
	// required to integrate with the serverless integration.
	PatchRoute(*envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error)

	// PatchCluster patches a cluster to include the custom Envoy configuration
	// required to integrate with the serverless integration.
	PatchCluster(*envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error)

	// PatchFilter patches an Envoy filter to include the custom Envoy
	// configuration required to integrate with the serverless integration.
	PatchFilter(*envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error)

	// PatchListener patches an Envoy filter to include the custom Envoy
	// configuration required to integrate with the serverless integration.
	PatchListener(*envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error)
}

type patchers map[api.CompoundServiceName]patcher

// getPatchersBySNI gets the patcher for the associated SNI.
func getPatchersBySNI(config xdscommon.PluginConfiguration, sni string) []patcher {
	if sni == "" {
		return nil
	}
	serviceName, ok := config.SNIToServiceName[sni]
	if !ok {
		return nil
	}

	serviceConfig, ok := config.ServiceConfigs[serviceName]
	if !ok {
		return nil
	}

	return makePatchers(config.Kind, serviceConfig)
}

// getPatchersByEnvoyID gets the patcher for the associated envoy id.
func getPatchersByEnvoyID(config xdscommon.PluginConfiguration, envoyID string) []patcher {
	if envoyID == "" {
		return nil
	}
	serviceName, ok := config.EnvoyIDToServiceName[envoyID]
	if !ok {
		return nil
	}

	serviceConfig, ok := config.ServiceConfigs[serviceName]
	if !ok {
		return nil
	}

	// TODO can we just use serviceConfig.Kind instead?
	return makePatchers(config.Kind, serviceConfig)
}

func makePatchers(kind api.ServiceKind, serviceConfig xdscommon.ServiceConfig) []patcher {
	var patchers []patcher
	// TODO iterating over a map is random order. Should we be worried about that now?
	for _, constructor := range patchConstructors {
		patcher, ok := constructor(serviceConfig)
		if ok && patcher.CanPatch(kind) {
			patchers = append(patchers, patcher)
		}
	}
	return patchers
}

// patchConstructor is used to construct patchers based on
// xdscommon.ServiceConfig. This function contains all of the logic around
// turning Meta data into the patcher.
type patchConstructor func(xdscommon.ServiceConfig) (patcher, bool)

// patchConstructors contains all patchers that getPatchers tries to create.
var patchConstructors = map[string]patchConstructor{
	"v1alpha1/aws-lambda":              makeLambdaPatcher,
	"v1alpha1/connection-load-balance": makeLoadBalancePatcher,
}
