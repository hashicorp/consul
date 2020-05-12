package proxycfg

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/copystructure"
)

// TODO(ingress): Can we think of a better for this bag of data?
// A shared data structure that contains information about discovered upstreams
type ConfigSnapshotUpstreams struct {
	Leaf *structs.IssuedCert
	// DiscoveryChain is a map of upstream.Identifier() ->
	// CompiledDiscoveryChain's, and is used to determine what services could be
	// targeted by this upstream. We then instantiate watches for those targets.
	DiscoveryChain map[string]*structs.CompiledDiscoveryChain

	// WatchedUpstreams is a map of upstream.Identifier() -> (map of TargetID ->
	// CancelFunc's) in order to cancel any watches when the configuration is
	// changed.
	WatchedUpstreams map[string]map[string]context.CancelFunc

	// WatchedUpstreamEndpoints is a map of upstream.Identifier() -> (map of
	// TargetID -> CheckServiceNodes) and is used to determine the backing
	// endpoints of an upstream.
	WatchedUpstreamEndpoints map[string]map[string]structs.CheckServiceNodes

	// WatchedGateways is a map of upstream.Identifier() -> (map of
	// TargetID -> CancelFunc) in order to cancel watches for mesh gateways
	WatchedGateways map[string]map[string]context.CancelFunc

	// WatchedGatewayEndpoints is a map of upstream.Identifier() -> (map of
	// TargetID -> CheckServiceNodes) and is used to determine the backing
	// endpoints of a mesh gateway.
	WatchedGatewayEndpoints map[string]map[string]structs.CheckServiceNodes
}

type configSnapshotConnectProxy struct {
	ConfigSnapshotUpstreams

	WatchedServiceChecks   map[structs.ServiceID][]structs.CheckType // TODO: missing garbage collection
	PreparedQueryEndpoints map[string]structs.CheckServiceNodes      // DEPRECATED:see:WatchedUpstreamEndpoints
}

func (c *configSnapshotConnectProxy) IsEmpty() bool {
	if c == nil {
		return true
	}
	return c.Leaf == nil &&
		len(c.DiscoveryChain) == 0 &&
		len(c.WatchedUpstreams) == 0 &&
		len(c.WatchedUpstreamEndpoints) == 0 &&
		len(c.WatchedGateways) == 0 &&
		len(c.WatchedGatewayEndpoints) == 0 &&
		len(c.WatchedServiceChecks) == 0 &&
		len(c.PreparedQueryEndpoints) == 0
}

type configSnapshotTerminatingGateway struct {
	// WatchedServices is a map of service id to a cancel function. This cancel
	// function is tied to the watch of linked service instances for the given
	// id. If the linked services watch would indicate the removal of
	// a service altogether we then cancel watching that service for its endpoints.
	WatchedServices map[structs.ServiceID]context.CancelFunc

	// WatchedIntentions is a map of service id to a cancel function.
	// This cancel function is tied to the watch of intentions for linked services.
	// As with WatchedServices, intention watches will be cancelled when services
	// are no longer linked to the gateway.
	WatchedIntentions map[structs.ServiceID]context.CancelFunc

	// WatchedLeaves is a map of ServiceID to a cancel function.
	// This cancel function is tied to the watch of leaf certs for linked services.
	// As with WatchedServices, leaf watches will be cancelled when services
	// are no longer linked to the gateway.
	WatchedLeaves map[structs.ServiceID]context.CancelFunc

	// ServiceLeaves is a map of ServiceID to a leaf cert.
	// Terminating gateways will present different certificates depending
	// on the service that the caller is trying to reach.
	ServiceLeaves map[structs.ServiceID]*structs.IssuedCert

	// WatchedResolvers is a map of ServiceID to a cancel function.
	// This cancel function is tied to the watch of resolvers for linked services.
	// As with WatchedServices, resolver watches will be cancelled when services
	// are no longer linked to the gateway.
	WatchedResolvers map[structs.ServiceID]context.CancelFunc

	// ServiceResolvers is a map of service id to an associated
	// service-resolver config entry for that service.
	ServiceResolvers map[structs.ServiceID]*structs.ServiceResolverConfigEntry

	// ServiceGroups is a map of service id to the service instances of that
	// service in the local datacenter.
	ServiceGroups map[structs.ServiceID]structs.CheckServiceNodes

	// GatewayServices is a map of service id to the config entry association
	// between the gateway and a service. TLS configuration stored here is
	// used for TLS origination from the gateway to the linked service.
	GatewayServices map[structs.ServiceID]structs.GatewayService
}

func (c *configSnapshotTerminatingGateway) IsEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.ServiceLeaves) == 0 &&
		len(c.WatchedLeaves) == 0 &&
		len(c.WatchedIntentions) == 0 &&
		len(c.ServiceGroups) == 0 &&
		len(c.WatchedServices) == 0 &&
		len(c.ServiceResolvers) == 0 &&
		len(c.WatchedResolvers) == 0 &&
		len(c.GatewayServices) == 0
}

type configSnapshotMeshGateway struct {
	// WatchedServices is a map of service id to a cancel function. This cancel
	// function is tied to the watch of connect enabled services for the given
	// id. If the main datacenter services watch would indicate the removal of
	// a service altogether we then cancel watching that service for its
	// connect endpoints.
	WatchedServices map[structs.ServiceID]context.CancelFunc

	// WatchedServicesSet indicates that the watch on the datacenters services
	// has completed. Even when there are no connect services, this being set
	// (and the Connect roots being available) will be enough for the config
	// snapshot to be considered valid. In the case of Envoy, this allows it to
	// start its listeners even when no services would be proxied and allow its
	// health check to pass.
	WatchedServicesSet bool

	// WatchedDatacenters is a map of datacenter name to a cancel function.
	// This cancel function is tied to the watch of mesh-gateway services in
	// that datacenter.
	WatchedDatacenters map[string]context.CancelFunc

	// ServiceGroups is a map of service id to the service instances of that
	// service in the local datacenter.
	ServiceGroups map[structs.ServiceID]structs.CheckServiceNodes

	// ServiceResolvers is a map of service id to an associated
	// service-resolver config entry for that service.
	ServiceResolvers map[structs.ServiceID]*structs.ServiceResolverConfigEntry

	// GatewayGroups is a map of datacenter names to services of kind
	// mesh-gateway in that datacenter.
	GatewayGroups map[string]structs.CheckServiceNodes

	// FedStateGateways is a map of datacenter names to mesh gateways in that
	// datacenter.
	FedStateGateways map[string]structs.CheckServiceNodes

	// ConsulServers is the list of consul servers in this datacenter.
	ConsulServers structs.CheckServiceNodes
}

func (c *configSnapshotMeshGateway) Datacenters() []string {
	sz1, sz2 := len(c.GatewayGroups), len(c.FedStateGateways)

	sz := sz1
	if sz2 > sz1 {
		sz = sz2
	}

	dcs := make([]string, 0, sz)
	for dc, _ := range c.GatewayGroups {
		dcs = append(dcs, dc)
	}
	for dc, _ := range c.FedStateGateways {
		if _, ok := c.GatewayGroups[dc]; !ok {
			dcs = append(dcs, dc)
		}
	}
	return dcs
}

func (c *configSnapshotMeshGateway) IsEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.WatchedServices) == 0 &&
		!c.WatchedServicesSet &&
		len(c.WatchedDatacenters) == 0 &&
		len(c.ServiceGroups) == 0 &&
		len(c.ServiceResolvers) == 0 &&
		len(c.GatewayGroups) == 0 &&
		len(c.FedStateGateways) == 0 &&
		len(c.ConsulServers) == 0
}

type configSnapshotIngressGateway struct {
	ConfigSnapshotUpstreams

	// TLSEnabled is whether this gateway's listeners should have TLS configured.
	TLSEnabled bool
	TLSSet     bool

	// Hosts is the list of extra host entries to add to our leaf cert's DNS SANs.
	Hosts    []string
	HostsSet bool

	// LeafCertWatchCancel is a CancelFunc to use when refreshing this gateway's
	// leaf cert watch with different parameters.
	LeafCertWatchCancel context.CancelFunc

	// Upstreams is a list of upstreams this ingress gateway should serve traffic
	// to. This is constructed from the ingress-gateway config entry, and uses
	// the GatewayServices RPC to retrieve them.
	Upstreams map[IngressListenerKey]structs.Upstreams

	// WatchedDiscoveryChains is a map of upstream.Identifier() -> CancelFunc's
	// in order to cancel any watches when the ingress gateway configuration is
	// changed. Ingress gateways need this because discovery chain watches are
	// added and removed through the lifecycle of single proxycfg.state instance.
	WatchedDiscoveryChains map[string]context.CancelFunc
}

func (c *configSnapshotIngressGateway) IsEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.Upstreams) == 0 &&
		len(c.DiscoveryChain) == 0 &&
		len(c.WatchedDiscoveryChains) == 0 &&
		len(c.WatchedUpstreams) == 0 &&
		len(c.WatchedUpstreamEndpoints) == 0
}

type IngressListenerKey struct {
	Protocol string
	Port     int
}

func (k *IngressListenerKey) RouteName() string {
	return fmt.Sprintf("%d", k.Port)
}

// ConfigSnapshot captures all the resulting config needed for a proxy instance.
// It is meant to be point-in-time coherent and is used to deliver the current
// config state to observers who need it to be pushed in (e.g. XDS server).
type ConfigSnapshot struct {
	Kind            structs.ServiceKind
	Service         string
	ProxyID         structs.ServiceID
	Address         string
	Port            int
	ServiceMeta     map[string]string
	TaggedAddresses map[string]structs.ServiceAddress
	Proxy           structs.ConnectProxyConfig
	Datacenter      string

	ServerSNIFn ServerSNIFunc
	Roots       *structs.IndexedCARoots

	// connect-proxy specific
	ConnectProxy configSnapshotConnectProxy

	// terminating-gateway specific
	TerminatingGateway configSnapshotTerminatingGateway

	// mesh-gateway specific
	MeshGateway configSnapshotMeshGateway

	// ingress-gateway specific
	IngressGateway configSnapshotIngressGateway

	// Skip intentions for now as we don't push those down yet, just pre-warm them.
}

// Valid returns whether or not the snapshot has all required fields filled yet.
func (s *ConfigSnapshot) Valid() bool {
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		return s.Roots != nil && s.ConnectProxy.Leaf != nil
	case structs.ServiceKindTerminatingGateway:
		return s.Roots != nil
	case structs.ServiceKindMeshGateway:
		if s.ServiceMeta[structs.MetaWANFederationKey] == "1" {
			if len(s.MeshGateway.ConsulServers) == 0 {
				return false
			}
		}
		return s.Roots != nil && (s.MeshGateway.WatchedServicesSet || len(s.MeshGateway.ServiceGroups) > 0)
	case structs.ServiceKindIngressGateway:
		return s.Roots != nil &&
			s.IngressGateway.Leaf != nil &&
			s.IngressGateway.TLSSet &&
			s.IngressGateway.HostsSet
	default:
		return false
	}
}

// Clone makes a deep copy of the snapshot we can send to other goroutines
// without worrying that they will racily read or mutate shared maps etc.
func (s *ConfigSnapshot) Clone() (*ConfigSnapshot, error) {
	snapCopy, err := copystructure.Copy(s)
	if err != nil {
		return nil, err
	}

	snap := snapCopy.(*ConfigSnapshot)

	// nil these out as anything receiving one of these clones does not need them and should never "cancel" our watches
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		snap.ConnectProxy.WatchedUpstreams = nil
		snap.ConnectProxy.WatchedGateways = nil
	case structs.ServiceKindTerminatingGateway:
		snap.TerminatingGateway.WatchedServices = nil
		snap.TerminatingGateway.WatchedIntentions = nil
		snap.TerminatingGateway.WatchedLeaves = nil
	case structs.ServiceKindMeshGateway:
		snap.MeshGateway.WatchedDatacenters = nil
		snap.MeshGateway.WatchedServices = nil
	case structs.ServiceKindIngressGateway:
		snap.IngressGateway.WatchedUpstreams = nil
		snap.IngressGateway.WatchedGateways = nil
		snap.IngressGateway.WatchedDiscoveryChains = nil
		snap.IngressGateway.LeafCertWatchCancel = nil
	}

	return snap, nil
}

func (s *ConfigSnapshot) Leaf() *structs.IssuedCert {
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		return s.ConnectProxy.Leaf
	case structs.ServiceKindIngressGateway:
		return s.IngressGateway.Leaf
	default:
		return nil
	}
}
