package xdscommon

import (
	"github.com/golang/protobuf/proto"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

const (
	// Resource types in xDS v3. These are copied from
	// envoyproxy/go-control-plane/pkg/resource/v3/resource.go since we don't need any of
	// the rest of that package.
	apiTypePrefix = "type.googleapis.com/"

	// EndpointType is the TypeURL for Endpoint discovery responses.
	EndpointType = apiTypePrefix + "envoy.config.endpoint.v3.ClusterLoadAssignment"

	// ClusterType is the TypeURL for Cluster discovery responses.
	ClusterType = apiTypePrefix + "envoy.config.cluster.v3.Cluster"

	// RouteType is the TypeURL for Route discovery responses.
	RouteType = apiTypePrefix + "envoy.config.route.v3.RouteConfiguration"

	// ListenerType is the TypeURL for Listener discovery responses.
	ListenerType = apiTypePrefix + "envoy.config.listener.v3.Listener"
)

type IndexedResources struct {
	// Index is a map of typeURL => resourceName => resource
	Index map[string]map[string]proto.Message

	// ChildIndex is a map of typeURL => parentResourceName => list of
	// childResourceNames. This only applies if the child and parent do not
	// share a name.
	ChildIndex map[string]map[string][]string
}

func EmptyIndexedResources() *IndexedResources {
	return &IndexedResources{
		Index: map[string]map[string]proto.Message{
			ListenerType: make(map[string]proto.Message),
			RouteType:    make(map[string]proto.Message),
			ClusterType:  make(map[string]proto.Message),
			EndpointType: make(map[string]proto.Message),
		},
		ChildIndex: map[string]map[string][]string{
			ListenerType: make(map[string][]string),
			ClusterType:  make(map[string][]string),
		},
	}
}

type ServiceConfig struct {
	// Kind identifies the final proxy kind that will make the request to the
	// destination service.
	Kind api.ServiceKind
	Meta map[string]string
}

// PluginConfiguration is passed into Envoy plugins. It should depend on the
// API client rather than the structs package because the API client is meant
// to be public.
type PluginConfiguration struct {
	// ServiceConfigs is a mapping from service names to the data Envoy plugins
	// need to override the default Envoy configurations.
	ServiceConfigs map[api.CompoundServiceName]ServiceConfig

	// SNIToServiceName is a mapping from SNIs to service names. This allows
	// Envoy plugins to easily convert from an SNI Envoy resource name to the
	// associated service's CompoundServiceName
	SNIToServiceName map[string]api.CompoundServiceName

	// EnvoyIDToServiceName is a mapping from EnvoyIDs to service names. This allows
	// Envoy plugins to easily convert from an EnvoyID Envoy resource name to the
	// associated service's CompoundServiceName
	EnvoyIDToServiceName map[string]api.CompoundServiceName

	// Kind is mode the local Envoy proxy is running in. For now, only
	// terminating gateways are supported.
	Kind api.ServiceKind
}

// MakePluginConfiguration generates the configuration that will be sent to
// Envoy plugins.
func MakePluginConfiguration(cfgSnap *proxycfg.ConfigSnapshot) PluginConfiguration {
	serviceConfigs := make(map[api.CompoundServiceName]ServiceConfig)
	sniMappings := make(map[string]api.CompoundServiceName)
	envoyIDMappings := make(map[string]api.CompoundServiceName)

	trustDomain := ""
	if cfgSnap.Roots != nil {
		trustDomain = cfgSnap.Roots.TrustDomain
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindTerminatingGateway:
		for svc, c := range cfgSnap.TerminatingGateway.ServiceConfigs {
			compoundServiceName := serviceNameToCompoundServiceName(svc)
			serviceConfigs[compoundServiceName] = ServiceConfig{
				Meta: c.Meta,
				Kind: api.ServiceKindTerminatingGateway,
			}

			sni := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, trustDomain)
			sniMappings[sni] = compoundServiceName

			envoyID := proxycfg.NewUpstreamIDFromServiceName(svc)
			envoyIDMappings[envoyID.EnvoyID()] = compoundServiceName

			resolver, hasResolver := cfgSnap.TerminatingGateway.ServiceResolvers[svc]
			if hasResolver {
				for subsetName := range resolver.Subsets {
					sni := connect.ServiceSNI(svc.Name, subsetName, svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, trustDomain)
					sniMappings[sni] = compoundServiceName
				}
			}
		}
	}

	return PluginConfiguration{
		ServiceConfigs:       serviceConfigs,
		SNIToServiceName:     sniMappings,
		EnvoyIDToServiceName: envoyIDMappings,
		Kind:                 api.ServiceKind(cfgSnap.Kind),
	}
}

func serviceNameToCompoundServiceName(svc structs.ServiceName) api.CompoundServiceName {
	return api.CompoundServiceName{
		Name:      svc.Name,
		Partition: svc.PartitionOrDefault(),
		Namespace: svc.NamespaceOrDefault(),
	}
}
