package proxycfg

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/consul/agent/connect"

	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul/agent/structs"
)

// TODO(ingress): Can we think of a better for this bag of data?
// A shared data structure that contains information about discovered upstreams
type ConfigSnapshotUpstreams struct {
	Leaf *structs.IssuedCert
	// DiscoveryChain is a map of upstream.Identifier() ->
	// CompiledDiscoveryChain's, and is used to determine what services could be
	// targeted by this upstream. We then instantiate watches for those targets.
	DiscoveryChain map[string]*structs.CompiledDiscoveryChain

	// WatchedDiscoveryChains is a map of upstream.Identifier() -> CancelFunc's
	// in order to cancel any watches when the proxy's configuration is
	// changed. Ingress gateways and transparent proxies need this because
	// discovery chain watches are added and removed through the lifecycle
	// of a single proxycfg state instance.
	WatchedDiscoveryChains map[string]context.CancelFunc

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

	// UpstreamConfig is a map to an upstream's configuration.
	UpstreamConfig map[string]*structs.Upstream

	// PassthroughEndpoints is a map of: ServiceName -> ServicePassthroughAddrs.
	PassthroughUpstreams map[string]ServicePassthroughAddrs
}

// ServicePassthroughAddrs contains the LAN addrs
type ServicePassthroughAddrs struct {
	// SNI is the Service SNI of the upstream.
	SNI string

	// SpiffeID is the SPIFFE ID to use for upstream SAN validation.
	SpiffeID connect.SpiffeIDService

	// Addrs is a set of the best LAN addresses for the instances of the upstream.
	Addrs map[string]struct{}
}

type configSnapshotConnectProxy struct {
	ConfigSnapshotUpstreams

	WatchedServiceChecks   map[structs.ServiceID][]structs.CheckType // TODO: missing garbage collection
	PreparedQueryEndpoints map[string]structs.CheckServiceNodes      // DEPRECATED:see:WatchedUpstreamEndpoints

	// NOTE: Intentions stores a list of lists as returned by the Intentions
	// Match RPC. So far we only use the first list as the list of matching
	// intentions.
	Intentions    structs.Intentions
	IntentionsSet bool

	MeshConfig    *structs.MeshConfigEntry
	MeshConfigSet bool
}

func (c *configSnapshotConnectProxy) IsEmpty() bool {
	if c == nil {
		return true
	}
	return c.Leaf == nil &&
		!c.IntentionsSet &&
		len(c.DiscoveryChain) == 0 &&
		len(c.WatchedDiscoveryChains) == 0 &&
		len(c.WatchedUpstreams) == 0 &&
		len(c.WatchedUpstreamEndpoints) == 0 &&
		len(c.WatchedGateways) == 0 &&
		len(c.WatchedGatewayEndpoints) == 0 &&
		len(c.WatchedServiceChecks) == 0 &&
		len(c.PreparedQueryEndpoints) == 0 &&
		len(c.UpstreamConfig) == 0 &&
		len(c.PassthroughUpstreams) == 0 &&
		!c.MeshConfigSet
}

type configSnapshotTerminatingGateway struct {
	// WatchedServices is a map of service name to a cancel function. This cancel
	// function is tied to the watch of linked service instances for the given
	// id. If the linked services watch would indicate the removal of
	// a service altogether we then cancel watching that service for its endpoints.
	WatchedServices map[structs.ServiceName]context.CancelFunc

	// WatchedIntentions is a map of service name to a cancel function.
	// This cancel function is tied to the watch of intentions for linked services.
	// As with WatchedServices, intention watches will be cancelled when services
	// are no longer linked to the gateway.
	WatchedIntentions map[structs.ServiceName]context.CancelFunc

	// NOTE: Intentions stores a map of list of lists as returned by the Intentions
	// Match RPC. So far we only use the first list as the list of matching
	// intentions.
	//
	// A key being present implies that we have gotten at least one watch reply for the
	// service. This is logically the same as ConnectProxy.IntentionsSet==true
	Intentions map[structs.ServiceName]structs.Intentions

	// WatchedLeaves is a map of ServiceName to a cancel function.
	// This cancel function is tied to the watch of leaf certs for linked services.
	// As with WatchedServices, leaf watches will be cancelled when services
	// are no longer linked to the gateway.
	WatchedLeaves map[structs.ServiceName]context.CancelFunc

	// ServiceLeaves is a map of ServiceName to a leaf cert.
	// Terminating gateways will present different certificates depending
	// on the service that the caller is trying to reach.
	ServiceLeaves map[structs.ServiceName]*structs.IssuedCert

	// WatchedConfigs is a map of ServiceName to a cancel function. This cancel
	// function is tied to the watch of service configs for linked services. As
	// with WatchedServices, service config watches will be cancelled when
	// services are no longer linked to the gateway.
	WatchedConfigs map[structs.ServiceName]context.CancelFunc

	// ServiceConfigs is a map of service name to the resolved service config
	// for that service.
	ServiceConfigs map[structs.ServiceName]*structs.ServiceConfigResponse

	// WatchedResolvers is a map of ServiceName to a cancel function.
	// This cancel function is tied to the watch of resolvers for linked services.
	// As with WatchedServices, resolver watches will be cancelled when services
	// are no longer linked to the gateway.
	WatchedResolvers map[structs.ServiceName]context.CancelFunc

	// ServiceResolvers is a map of service name to an associated
	// service-resolver config entry for that service.
	ServiceResolvers    map[structs.ServiceName]*structs.ServiceResolverConfigEntry
	ServiceResolversSet map[structs.ServiceName]bool

	// ServiceGroups is a map of service name to the service instances of that
	// service in the local datacenter.
	ServiceGroups map[structs.ServiceName]structs.CheckServiceNodes

	// GatewayServices is a map of service name to the config entry association
	// between the gateway and a service. TLS configuration stored here is
	// used for TLS origination from the gateway to the linked service.
	GatewayServices map[structs.ServiceName]structs.GatewayService

	// HostnameServices is a map of service name to service instances with a hostname as the address.
	// If hostnames are configured they must be provided to Envoy via CDS not EDS.
	HostnameServices map[structs.ServiceName]structs.CheckServiceNodes
}

// ValidServices returns the list of service keys that have enough data to be emitted.
func (c *configSnapshotTerminatingGateway) ValidServices() []structs.ServiceName {
	out := make([]structs.ServiceName, 0, len(c.ServiceGroups))
	for svc := range c.ServiceGroups {
		// It only counts if ALL of our watches have come back (with data or not).

		// Skip the service if we don't know if there is a resolver or not.
		if _, ok := c.ServiceResolversSet[svc]; !ok {
			continue
		}

		// Skip the service if we don't have a cert to present for mTLS.
		if cert, ok := c.ServiceLeaves[svc]; !ok || cert == nil {
			continue
		}

		// Skip the service if we haven't gotten our intentions yet.
		if _, intentionsSet := c.Intentions[svc]; !intentionsSet {
			continue
		}

		// Skip the service if we haven't gotten our service config yet to know
		// the protocol.
		if _, ok := c.ServiceConfigs[svc]; !ok {
			continue
		}

		out = append(out, svc)
	}
	return out
}

func (c *configSnapshotTerminatingGateway) IsEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.ServiceLeaves) == 0 &&
		len(c.WatchedLeaves) == 0 &&
		len(c.WatchedIntentions) == 0 &&
		len(c.Intentions) == 0 &&
		len(c.ServiceGroups) == 0 &&
		len(c.WatchedServices) == 0 &&
		len(c.ServiceResolvers) == 0 &&
		len(c.ServiceResolversSet) == 0 &&
		len(c.WatchedResolvers) == 0 &&
		len(c.ServiceConfigs) == 0 &&
		len(c.WatchedConfigs) == 0 &&
		len(c.GatewayServices) == 0 &&
		len(c.HostnameServices) == 0
}

type configSnapshotMeshGateway struct {
	// WatchedServices is a map of service name to a cancel function. This cancel
	// function is tied to the watch of connect enabled services for the given
	// id. If the main datacenter services watch would indicate the removal of
	// a service altogether we then cancel watching that service for its
	// connect endpoints.
	WatchedServices map[structs.ServiceName]context.CancelFunc

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

	// ServiceGroups is a map of service name to the service instances of that
	// service in the local datacenter.
	ServiceGroups map[structs.ServiceName]structs.CheckServiceNodes

	// ServiceResolvers is a map of service name to an associated
	// service-resolver config entry for that service.
	ServiceResolvers map[structs.ServiceName]*structs.ServiceResolverConfigEntry

	// GatewayGroups is a map of datacenter names to services of kind
	// mesh-gateway in that datacenter.
	GatewayGroups map[string]structs.CheckServiceNodes

	// FedStateGateways is a map of datacenter names to mesh gateways in that
	// datacenter.
	FedStateGateways map[string]structs.CheckServiceNodes

	// ConsulServers is the list of consul servers in this datacenter.
	ConsulServers structs.CheckServiceNodes

	// HostnameDatacenters is a map of datacenters to mesh gateway instances with a hostname as the address.
	// If hostnames are configured they must be provided to Envoy via CDS not EDS.
	HostnameDatacenters map[string]structs.CheckServiceNodes
}

func (c *configSnapshotMeshGateway) Datacenters() []string {
	sz1, sz2 := len(c.GatewayGroups), len(c.FedStateGateways)

	sz := sz1
	if sz2 > sz1 {
		sz = sz2
	}

	dcs := make([]string, 0, sz)
	for dc := range c.GatewayGroups {
		dcs = append(dcs, dc)
	}
	for dc := range c.FedStateGateways {
		if _, ok := c.GatewayGroups[dc]; !ok {
			dcs = append(dcs, dc)
		}
	}

	// Always sort the results to ensure we generate deterministic things over
	// xDS, such as mesh-gateway listener filter chains.
	sort.Strings(dcs)
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
		len(c.ConsulServers) == 0 &&
		len(c.HostnameDatacenters) == 0
}

type configSnapshotIngressGateway struct {
	ConfigSnapshotUpstreams

	// TLSEnabled is whether this gateway's listeners should have TLS configured.
	TLSEnabled bool

	// TODO(banks): rename to "ConfigLoaded" or something or just remove it since
	// only usages seem to be places that really should be checking TLSEnabled ==
	// true anyway?
	TLSSet bool

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

	// Listeners is the original listener config from the ingress-gateway config
	// entry to save us trying to pass fields through Upstreams
	Listeners map[IngressListenerKey]structs.IngressListener
}

func (c *configSnapshotIngressGateway) IsEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.Upstreams) == 0 &&
		len(c.DiscoveryChain) == 0 &&
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
	Kind                  structs.ServiceKind
	Service               string
	ProxyID               structs.ServiceID
	Address               string
	Port                  int
	ServiceMeta           map[string]string
	TaggedAddresses       map[string]structs.ServiceAddress
	Proxy                 structs.ConnectProxyConfig
	Datacenter            string
	IntentionDefaultAllow bool

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
}

// Valid returns whether or not the snapshot has all required fields filled yet.
func (s *ConfigSnapshot) Valid() bool {
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		if s.Proxy.Mode == structs.ProxyModeTransparent && !s.ConnectProxy.MeshConfigSet {
			return false
		}
		return s.Roots != nil &&
			s.ConnectProxy.Leaf != nil &&
			s.ConnectProxy.IntentionsSet

	case structs.ServiceKindTerminatingGateway:
		return s.Roots != nil

	case structs.ServiceKindMeshGateway:
		if s.ServiceMeta[structs.MetaWANFederationKey] == "1" {
			if len(s.MeshGateway.ConsulServers) == 0 {
				return false
			}
		}
		return s.Roots != nil &&
			(s.MeshGateway.WatchedServicesSet || len(s.MeshGateway.ServiceGroups) > 0)

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
		snap.TerminatingGateway.WatchedConfigs = nil
		snap.TerminatingGateway.WatchedResolvers = nil
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
