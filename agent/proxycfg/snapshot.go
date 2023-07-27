// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfg

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

// TODO(ingress): Can we think of a better for this bag of data?
// A shared data structure that contains information about discovered upstreams
type ConfigSnapshotUpstreams struct {
	Leaf *structs.IssuedCert

	MeshConfig    *structs.MeshConfigEntry
	MeshConfigSet bool

	// DiscoveryChain is a map of UpstreamID -> CompiledDiscoveryChain's, and
	// is used to determine what services could be targeted by this upstream.
	// We then instantiate watches for those targets.
	DiscoveryChain map[UpstreamID]*structs.CompiledDiscoveryChain

	// WatchedDiscoveryChains is a map of UpstreamID -> CancelFunc's
	// in order to cancel any watches when the proxy's configuration is
	// changed. Ingress gateways and transparent proxies need this because
	// discovery chain watches are added and removed through the lifecycle
	// of a single proxycfg state instance.
	WatchedDiscoveryChains map[UpstreamID]context.CancelFunc

	// WatchedUpstreams is a map of UpstreamID -> (map of TargetID ->
	// CancelFunc's) in order to cancel any watches when the configuration is
	// changed.
	WatchedUpstreams map[UpstreamID]map[string]context.CancelFunc

	// WatchedUpstreamEndpoints is a map of UpstreamID -> (map of
	// TargetID -> CheckServiceNodes) and is used to determine the backing
	// endpoints of an upstream.
	WatchedUpstreamEndpoints map[UpstreamID]map[string]structs.CheckServiceNodes

	// UpstreamPeerTrustBundles is a map of (PeerName -> PeeringTrustBundle).
	// It is used to store trust bundles for upstream TLS transport sockets.
	UpstreamPeerTrustBundles watch.Map[PeerName, *pbpeering.PeeringTrustBundle]

	// WatchedGateways is a map of UpstreamID -> (map of GatewayKey.String() ->
	// CancelFunc) in order to cancel watches for mesh gateways
	WatchedGateways map[UpstreamID]map[string]context.CancelFunc

	// WatchedGatewayEndpoints is a map of UpstreamID -> (map of
	// GatewayKey.String() -> CheckServiceNodes) and is used to determine the
	// backing endpoints of a mesh gateway.
	WatchedGatewayEndpoints map[UpstreamID]map[string]structs.CheckServiceNodes

	// WatchedLocalGWEndpoints is used to store the backing endpoints of
	// a local mesh gateway. Currently, this is used by peered upstreams
	// configured with local mesh gateway mode so that they can watch for
	// gateway endpoints.
	//
	// Note that the string form of GatewayKey is used as the key so empty
	// fields can be normalized in OSS.
	//   GatewayKey.String() -> structs.CheckServiceNodes
	WatchedLocalGWEndpoints watch.Map[string, structs.CheckServiceNodes]

	// UpstreamConfig is a map to an upstream's configuration.
	UpstreamConfig map[UpstreamID]*structs.Upstream

	// PassthroughEndpoints is a map of: UpstreamID -> (map of TargetID ->
	// (set of IP addresses)). It contains the upstream endpoints that
	// can be dialed directly by a transparent proxy.
	PassthroughUpstreams map[UpstreamID]map[string]map[string]struct{}

	// PassthroughIndices is a map of: address -> indexedTarget.
	// It is used to track the modify index associated with a passthrough address.
	// Tracking this index helps break ties when a single address is shared by
	// more than one upstream due to a race.
	PassthroughIndices map[string]indexedTarget

	// IntentionUpstreams is a set of upstreams inferred from intentions.
	//
	// This list only applies to proxies registered in 'transparent' mode.
	IntentionUpstreams map[UpstreamID]struct{}

	// PeeredUpstreams is a set of all upstream targets in a local partition.
	//
	// This list only applies to proxies registered in 'transparent' mode.
	PeeredUpstreams map[UpstreamID]struct{}

	// PeerUpstreamEndpoints is a map of UpstreamID -> (set of IP addresses)
	// and used to determine the backing endpoints of an upstream in another
	// peer.
	PeerUpstreamEndpoints watch.Map[UpstreamID, structs.CheckServiceNodes]

	PeerUpstreamEndpointsUseHostnames map[UpstreamID]struct{}
}

// indexedTarget is used to associate the Raft modify index of a resource
// with the corresponding upstream target.
type indexedTarget struct {
	upstreamID UpstreamID
	targetID   string
	idx        uint64
}

type GatewayKey struct {
	Datacenter string
	Partition  string
}

func (k GatewayKey) String() string {
	resp := k.Datacenter
	if !acl.IsDefaultPartition(k.Partition) {
		resp = k.Partition + "." + resp
	}
	return resp
}

func (k GatewayKey) IsEmpty() bool {
	return k.Partition == "" && k.Datacenter == ""
}

func (k GatewayKey) Matches(dc, partition string) bool {
	return acl.EqualPartitions(k.Partition, partition) && k.Datacenter == dc
}

func gatewayKeyFromString(s string) GatewayKey {
	split := strings.SplitN(s, ".", 2)

	if len(split) == 1 {
		return GatewayKey{Datacenter: split[0], Partition: acl.DefaultPartitionName}
	}
	return GatewayKey{Partition: split[0], Datacenter: split[1]}
}

type configSnapshotConnectProxy struct {
	ConfigSnapshotUpstreams

	InboundPeerTrustBundlesSet bool
	InboundPeerTrustBundles    []*pbpeering.PeeringTrustBundle

	WatchedServiceChecks   map[structs.ServiceID][]structs.CheckType // TODO: missing garbage collection
	PreparedQueryEndpoints map[UpstreamID]structs.CheckServiceNodes  // DEPRECATED:see:WatchedUpstreamEndpoints

	// NOTE: Intentions stores a list of lists as returned by the Intentions
	// Match RPC. So far we only use the first list as the list of matching
	// intentions.
	Intentions    structs.SimplifiedIntentions
	IntentionsSet bool

	DestinationsUpstream watch.Map[UpstreamID, *structs.ServiceConfigEntry]
	DestinationGateways  watch.Map[UpstreamID, structs.CheckServiceNodes]
}

// isEmpty is a test helper
func (c *configSnapshotConnectProxy) isEmpty() bool {
	if c == nil {
		return true
	}
	return c.Leaf == nil &&
		!c.IntentionsSet &&
		len(c.DiscoveryChain) == 0 &&
		len(c.WatchedDiscoveryChains) == 0 &&
		len(c.WatchedUpstreams) == 0 &&
		len(c.WatchedUpstreamEndpoints) == 0 &&
		c.UpstreamPeerTrustBundles.Len() == 0 &&
		len(c.WatchedGateways) == 0 &&
		len(c.WatchedGatewayEndpoints) == 0 &&
		len(c.WatchedServiceChecks) == 0 &&
		len(c.PreparedQueryEndpoints) == 0 &&
		len(c.UpstreamConfig) == 0 &&
		len(c.PassthroughUpstreams) == 0 &&
		len(c.IntentionUpstreams) == 0 &&
		c.DestinationGateways.Len() == 0 &&
		c.DestinationsUpstream.Len() == 0 &&
		len(c.PeeredUpstreams) == 0 &&
		!c.InboundPeerTrustBundlesSet &&
		!c.MeshConfigSet &&
		c.PeerUpstreamEndpoints.Len() == 0 &&
		len(c.PeerUpstreamEndpointsUseHostnames) == 0
}

func (c *configSnapshotConnectProxy) IsImplicitUpstream(uid UpstreamID) bool {
	_, intentionImplicit := c.IntentionUpstreams[uid]
	_, peeringImplicit := c.PeeredUpstreams[uid]
	return intentionImplicit || peeringImplicit
}

func (c *configSnapshotConnectProxy) GetUpstream(uid UpstreamID, entMeta *acl.EnterpriseMeta) (*structs.Upstream, bool) {
	upstream, found := c.UpstreamConfig[uid]
	// We should fallback to the wildcard defaults generated from service-defaults + proxy-defaults
	// whenever we don't find the upstream config.
	if !found {
		wildcardUID := NewWildcardUID(entMeta)
		upstream = c.UpstreamConfig[wildcardUID]
	}

	explicit := upstream != nil && upstream.HasLocalPortOrSocket()
	implicit := c.IsImplicitUpstream(uid)
	return upstream, !implicit && !explicit
}

type configSnapshotTerminatingGateway struct {
	MeshConfig    *structs.MeshConfigEntry
	MeshConfigSet bool

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
	Intentions map[structs.ServiceName]structs.SimplifiedIntentions

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
	// This map does not include GatewayServices that represent Endpoints to external
	// destinations.
	GatewayServices map[structs.ServiceName]structs.GatewayService

	// DestinationServices is a map of service name to GatewayServices that represent
	// a destination to an external destination of the service mesh.
	DestinationServices map[structs.ServiceName]structs.GatewayService

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

// ValidDestinations returns the list of service keys (that represent exclusively endpoints) that have enough data to be emitted.
func (c *configSnapshotTerminatingGateway) ValidDestinations() []structs.ServiceName {
	out := make([]structs.ServiceName, 0, len(c.DestinationServices))
	for svc := range c.DestinationServices {
		// It only counts if ALL of our watches have come back (with data or not).

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
		if conf, ok := c.ServiceConfigs[svc]; !ok || len(conf.Destination.Addresses) == 0 {
			continue
		}

		out = append(out, svc)
	}
	return out
}

// isEmpty is a test helper
func (c *configSnapshotTerminatingGateway) isEmpty() bool {
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
		len(c.DestinationServices) == 0 &&
		len(c.HostnameServices) == 0 &&
		!c.MeshConfigSet
}

type PeerServersValue struct {
	Addresses []structs.ServiceAddress
	Index     uint64
	UseCDS    bool
}

type PeeringServiceValue struct {
	Nodes  structs.CheckServiceNodes
	UseCDS bool
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

	// WatchedGateways is a map of GatewayKeys to a cancel function.
	// This cancel function is tied to the watch of mesh-gateway services in
	// that datacenter/partition.
	WatchedGateways map[string]context.CancelFunc

	// ServiceGroups is a map of service name to the service instances of that
	// service in the local datacenter.
	ServiceGroups map[structs.ServiceName]structs.CheckServiceNodes

	// PeeringServices is a map of peer name -> (map of
	// service name -> CheckServiceNodes) and is used to determine the backing
	// endpoints of a service on a peer.
	PeeringServices map[string]map[structs.ServiceName]PeeringServiceValue

	// WatchedPeeringServices is a map of peer name -> (map of service name ->
	// cancel function) and is used to track watches on services within a peer.
	WatchedPeeringServices map[string]map[structs.ServiceName]context.CancelFunc

	// WatchedPeers is a map of peer name -> cancel functions. It is used to
	// track watches on peers.
	WatchedPeers map[string]context.CancelFunc

	// ServiceResolvers is a map of service name to an associated
	// service-resolver config entry for that service.
	ServiceResolvers map[structs.ServiceName]*structs.ServiceResolverConfigEntry

	// GatewayGroups is a map of datacenter names to services of kind
	// mesh-gateway in that datacenter.
	GatewayGroups map[string]structs.CheckServiceNodes

	// FedStateGateways is a map of datacenter names to mesh gateways in that
	// datacenter.
	FedStateGateways map[string]structs.CheckServiceNodes

	// WatchedLocalServers is a map of (structs.ConsulServiceName -> structs.CheckServiceNodes)`
	// Mesh gateways can spin up watches for local servers both for
	// WAN federation and for peering. This map ensures we only have one
	// watch at a time.
	WatchedLocalServers watch.Map[string, structs.CheckServiceNodes]

	// HostnameDatacenters is a map of datacenters to mesh gateway instances with a hostname as the address.
	// If hostnames are configured they must be provided to Envoy via CDS not EDS.
	HostnameDatacenters map[string]structs.CheckServiceNodes

	// ExportedServicesSlice is a sorted slice of services that are exported to
	// connected peers.
	ExportedServicesSlice []structs.ServiceName

	// ExportedServicesWithPeers is a map of exported service name to a sorted
	// slice of peers that they are exported to.
	ExportedServicesWithPeers map[structs.ServiceName][]string

	// ExportedServicesSet indicates that the watch on the list of
	// peer-exported services has completed at least once.
	ExportedServicesSet bool

	// DiscoveryChain is a map of the peer-exported service names to their
	// local compiled discovery chain. This will be populated regardless of
	// L4/L7 status of the chain.
	DiscoveryChain map[structs.ServiceName]*structs.CompiledDiscoveryChain

	// WatchedDiscoveryChains is a map of peer-exported service names to a
	// cancel function.
	WatchedDiscoveryChains map[structs.ServiceName]context.CancelFunc

	// MeshConfig is the mesh config entry that should be used for services
	// fronted by this mesh gateway.
	MeshConfig *structs.MeshConfigEntry

	// MeshConfigSet indicates that the watch on the mesh config entry has
	// completed at least once.
	MeshConfigSet bool

	// Leaf is the leaf cert to be used by this mesh gateway.
	Leaf *structs.IssuedCert

	// LeafCertWatchCancel is a CancelFunc to use when refreshing this gateway's
	// leaf cert watch with different parameters.
	LeafCertWatchCancel context.CancelFunc

	// PeerServers is the map of peering server names to their addresses.
	PeerServers map[string]PeerServersValue

	// PeerServersWatchCancel is a CancelFunc to use when resetting the watch
	// on all peerings as it is enabled/disabled.
	PeerServersWatchCancel context.CancelFunc

	// PeeringTrustBundles is the list of trust bundles for peers where
	// services have been exported to using this mesh gateway.
	PeeringTrustBundles []*pbpeering.PeeringTrustBundle

	// PeeringTrustBundlesSet indicates that the watch on the peer trust
	// bundles has completed at least once.
	PeeringTrustBundlesSet bool
}

// MeshGatewayValidExportedServices ensures that the following data is present
// if it exists for a service before it returns that in the set of services to
// expose.
//
// - peering info
// - discovery chain
func (c *ConfigSnapshot) MeshGatewayValidExportedServices() []structs.ServiceName {
	out := make([]structs.ServiceName, 0, len(c.MeshGateway.ExportedServicesSlice))
	for _, svc := range c.MeshGateway.ExportedServicesSlice {
		if _, ok := c.MeshGateway.ExportedServicesWithPeers[svc]; !ok {
			continue // not possible
		}

		if _, ok := c.MeshGateway.ServiceGroups[svc]; !ok {
			continue // unregistered services
		}

		chain, ok := c.MeshGateway.DiscoveryChain[svc]
		if !ok {
			continue // ignore; not ready
		}

		if structs.IsProtocolHTTPLike(chain.Protocol) {
			if c.MeshGateway.Leaf == nil {
				continue // ignore; not ready
			}
		}
		out = append(out, svc)
	}
	return out
}

func (c *ConfigSnapshot) GetMeshGatewayEndpoints(key GatewayKey) structs.CheckServiceNodes {
	// Mesh gateways in remote DCs are discovered in two ways:
	//
	//	1. Via an Internal.ServiceDump RPC in the remote DC (GatewayGroups).
	//	2. In the federation state that is replicated from the primary DC (FedStateGateways).
	//
	// We determine which set to use based on whichever contains the highest
	// raft ModifyIndex (and is therefore most up-to-date).
	//
	// Previously, GatewayGroups was always given presedence over FedStateGateways
	// but this was problematic when using mesh gateways for WAN federation.
	//
	// Consider the following example:
	//
	//	- Primary and Secondary DCs are WAN Federated via local mesh gateways.
	//
	//	- Secondary DC's mesh gateway is running on an ephemeral compute instance
	//	  and is abruptly terminated and rescheduled with a *new IP address*.
	//
	//	- Primary DC's mesh gateway is no longer able to connect to the Secondary
	//	  DC as its proxy is configured with the old IP address. Therefore any RPC
	//	  from the Primary to the Secondary DC will fail (including the one to
	//	  discover the gateway's new IP address).
	//
	//	- Secondary DC performs its regular anti-entropy of federation state data
	//	  to the Primary DC (this succeeds as there is still connectivity in this
	//	  direction).
	//
	//	- At this point the Primary DC's mesh gateway should observe the new IP
	//	  address and reconfigure its proxy, however as we always prioritised
	//	  GatewayGroups this didn't happen and the connection remained severed.
	maxModifyIndex := func(vals structs.CheckServiceNodes) uint64 {
		var max uint64
		for _, v := range vals {
			if i := v.Service.RaftIndex.ModifyIndex; i > max {
				max = i
			}
		}
		return max
	}

	endpoints := c.MeshGateway.GatewayGroups[key.String()]
	fedStateEndpoints := c.MeshGateway.FedStateGateways[key.String()]

	if maxModifyIndex(fedStateEndpoints) > maxModifyIndex(endpoints) {
		return fedStateEndpoints
	}
	return endpoints
}

func (c *configSnapshotMeshGateway) IsServiceExported(svc structs.ServiceName) bool {
	if c == nil || len(c.ExportedServicesWithPeers) == 0 {
		return false
	}

	_, ok := c.ExportedServicesWithPeers[svc]
	return ok
}

func (c *configSnapshotMeshGateway) GatewayKeys() []GatewayKey {
	sz1, sz2 := len(c.GatewayGroups), len(c.FedStateGateways)

	sz := sz1
	if sz2 > sz1 {
		sz = sz2
	}

	keys := make([]GatewayKey, 0, sz)
	for key := range c.FedStateGateways {
		keys = append(keys, gatewayKeyFromString(key))
	}
	for key := range c.GatewayGroups {
		gk := gatewayKeyFromString(key)
		if _, ok := c.FedStateGateways[gk.Datacenter]; !ok {
			keys = append(keys, gk)
		}
	}

	// Always sort the results to ensure we generate deterministic things over
	// xDS, such as mesh-gateway listener filter chains.
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Datacenter != keys[j].Datacenter {
			return keys[i].Datacenter < keys[j].Datacenter
		}
		return keys[i].Partition < keys[j].Partition
	})
	return keys
}

// isEmpty is a test helper
func (c *configSnapshotMeshGateway) isEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.WatchedServices) == 0 &&
		!c.WatchedServicesSet &&
		len(c.WatchedGateways) == 0 &&
		len(c.ServiceGroups) == 0 &&
		len(c.ServiceResolvers) == 0 &&
		len(c.GatewayGroups) == 0 &&
		len(c.FedStateGateways) == 0 &&
		len(c.HostnameDatacenters) == 0 &&
		c.WatchedLocalServers.Len() == 0 &&
		c.isEmptyPeering()
}

// isEmptyPeering is a test helper
func (c *configSnapshotMeshGateway) isEmptyPeering() bool {
	if c == nil {
		return true
	}

	return len(c.ExportedServicesSlice) == 0 &&
		len(c.ExportedServicesWithPeers) == 0 &&
		!c.ExportedServicesSet &&
		len(c.DiscoveryChain) == 0 &&
		len(c.WatchedDiscoveryChains) == 0 &&
		!c.MeshConfigSet &&
		c.LeafCertWatchCancel == nil &&
		c.Leaf == nil &&
		len(c.PeeringTrustBundles) == 0 &&
		!c.PeeringTrustBundlesSet
}

type upstreamIDSet map[UpstreamID]struct{}

func (u upstreamIDSet) add(uid UpstreamID) {
	u[uid] = struct{}{}
}

type routeUpstreamSet map[structs.ResourceReference]upstreamIDSet

func (r routeUpstreamSet) hasUpstream(uid UpstreamID) bool {
	for _, set := range r {
		if _, ok := set[uid]; ok {
			return true
		}
	}
	return false
}

func (r routeUpstreamSet) set(route structs.ResourceReference, set upstreamIDSet) {
	r[route] = set
}

func (r routeUpstreamSet) delete(route structs.ResourceReference) {
	delete(r, route)
}

type (
	listenerUpstreamMap    map[APIGatewayListenerKey]structs.Upstreams
	listenerRouteUpstreams map[structs.ResourceReference]listenerUpstreamMap
)

func (l listenerRouteUpstreams) set(route structs.ResourceReference, listener APIGatewayListenerKey, upstreams structs.Upstreams) {
	if _, ok := l[route]; !ok {
		l[route] = make(listenerUpstreamMap)
	}
	l[route][listener] = upstreams
}

func (l listenerRouteUpstreams) delete(route structs.ResourceReference) {
	delete(l, route)
}

func (l listenerRouteUpstreams) toUpstreams() map[IngressListenerKey]structs.Upstreams {
	listeners := make(map[IngressListenerKey]structs.Upstreams, len(l))
	for _, listenerMap := range l {
		for listener, set := range listenerMap {
			listeners[listener] = append(listeners[listener], set...)
		}
	}
	return listeners
}

type configSnapshotAPIGateway struct {
	ConfigSnapshotUpstreams

	TLSConfig structs.GatewayTLSConfig

	// GatewayConfigLoaded is used to determine if we have received the initial
	// api-gateway config entry yet.
	GatewayConfigLoaded bool
	GatewayConfig       *structs.APIGatewayConfigEntry

	// BoundGatewayConfigLoaded is used to determine if we have received the initial
	// bound-api-gateway config entry yet.
	BoundGatewayConfigLoaded bool
	BoundGatewayConfig       *structs.BoundAPIGatewayConfigEntry

	// LeafCertWatchCancel is a CancelFunc to use when refreshing this gateway's
	// leaf cert watch with different parameters.
	// LeafCertWatchCancel context.CancelFunc

	// Upstreams is a list of upstreams this ingress gateway should serve traffic
	// to. This is constructed from the ingress-gateway config entry, and uses
	// the GatewayServices RPC to retrieve them.
	// TODO Determine if this is updated "for free" or not. If not, we might need
	//   to do some work to populate it in handlerAPIGateway
	Upstreams listenerRouteUpstreams

	// UpstreamsSet is the unique set of UpstreamID the gateway routes to.
	UpstreamsSet routeUpstreamSet

	HTTPRoutes   watch.Map[structs.ResourceReference, *structs.HTTPRouteConfigEntry]
	TCPRoutes    watch.Map[structs.ResourceReference, *structs.TCPRouteConfigEntry]
	Certificates watch.Map[structs.ResourceReference, *structs.InlineCertificateConfigEntry]

	// LeafCertWatchCancel is a CancelFunc to use when refreshing this gateway's
	// leaf cert watch with different parameters.
	LeafCertWatchCancel context.CancelFunc

	// Listeners is the original listener config from the api-gateway config
	// entry to save us trying to pass fields through Upstreams
	Listeners map[string]structs.APIGatewayListener

	BoundListeners map[string]structs.BoundAPIGatewayListener
}

func (c *configSnapshotAPIGateway) synthesizeChains(datacenter string, listener structs.APIGatewayListener, boundListener structs.BoundAPIGatewayListener) ([]structs.IngressService, structs.Upstreams, []*structs.CompiledDiscoveryChain, error) {
	chains := []*structs.CompiledDiscoveryChain{}

	// We leverage the test trust domain knowing
	// that the domain will get overridden if
	// there is a target to something other than an
	// external/peered service. If the below
	// code doesn't get a trust domain due to all the
	// targets being external, the chain will
	// have the domain munged anyway during synthesis.
	trustDomain := connect.TestTrustDomain

DOMAIN_LOOP:
	for _, chain := range c.DiscoveryChain {
		for _, target := range chain.Targets {
			if !target.External {
				domain := connect.TrustDomainForTarget(*target)
				if domain != "" {
					trustDomain = domain
					break DOMAIN_LOOP
				}
			}
		}
	}

	synthesizer := discoverychain.NewGatewayChainSynthesizer(datacenter, trustDomain, listener.Name, c.GatewayConfig)
	synthesizer.SetHostname(listener.GetHostname())
	for _, routeRef := range boundListener.Routes {
		switch routeRef.Kind {
		case structs.HTTPRoute:
			route, ok := c.HTTPRoutes.Get(routeRef)
			if !ok || listener.Protocol != structs.ListenerProtocolHTTP {
				continue
			}
			synthesizer.AddHTTPRoute(*route)
			for _, service := range route.GetServices() {
				id := NewUpstreamIDFromServiceName(structs.NewServiceName(service.Name, &service.EnterpriseMeta))
				if chain := c.DiscoveryChain[id]; chain != nil {
					chains = append(chains, chain)
				}
			}
		case structs.TCPRoute:
			route, ok := c.TCPRoutes.Get(routeRef)
			if !ok || listener.Protocol != structs.ListenerProtocolTCP {
				continue
			}
			synthesizer.AddTCPRoute(*route)
			for _, service := range route.GetServices() {
				id := NewUpstreamIDFromServiceName(structs.NewServiceName(service.Name, &service.EnterpriseMeta))
				if chain := c.DiscoveryChain[id]; chain != nil {
					chains = append(chains, chain)
				}
			}
		default:
			return nil, nil, nil, fmt.Errorf("unknown route kind %q", routeRef.Kind)
		}
	}

	if len(chains) == 0 {
		return nil, nil, nil, nil
	}

	services, compiled, err := synthesizer.Synthesize(chains...)
	if err != nil {
		return nil, nil, nil, err
	}

	// reconstruct the upstreams
	upstreams := make([]structs.Upstream, 0, len(services))
	for _, service := range services {
		upstreams = append(upstreams, structs.Upstream{
			DestinationName:      service.Name,
			DestinationNamespace: service.NamespaceOrDefault(),
			DestinationPartition: service.PartitionOrDefault(),
			IngressHosts:         service.Hosts,
			LocalBindPort:        listener.Port,
			Config: map[string]interface{}{
				"protocol": string(listener.Protocol),
			},
		})
	}

	return services, upstreams, compiled, err
}

// valid tests for two valid api gateway snapshot states:
//  1. waiting: the watch on api and bound gateway entries is set, but none were received
//  2. loaded: both the valid config entries AND the leaf certs are set
func (c *configSnapshotAPIGateway) valid() bool {
	waiting := c.GatewayConfigLoaded && len(c.Upstreams) == 0 && c.BoundGatewayConfigLoaded && c.Leaf == nil

	// If we have a leaf, it implies we successfully watched parent resources
	loaded := c.GatewayConfigLoaded && c.BoundGatewayConfigLoaded && c.Leaf != nil

	return waiting || loaded
}

type configSnapshotIngressGateway struct {
	ConfigSnapshotUpstreams

	// TLSConfig is the gateway-level TLS configuration. Listener/service level
	// config is preserved in the Listeners map below.
	TLSConfig structs.GatewayTLSConfig

	// GatewayConfigLoaded is used to determine if we have received the initial
	// ingress-gateway config entry yet.
	GatewayConfigLoaded bool

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

	// UpstreamsSet is the unique set of UpstreamID the gateway routes to.
	UpstreamsSet map[UpstreamID]struct{}

	// Listeners is the original listener config from the ingress-gateway config
	// entry to save us trying to pass fields through Upstreams
	Listeners map[IngressListenerKey]structs.IngressListener

	// Defaults is the default configuration for upstream service instances
	Defaults structs.IngressServiceConfig
}

// isEmpty is a test helper
func (c *configSnapshotIngressGateway) isEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.Upstreams) == 0 &&
		len(c.UpstreamsSet) == 0 &&
		len(c.DiscoveryChain) == 0 &&
		len(c.WatchedUpstreams) == 0 &&
		len(c.WatchedUpstreamEndpoints) == 0 &&
		!c.MeshConfigSet
}

// valid tests for two valid ingress snapshot states:
//  1. waiting: the watch on ingress config entries is set, but none were received
//  2. loaded: both the ingress config entry AND the leaf cert are set
func (c *configSnapshotIngressGateway) valid() bool {
	waiting := c.GatewayConfigLoaded && len(c.Upstreams) == 0 && c.Leaf == nil

	// If we have a leaf, it implies we successfully watched parent resources
	loaded := c.GatewayConfigLoaded && c.Leaf != nil

	return waiting || loaded
}

type APIGatewayListenerKey = IngressListenerKey

func APIGatewayListenerKeyFromListener(l structs.APIGatewayListener) APIGatewayListenerKey {
	return APIGatewayListenerKey{Protocol: string(l.Protocol), Port: l.Port}
}

type IngressListenerKey struct {
	Protocol string
	Port     int
}

func (k *IngressListenerKey) RouteName() string {
	return fmt.Sprintf("%d", k.Port)
}

func IngressListenerKeyFromGWService(s structs.GatewayService) IngressListenerKey {
	return IngressListenerKey{Protocol: s.Protocol, Port: s.Port}
}

func IngressListenerKeyFromListener(l structs.IngressListener) IngressListenerKey {
	return IngressListenerKey{Protocol: l.Protocol, Port: l.Port}
}

// ConfigSnapshot captures all the resulting config needed for a proxy instance.
// It is meant to be point-in-time coherent and is used to deliver the current
// config state to observers who need it to be pushed in (e.g. XDS server).
type ConfigSnapshot struct {
	Kind                  structs.ServiceKind
	Service               string
	ServiceLocality       *structs.Locality
	ProxyID               ProxyID
	Address               string
	Port                  int
	ServiceMeta           map[string]string
	TaggedAddresses       map[string]structs.ServiceAddress
	Proxy                 structs.ConnectProxyConfig
	Datacenter            string
	IntentionDefaultAllow bool
	Locality              GatewayKey
	JWTProviders          map[string]*structs.JWTProviderConfigEntry

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

	// api-gateway specific
	APIGateway configSnapshotAPIGateway
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
			s.ConnectProxy.IntentionsSet &&
			s.ConnectProxy.MeshConfigSet

	case structs.ServiceKindTerminatingGateway:
		return s.Roots != nil &&
			s.TerminatingGateway.MeshConfigSet

	case structs.ServiceKindMeshGateway:
		if s.MeshGateway.WatchedLocalServers.Len() == 0 {
			if s.ServiceMeta[structs.MetaWANFederationKey] == "1" {
				return false
			}
			if cfg := s.MeshConfig(); cfg.PeerThroughMeshGateways() {
				return false
			}
		}
		return s.Roots != nil &&
			(s.MeshGateway.WatchedServicesSet || len(s.MeshGateway.ServiceGroups) > 0) &&
			s.MeshGateway.ExportedServicesSet &&
			s.MeshGateway.MeshConfigSet &&
			s.MeshGateway.PeeringTrustBundlesSet

	case structs.ServiceKindIngressGateway:
		return s.Roots != nil &&
			s.IngressGateway.valid() &&
			s.IngressGateway.HostsSet &&
			s.IngressGateway.MeshConfigSet

	case structs.ServiceKindAPIGateway:
		// TODO Is this the proper set of things to validate?
		return s.Roots != nil &&
			s.APIGateway.valid() &&
			s.APIGateway.MeshConfigSet
	default:
		return false
	}
}

// Clone makes a deep copy of the snapshot we can send to other goroutines
// without worrying that they will racily read or mutate shared maps etc.
func (s *ConfigSnapshot) Clone() *ConfigSnapshot {
	snap := s.DeepCopy()

	// nil these out as anything receiving one of these clones does not need them and should never "cancel" our watches
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		// common with connect-proxy and ingress-gateway
		snap.ConnectProxy.WatchedUpstreams = nil
		snap.ConnectProxy.WatchedGateways = nil
		snap.ConnectProxy.WatchedDiscoveryChains = nil
	case structs.ServiceKindTerminatingGateway:
		snap.TerminatingGateway.WatchedServices = nil
		snap.TerminatingGateway.WatchedIntentions = nil
		snap.TerminatingGateway.WatchedLeaves = nil
		snap.TerminatingGateway.WatchedConfigs = nil
		snap.TerminatingGateway.WatchedResolvers = nil
	case structs.ServiceKindMeshGateway:
		snap.MeshGateway.WatchedGateways = nil
		snap.MeshGateway.WatchedServices = nil
	case structs.ServiceKindIngressGateway:
		// common with connect-proxy and ingress-gateway
		snap.IngressGateway.WatchedUpstreams = nil
		snap.IngressGateway.WatchedGateways = nil
		snap.IngressGateway.WatchedDiscoveryChains = nil
		// only ingress-gateway
		snap.IngressGateway.LeafCertWatchCancel = nil
	case structs.ServiceKindAPIGateway:
		// common with connect-proxy and api-gateway
		snap.APIGateway.WatchedUpstreams = nil
		snap.APIGateway.WatchedGateways = nil
		snap.APIGateway.WatchedDiscoveryChains = nil

		// only api-gateway
		// snap.APIGateway.LeafCertWatchCancel = nil
		// snap.APIGateway.
	}

	return snap
}

func (s *ConfigSnapshot) Leaf() *structs.IssuedCert {
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		return s.ConnectProxy.Leaf
	case structs.ServiceKindIngressGateway:
		return s.IngressGateway.Leaf
	case structs.ServiceKindAPIGateway:
		return s.APIGateway.Leaf
	case structs.ServiceKindMeshGateway:
		return s.MeshGateway.Leaf
	default:
		return nil
	}
}

func (s *ConfigSnapshot) PeeringTrustBundles() []*pbpeering.PeeringTrustBundle {
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		return s.ConnectProxy.InboundPeerTrustBundles
	case structs.ServiceKindMeshGateway:
		return s.MeshGateway.PeeringTrustBundles
	default:
		return nil
	}
}

// RootPEMs returns all PEM-encoded public certificates for the root CA.
func (s *ConfigSnapshot) RootPEMs() string {
	var rootPEMs string
	for _, root := range s.Roots.Roots {
		rootPEMs += lib.EnsureTrailingNewline(root.RootCert)
	}
	return rootPEMs
}

func (s *ConfigSnapshot) MeshConfig() *structs.MeshConfigEntry {
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		return s.ConnectProxy.MeshConfig
	case structs.ServiceKindIngressGateway:
		return s.IngressGateway.MeshConfig
	case structs.ServiceKindAPIGateway:
		return s.APIGateway.MeshConfig
	case structs.ServiceKindTerminatingGateway:
		return s.TerminatingGateway.MeshConfig
	case structs.ServiceKindMeshGateway:
		return s.MeshGateway.MeshConfig
	default:
		return nil
	}
}

func (s *ConfigSnapshot) MeshConfigTLSIncoming() *structs.MeshDirectionalTLSConfig {
	mesh := s.MeshConfig()
	if mesh == nil || mesh.TLS == nil {
		return nil
	}
	return mesh.TLS.Incoming
}

func (s *ConfigSnapshot) MeshConfigTLSOutgoing() *structs.MeshDirectionalTLSConfig {
	mesh := s.MeshConfig()
	if mesh == nil || mesh.TLS == nil {
		return nil
	}
	return mesh.TLS.Outgoing
}

func (s *ConfigSnapshot) ToConfigSnapshotUpstreams() (*ConfigSnapshotUpstreams, error) {
	switch s.Kind {
	case structs.ServiceKindConnectProxy:
		return &s.ConnectProxy.ConfigSnapshotUpstreams, nil
	case structs.ServiceKindIngressGateway:
		return &s.IngressGateway.ConfigSnapshotUpstreams, nil
	case structs.ServiceKindAPIGateway:
		return &s.APIGateway.ConfigSnapshotUpstreams, nil
	default:
		// This is a coherence check and should never fail
		return nil, fmt.Errorf("No upstream snapshot for gateway mode %q", s.Kind)
	}
}

func (u *ConfigSnapshotUpstreams) UpstreamPeerMeta(uid UpstreamID) (structs.PeeringServiceMeta, bool) {
	nodes, _ := u.PeerUpstreamEndpoints.Get(uid)
	if len(nodes) == 0 {
		return structs.PeeringServiceMeta{}, false
	}

	// In agent/rpc/peering/subscription_manager.go we denormalize the
	// PeeringServiceMeta data onto each replicated service instance to convey
	// this information back to the importing side of the peering.
	//
	// This data is guaranteed (subject to any eventual consistency lag around
	// updates) to be the same across all instances, so we only need to take
	// the first item.
	//
	// TODO(peering): consider replicating this "common to all instances" data
	// using a different replication type and persist it separately in the
	// catalog to avoid this weird construction.
	csn := nodes[0]
	if csn.Service == nil {
		return structs.PeeringServiceMeta{}, false
	}
	return *csn.Service.Connect.PeerMeta, true
}

// PeeredUpstreamIDs returns a slice of peered UpstreamIDs from explicit config entries
// and implicit imported services.
// Upstreams whose trust bundles have not been stored in the snapshot are ignored.
func (u *ConfigSnapshotUpstreams) PeeredUpstreamIDs() []UpstreamID {
	out := make([]UpstreamID, 0, u.PeerUpstreamEndpoints.Len())
	u.PeerUpstreamEndpoints.ForEachKey(func(uid UpstreamID) bool {
		if _, ok := u.PeerUpstreamEndpoints.Get(uid); !ok {
			// uid might exist in the map but if Set hasn't been called, skip for now.
			return true
		}

		if _, ok := u.UpstreamPeerTrustBundles.Get(uid.Peer); !ok {
			// The trust bundle for this upstream is not available yet, skip for now.
			return true
		}
		out = append(out, uid)
		return true
	})
	return out
}
