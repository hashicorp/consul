package xds

import (
	"errors"
	"fmt"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	bexpr "github.com/hashicorp/go-bexpr"
)

const (
	UnnamedSubset = ""
)

// endpointsFromSnapshot returns the xDS API representation of the "endpoints"
func (s *Server) endpointsFromSnapshot(_ connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.endpointsFromSnapshotConnectProxy(cfgSnap)
	case structs.ServiceKindTerminatingGateway:
		return s.endpointsFromSnapshotTerminatingGateway(cfgSnap)
	case structs.ServiceKindMeshGateway:
		return s.endpointsFromSnapshotMeshGateway(cfgSnap)
	case structs.ServiceKindIngressGateway:
		return s.endpointsFromSnapshotIngressGateway(cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// endpointsFromSnapshotConnectProxy returns the xDS API representation of the "endpoints"
// (upstream instances) in the snapshot.
func (s *Server) endpointsFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	resources := make([]proto.Message, 0,
		len(cfgSnap.ConnectProxy.PreparedQueryEndpoints)+len(cfgSnap.ConnectProxy.WatchedUpstreamEndpoints))

	for _, u := range cfgSnap.Proxy.Upstreams {
		id := u.Identifier()

		var chain *structs.CompiledDiscoveryChain
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			chain = cfgSnap.ConnectProxy.DiscoveryChain[id]
		}

		if chain == nil {
			// We ONLY want this branch for prepared queries.

			dc := u.Datacenter
			if dc == "" {
				dc = cfgSnap.Datacenter
			}
			clusterName := connect.UpstreamSNI(&u, "", dc, cfgSnap.Roots.TrustDomain)

			endpoints, ok := cfgSnap.ConnectProxy.PreparedQueryEndpoints[id]
			if ok {
				la := makeLoadAssignment(
					clusterName,
					[]loadAssignmentEndpointGroup{
						{Endpoints: endpoints},
					},
					cfgSnap.Datacenter,
				)
				resources = append(resources, la)
			}

		} else {
			// Newfangled discovery chain plumbing.
			es := s.endpointsFromDiscoveryChain(
				u,
				chain,
				cfgSnap.Datacenter,
				cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[id],
				cfgSnap.ConnectProxy.WatchedGatewayEndpoints[id],
			)
			resources = append(resources, es...)
		}
	}

	return resources, nil
}

func (s *Server) filterSubsetEndpoints(subset *structs.ServiceResolverSubset, endpoints structs.CheckServiceNodes) (structs.CheckServiceNodes, error) {
	// locally execute the subsets filter
	if subset.Filter != "" {
		filter, err := bexpr.CreateFilter(subset.Filter, nil, endpoints)
		if err != nil {
			return nil, err
		}

		raw, err := filter.Execute(endpoints)
		if err != nil {
			return nil, err
		}
		return raw.(structs.CheckServiceNodes), nil
	}
	return endpoints, nil
}

func (s *Server) endpointsFromSnapshotTerminatingGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	return s.endpointsFromServicesAndResolvers(cfgSnap, cfgSnap.TerminatingGateway.ServiceGroups, cfgSnap.TerminatingGateway.ServiceResolvers)
}

func (s *Server) endpointsFromSnapshotMeshGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	datacenters := cfgSnap.MeshGateway.Datacenters()
	resources := make([]proto.Message, 0, len(datacenters)+len(cfgSnap.MeshGateway.ServiceGroups))

	// generate the endpoints for the gateways in the remote datacenters
	for _, dc := range datacenters {
		// Skip creating endpoints for mesh gateways in local DC and gateways in remote DCs with a hostname as their address
		// EDS cannot resolve hostnames so we provide them through CDS instead
		if dc == cfgSnap.Datacenter || len(cfgSnap.MeshGateway.HostnameDatacenters[dc]) > 0 {
			continue
		}

		endpoints, ok := cfgSnap.MeshGateway.GatewayGroups[dc]
		if !ok {
			endpoints, ok = cfgSnap.MeshGateway.FedStateGateways[dc]
			if !ok { // not possible
				s.Logger.Error("skipping mesh gateway endpoints because no definition found", "datacenter", dc)
				continue
			}
		}

		{ // standard connect
			clusterName := connect.DatacenterSNI(dc, cfgSnap.Roots.TrustDomain)

			la := makeLoadAssignment(
				clusterName,
				[]loadAssignmentEndpointGroup{
					{Endpoints: endpoints},
				},
				cfgSnap.Datacenter,
			)
			resources = append(resources, la)
		}

		if cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" && cfgSnap.ServerSNIFn != nil {
			clusterName := cfgSnap.ServerSNIFn(dc, "")

			la := makeLoadAssignment(
				clusterName,
				[]loadAssignmentEndpointGroup{
					{Endpoints: endpoints},
				},
				cfgSnap.Datacenter,
			)
			resources = append(resources, la)
		}
	}

	// generate endpoints for our servers if WAN federation is enabled
	if cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" && cfgSnap.ServerSNIFn != nil {
		var allServersLbEndpoints []*envoyendpoint.LbEndpoint

		for _, srv := range cfgSnap.MeshGateway.ConsulServers {
			clusterName := cfgSnap.ServerSNIFn(cfgSnap.Datacenter, srv.Node.Node)

			addr, port := srv.BestAddress(false /*wan*/)

			lbEndpoint := &envoyendpoint.LbEndpoint{
				HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
					Endpoint: &envoyendpoint.Endpoint{
						Address: makeAddress(addr, port),
					},
				},
				HealthStatus: envoycore.HealthStatus_UNKNOWN,
			}

			cla := &envoy.ClusterLoadAssignment{
				ClusterName: clusterName,
				Endpoints: []*envoyendpoint.LocalityLbEndpoints{{
					LbEndpoints: []*envoyendpoint.LbEndpoint{lbEndpoint},
				}},
			}
			allServersLbEndpoints = append(allServersLbEndpoints, lbEndpoint)

			resources = append(resources, cla)
		}

		// And add one catch all so that remote datacenters can dial ANY server
		// in this datacenter without knowing its name.
		resources = append(resources, &envoy.ClusterLoadAssignment{
			ClusterName: cfgSnap.ServerSNIFn(cfgSnap.Datacenter, ""),
			Endpoints: []*envoyendpoint.LocalityLbEndpoints{{
				LbEndpoints: allServersLbEndpoints,
			}},
		})
	}

	// Generate the endpoints for each service and its subsets
	e, err := s.endpointsFromServicesAndResolvers(cfgSnap, cfgSnap.MeshGateway.ServiceGroups, cfgSnap.MeshGateway.ServiceResolvers)
	if err != nil {
		return nil, err
	}
	resources = append(resources, e...)

	return resources, nil
}

func (s *Server) endpointsFromServicesAndResolvers(
	cfgSnap *proxycfg.ConfigSnapshot,
	services map[structs.ServiceName]structs.CheckServiceNodes,
	resolvers map[structs.ServiceName]*structs.ServiceResolverConfigEntry) ([]proto.Message, error) {

	resources := make([]proto.Message, 0, len(services))

	// generate the endpoints for the linked service groups
	for svc, endpoints := range services {
		// Skip creating endpoints for services that have hostnames as addresses
		// EDS cannot resolve hostnames so we provide them through CDS instead
		if cfgSnap.Kind == structs.ServiceKindTerminatingGateway && len(cfgSnap.TerminatingGateway.HostnameServices[svc]) > 0 {
			continue
		}

		clusterEndpoints := make(map[string][]loadAssignmentEndpointGroup)
		clusterEndpoints[UnnamedSubset] = []loadAssignmentEndpointGroup{{Endpoints: endpoints, OnlyPassing: false}}

		// Collect all of the loadAssignmentEndpointGroups for the various subsets. We do this before generating
		// the endpoints for the default/unnamed subset so that we can take into account the DefaultSubset on the
		// service-resolver which may prevent the default/unnamed cluster from creating endpoints for all service
		// instances.
		if resolver, hasResolver := resolvers[svc]; hasResolver {
			for subsetName, subset := range resolver.Subsets {
				subsetEndpoints, err := s.filterSubsetEndpoints(&subset, endpoints)
				if err != nil {
					return nil, err
				}
				groups := []loadAssignmentEndpointGroup{{Endpoints: subsetEndpoints, OnlyPassing: subset.OnlyPassing}}
				clusterEndpoints[subsetName] = groups

				// if this subset is the default then override the unnamed subset with this configuration
				if subsetName == resolver.DefaultSubset {
					clusterEndpoints[UnnamedSubset] = groups
				}
			}
		}

		// now generate the load assignment for all subsets
		for subsetName, groups := range clusterEndpoints {
			clusterName := connect.ServiceSNI(svc.Name, subsetName, svc.NamespaceOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
			la := makeLoadAssignment(
				clusterName,
				groups,
				cfgSnap.Datacenter,
			)
			resources = append(resources, la)
		}
	}

	return resources, nil
}

func (s *Server) endpointsFromSnapshotIngressGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message
	createdClusters := make(map[string]bool)
	for _, upstreams := range cfgSnap.IngressGateway.Upstreams {
		for _, u := range upstreams {
			id := u.Identifier()

			// If we've already created endpoints for this upstream, skip it. Multiple listeners may
			// reference the same upstream, so we don't need to create duplicate endpoints in that case.
			if createdClusters[id] {
				continue
			}

			es := s.endpointsFromDiscoveryChain(
				u,
				cfgSnap.IngressGateway.DiscoveryChain[id],
				cfgSnap.Datacenter,
				cfgSnap.IngressGateway.WatchedUpstreamEndpoints[id],
				cfgSnap.IngressGateway.WatchedGatewayEndpoints[id],
			)
			resources = append(resources, es...)
			createdClusters[id] = true
		}
	}
	return resources, nil
}

func makeEndpoint(host string, port int) *envoyendpoint.LbEndpoint {
	return &envoyendpoint.LbEndpoint{
		HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
			Endpoint: &envoyendpoint.Endpoint{
				Address: makeAddress(host, port),
			},
		},
	}
}

func (s *Server) endpointsFromDiscoveryChain(
	upstream structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
	datacenter string,
	upstreamEndpoints, gatewayEndpoints map[string]structs.CheckServiceNodes,
) []proto.Message {
	var resources []proto.Message

	if chain == nil {
		return resources
	}

	cfg, err := ParseUpstreamConfigNoDefaults(upstream.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse", "upstream", upstream.Identifier(),
			"error", err)
	}

	var escapeHatchCluster *envoy.Cluster
	if cfg.ClusterJSON != "" {
		if chain.IsDefault() {
			// If you haven't done anything to setup the discovery chain, then
			// you can use the envoy_cluster_json escape hatch.
			escapeHatchCluster, err = makeClusterFromUserConfig(cfg.ClusterJSON)
			if err != nil {
				return resources
			}
		} else {
			s.Logger.Warn("ignoring escape hatch setting, because a discovery chain is configued for",
				"discovery chain", chain.ServiceName, "upstream", upstream.Identifier(),
				"envoy_cluster_json", chain.ServiceName)
		}
	}

	// Find all resolver nodes.
	for _, node := range chain.Nodes {
		if node.Type != structs.DiscoveryGraphNodeTypeResolver {
			continue
		}
		failover := node.Resolver.Failover
		targetID := node.Resolver.Target

		target := chain.Targets[targetID]

		clusterName := CustomizeClusterName(target.Name, chain)
		if escapeHatchCluster != nil {
			clusterName = escapeHatchCluster.Name
		}
		s.Logger.Debug("generating endpoints for", "cluster", clusterName)

		// Determine if we have to generate the entire cluster differently.
		failoverThroughMeshGateway := chain.WillFailoverThroughMeshGateway(node)

		if failoverThroughMeshGateway {
			actualTargetID := firstHealthyTarget(
				chain.Targets,
				upstreamEndpoints,
				targetID,
				failover.Targets,
			)
			if actualTargetID != targetID {
				targetID = actualTargetID
			}

			failover = nil
		}

		primaryGroup, valid := makeLoadAssignmentEndpointGroup(
			chain.Targets,
			upstreamEndpoints,
			gatewayEndpoints,
			targetID,
			datacenter,
		)
		if !valid {
			continue // skip the cluster if we're still populating the snapshot
		}

		var endpointGroups []loadAssignmentEndpointGroup

		if failover != nil && len(failover.Targets) > 0 {
			endpointGroups = make([]loadAssignmentEndpointGroup, 0, len(failover.Targets)+1)

			endpointGroups = append(endpointGroups, primaryGroup)

			for _, failTargetID := range failover.Targets {
				failoverGroup, valid := makeLoadAssignmentEndpointGroup(
					chain.Targets,
					upstreamEndpoints,
					gatewayEndpoints,
					failTargetID,
					datacenter,
				)
				if !valid {
					continue // skip the failover target if we're still populating the snapshot
				}
				endpointGroups = append(endpointGroups, failoverGroup)
			}
		} else {
			endpointGroups = append(endpointGroups, primaryGroup)
		}

		la := makeLoadAssignment(
			clusterName,
			endpointGroups,
			datacenter,
		)
		resources = append(resources, la)
	}

	return resources
}

type loadAssignmentEndpointGroup struct {
	Endpoints      structs.CheckServiceNodes
	OnlyPassing    bool
	OverrideHealth envoycore.HealthStatus
}

func makeLoadAssignment(clusterName string, endpointGroups []loadAssignmentEndpointGroup, localDatacenter string) *envoy.ClusterLoadAssignment {
	cla := &envoy.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   make([]*envoyendpoint.LocalityLbEndpoints, 0, len(endpointGroups)),
	}

	if len(endpointGroups) > 1 {
		cla.Policy = &envoy.ClusterLoadAssignment_Policy{
			// We choose such a large value here that the failover math should
			// in effect not happen until zero instances are healthy.
			OverprovisioningFactor: makeUint32Value(100000),
		}
	}

	for priority, endpointGroup := range endpointGroups {
		endpoints := endpointGroup.Endpoints
		es := make([]*envoyendpoint.LbEndpoint, 0, len(endpoints))

		for _, ep := range endpoints {
			// TODO (mesh-gateway) - should we respect the translate_wan_addrs configuration here or just always use the wan for cross-dc?
			addr, port := ep.BestAddress(localDatacenter != ep.Node.Datacenter)
			healthStatus, weight := calculateEndpointHealthAndWeight(ep, endpointGroup.OnlyPassing)

			if endpointGroup.OverrideHealth != envoycore.HealthStatus_UNKNOWN {
				healthStatus = endpointGroup.OverrideHealth
			}

			es = append(es, &envoyendpoint.LbEndpoint{
				HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
					Endpoint: &envoyendpoint.Endpoint{
						Address: makeAddress(addr, port),
					},
				},
				HealthStatus:        healthStatus,
				LoadBalancingWeight: makeUint32Value(weight),
			})
		}

		cla.Endpoints = append(cla.Endpoints, &envoyendpoint.LocalityLbEndpoints{
			Priority:    uint32(priority),
			LbEndpoints: es,
		})
	}

	return cla
}

func makeLoadAssignmentEndpointGroup(
	targets map[string]*structs.DiscoveryTarget,
	targetHealth map[string]structs.CheckServiceNodes,
	gatewayHealth map[string]structs.CheckServiceNodes,
	targetID string,
	currentDatacenter string,
) (loadAssignmentEndpointGroup, bool) {
	realEndpoints, ok := targetHealth[targetID]
	if !ok {
		// skip the cluster if we're still populating the snapshot
		return loadAssignmentEndpointGroup{}, false
	}
	target := targets[targetID]

	var gatewayDatacenter string
	switch target.MeshGateway.Mode {
	case structs.MeshGatewayModeRemote:
		gatewayDatacenter = target.Datacenter
	case structs.MeshGatewayModeLocal:
		gatewayDatacenter = currentDatacenter
	}

	if gatewayDatacenter == "" {
		return loadAssignmentEndpointGroup{
			Endpoints:   realEndpoints,
			OnlyPassing: target.Subset.OnlyPassing,
		}, true
	}

	// If using a mesh gateway we need to pull those endpoints instead.
	gatewayEndpoints, ok := gatewayHealth[gatewayDatacenter]
	if !ok {
		// skip the cluster if we're still populating the snapshot
		return loadAssignmentEndpointGroup{}, false
	}

	// But we will use the health from the actual backend service.
	overallHealth := envoycore.HealthStatus_UNHEALTHY
	for _, ep := range realEndpoints {
		health, _ := calculateEndpointHealthAndWeight(ep, target.Subset.OnlyPassing)
		if health == envoycore.HealthStatus_HEALTHY {
			overallHealth = envoycore.HealthStatus_HEALTHY
			break
		}
	}

	return loadAssignmentEndpointGroup{
		Endpoints:      gatewayEndpoints,
		OverrideHealth: overallHealth,
	}, true
}

func calculateEndpointHealthAndWeight(
	ep structs.CheckServiceNode,
	onlyPassing bool,
) (envoycore.HealthStatus, int) {
	healthStatus := envoycore.HealthStatus_HEALTHY
	weight := 1
	if ep.Service.Weights != nil {
		weight = ep.Service.Weights.Passing
	}

	for _, chk := range ep.Checks {
		if chk.Status == api.HealthCritical {
			healthStatus = envoycore.HealthStatus_UNHEALTHY
		}
		if onlyPassing && chk.Status != api.HealthPassing {
			healthStatus = envoycore.HealthStatus_UNHEALTHY
		}
		if chk.Status == api.HealthWarning && ep.Service.Weights != nil {
			weight = ep.Service.Weights.Warning
		}
	}
	// Make weights fit Envoy's limits. A zero weight means that either Warning
	// (likely) or Passing (weirdly) weight has been set to 0 effectively making
	// this instance unhealthy and should not be sent traffic.
	if weight < 1 {
		healthStatus = envoycore.HealthStatus_UNHEALTHY
		weight = 1
	}
	if weight > 128 {
		weight = 128
	}
	return healthStatus, weight
}
