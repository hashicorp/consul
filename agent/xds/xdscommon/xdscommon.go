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
	Kind            api.ServiceKind
	EnvoyExtensions []api.EnvoyExtension
}

// ExtensionConfiguration is the configuration for an extension attached to a service on the local proxy. Currently, it
// is only created for the local proxy's upstream service if the upstream service has an extension configured. In the
// future it will also include information about the service local to the local proxy as well. It should depend on the
// API client rather than the structs package because the API client is meant to be public.
type ExtensionConfiguration struct {
	// EnvoyExtension is the extension that will patch Envoy resources.
	EnvoyExtension api.EnvoyExtension

	// ServiceName is the name of the service the EnvoyExtension is being applied to. It could be the local service or
	// an upstream of the local service.
	ServiceName api.CompoundServiceName

	// Upstreams will only be configured on the ExtensionConfiguration if the EnvoyExtension is being applied to an
	// upstream. If there are no Upstreams, then EnvoyExtension is being applied to the local service's resources.
	Upstreams map[api.CompoundServiceName]UpstreamData

	// Kind is mode the local Envoy proxy is running in. For now, only connect proxy and
	// terminating gateways are supported.
	Kind api.ServiceKind
}

// UpstreamData has the SNI, EnvoyID, and OutgoingProxyKind of the upstream services for the local proxy and this data
// is used to choose which Envoy resources to patch.
type UpstreamData struct {
	// SNI is the SNI header used to reach an upstream service.
	SNI map[string]struct{}
	// EnvoyID is the envoy ID of an upstream service, structured <service> or <partition>/<ns>/<service> when using a
	// non-default namespace or partition.
	EnvoyID string
	// OutgoingProxyKind is the type of proxy of the upstream service. However, if the upstream is "typical" this will
	// be set to "connect-proxy" instead.
	OutgoingProxyKind api.ServiceKind
}

func (ec ExtensionConfiguration) IsUpstream() bool {
	_, ok := ec.Upstreams[ec.ServiceName]
	return ok
}

func (ec ExtensionConfiguration) MatchesUpstreamServiceSNI(sni string) bool {
	u := ec.Upstreams[ec.ServiceName]
	_, match := u.SNI[sni]
	return match
}

func (ec ExtensionConfiguration) EnvoyID() string {
	u := ec.Upstreams[ec.ServiceName]
	return u.EnvoyID
}

func (ec ExtensionConfiguration) OutgoingProxyKind() api.ServiceKind {
	u := ec.Upstreams[ec.ServiceName]
	return u.OutgoingProxyKind
}

func GetExtensionConfigurations(cfgSnap *proxycfg.ConfigSnapshot) map[api.CompoundServiceName][]ExtensionConfiguration {
	extensionsMap := make(map[api.CompoundServiceName][]api.EnvoyExtension)
	upstreamMap := make(map[api.CompoundServiceName]UpstreamData)
	var kind api.ServiceKind
	extensionConfigurationsMap := make(map[api.CompoundServiceName][]ExtensionConfiguration)

	trustDomain := ""
	if cfgSnap.Roots != nil {
		trustDomain = cfgSnap.Roots.TrustDomain
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		kind = api.ServiceKindConnectProxy
		outgoingKindByService := make(map[api.CompoundServiceName]api.ServiceKind)
		for uid, upstreamData := range cfgSnap.ConnectProxy.WatchedUpstreamEndpoints {
			sn := upstreamIDToCompoundServiceName(uid)

			for _, serviceNodes := range upstreamData {
				for _, serviceNode := range serviceNodes {
					if serviceNode.Service == nil {
						continue
					}
					// Store the upstream's kind, and for ServiceKindTypical we don't do anything because we'll default
					// any unset upstreams to ServiceKindConnectProxy below.
					switch serviceNode.Service.Kind {
					case structs.ServiceKindTypical:
					default:
						outgoingKindByService[sn] = api.ServiceKind(serviceNode.Service.Kind)
					}
					// We only need the kind from one instance, so break once we find it.
					break
				}
			}
		}

		// TODO(peering): consider PeerUpstreamEndpoints in addition to DiscoveryChain
		// These are the discovery chains for upstreams which have the Envoy Extensions applied to the local service.
		for uid, dc := range cfgSnap.ConnectProxy.DiscoveryChain {
			compoundServiceName := upstreamIDToCompoundServiceName(uid)
			extensionsMap[compoundServiceName] = convertEnvoyExtensions(dc.EnvoyExtensions)

			meta := uid.EnterpriseMeta
			sni := connect.ServiceSNI(uid.Name, "", meta.NamespaceOrDefault(), meta.PartitionOrDefault(), cfgSnap.Datacenter, trustDomain)
			outgoingKind, ok := outgoingKindByService[compoundServiceName]
			if !ok {
				outgoingKind = api.ServiceKindConnectProxy
			}

			upstreamMap[compoundServiceName] = UpstreamData{
				SNI:               map[string]struct{}{sni: {}},
				EnvoyID:           uid.EnvoyID(),
				OutgoingProxyKind: outgoingKind,
			}
		}
		// Adds extensions configured for the local service to the ExtensionConfiguration. This only applies to
		// connect-proxies because extensions are either global or tied to a specific service, so the terminating
		// gateway's Envoy resources for the local service (i.e not to upstreams) would never need to be modified.
		localSvc := api.CompoundServiceName{
			Name:      cfgSnap.Proxy.DestinationServiceName,
			Namespace: cfgSnap.ProxyID.NamespaceOrDefault(),
			Partition: cfgSnap.ProxyID.PartitionOrEmpty(),
		}
		extensionConfigurationsMap[localSvc] = []ExtensionConfiguration{}
		cfgSnapExts := convertEnvoyExtensions(cfgSnap.Proxy.EnvoyExtensions)
		for _, ext := range cfgSnapExts {
			extCfg := ExtensionConfiguration{
				EnvoyExtension: ext,
				ServiceName:    localSvc,
				// Upstreams is nil to signify this extension is not being applied to an upstream service, but rather to the local service.
				Upstreams: nil,
				Kind:      kind,
			}
			extensionConfigurationsMap[localSvc] = append(extensionConfigurationsMap[localSvc], extCfg)
		}
	case structs.ServiceKindTerminatingGateway:
		kind = api.ServiceKindTerminatingGateway
		for svc, c := range cfgSnap.TerminatingGateway.ServiceConfigs {
			compoundServiceName := serviceNameToCompoundServiceName(svc)
			extensionsMap[compoundServiceName] = convertEnvoyExtensions(c.EnvoyExtensions)

			sni := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, trustDomain)
			envoyID := proxycfg.NewUpstreamIDFromServiceName(svc)

			snis := map[string]struct{}{sni: {}}

			resolver, hasResolver := cfgSnap.TerminatingGateway.ServiceResolvers[svc]
			if hasResolver {
				for subsetName := range resolver.Subsets {
					sni := connect.ServiceSNI(svc.Name, subsetName, svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, trustDomain)
					snis[sni] = struct{}{}
				}
			}

			upstreamMap[compoundServiceName] = UpstreamData{
				SNI:               snis,
				EnvoyID:           envoyID.EnvoyID(),
				OutgoingProxyKind: api.ServiceKindTerminatingGateway,
			}

		}
	}

	for svc, exts := range extensionsMap {
		extensionConfigurationsMap[svc] = []ExtensionConfiguration{}
		for _, ext := range exts {
			extCfg := ExtensionConfiguration{
				EnvoyExtension: ext,
				Kind:           kind,
				ServiceName:    svc,
				Upstreams:      upstreamMap,
			}
			extensionConfigurationsMap[svc] = append(extensionConfigurationsMap[svc], extCfg)
		}
	}

	return extensionConfigurationsMap
}

func serviceNameToCompoundServiceName(svc structs.ServiceName) api.CompoundServiceName {
	return api.CompoundServiceName{
		Name:      svc.Name,
		Partition: svc.PartitionOrDefault(),
		Namespace: svc.NamespaceOrDefault(),
	}
}

func upstreamIDToCompoundServiceName(uid proxycfg.UpstreamID) api.CompoundServiceName {
	return api.CompoundServiceName{
		Name:      uid.Name,
		Partition: uid.PartitionOrDefault(),
		Namespace: uid.NamespaceOrDefault(),
	}
}

func convertEnvoyExtensions(structExtensions structs.EnvoyExtensions) []api.EnvoyExtension {
	return structExtensions.ToAPI()
}
