package xds

import (
	"errors"
	"fmt"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/proto"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"

	bexpr "github.com/hashicorp/go-bexpr"
)

// endpointsFromSnapshot returns the xDS API representation of the "endpoints"
func (s *Server) endpointsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.endpointsFromSnapshotConnectProxy(cfgSnap, token)
	case structs.ServiceKindMeshGateway:
		return s.endpointsFromSnapshotMeshGateway(cfgSnap, token)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// endpointsFromSnapshotConnectProxy returns the xDS API representation of the "endpoints"
// (upstream instances) in the snapshot.
func (s *Server) endpointsFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	resources := make([]proto.Message, 0,
		len(cfgSnap.ConnectProxy.UpstreamEndpoints)+len(cfgSnap.ConnectProxy.WatchedUpstreamEndpoints))

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

			endpoints, ok := cfgSnap.ConnectProxy.UpstreamEndpoints[id]
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

			// Find all resolver nodes.
			for _, node := range chain.Nodes {
				if node.Type != structs.DiscoveryGraphNodeTypeResolver {
					continue
				}
				failover := node.Resolver.Failover
				targetID := node.Resolver.Target

				target := chain.Targets[targetID]

				clusterName := CustomizeClusterName(target.Name, chain)

				// Determine if we have to generate the entire cluster differently.
				failoverThroughMeshGateway := chain.WillFailoverThroughMeshGateway(node)

				if failoverThroughMeshGateway {
					actualTargetID := firstHealthyTarget(
						chain.Targets,
						cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[id],
						targetID,
						failover.Targets,
					)
					if actualTargetID != targetID {
						targetID = actualTargetID
						target = chain.Targets[actualTargetID]
					}

					failover = nil
				}

				primaryGroup, valid := makeLoadAssignmentEndpointGroup(
					chain.Targets,
					cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[id],
					cfgSnap.ConnectProxy.WatchedGatewayEndpoints[id],
					targetID,
					cfgSnap.Datacenter,
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
							cfgSnap.ConnectProxy.WatchedUpstreamEndpoints[id],
							cfgSnap.ConnectProxy.WatchedGatewayEndpoints[id],
							failTargetID,
							cfgSnap.Datacenter,
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
					cfgSnap.Datacenter,
				)
				resources = append(resources, la)
			}
		}
	}

	return resources, nil
}

func (s *Server) endpointsFromSnapshotMeshGateway(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	resources := make([]proto.Message, 0, len(cfgSnap.MeshGateway.GatewayGroups)+len(cfgSnap.MeshGateway.ServiceGroups))

	// generate the endpoints for the gateways in the remote datacenters
	for dc, endpoints := range cfgSnap.MeshGateway.GatewayGroups {
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

	// generate the endpoints for the local service groups
	for svc, endpoints := range cfgSnap.MeshGateway.ServiceGroups {
		clusterName := connect.ServiceSNI(svc, "", "default", cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
		la := makeLoadAssignment(
			clusterName,
			[]loadAssignmentEndpointGroup{
				{Endpoints: endpoints},
			},
			cfgSnap.Datacenter,
		)
		resources = append(resources, la)
	}

	// generate the endpoints for the service subsets
	for svc, resolver := range cfgSnap.MeshGateway.ServiceResolvers {
		for subsetName, subset := range resolver.Subsets {
			clusterName := connect.ServiceSNI(svc, subsetName, "default", cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

			endpoints := cfgSnap.MeshGateway.ServiceGroups[svc]

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
				endpoints = raw.(structs.CheckServiceNodes)
			}

			la := makeLoadAssignment(
				clusterName,
				[]loadAssignmentEndpointGroup{
					{
						Endpoints:   endpoints,
						OnlyPassing: subset.OnlyPassing,
					},
				},
				cfgSnap.Datacenter,
			)
			resources = append(resources, la)
		}
	}

	return resources, nil
}

func makeEndpoint(clusterName, host string, port int) envoyendpoint.LbEndpoint {
	return envoyendpoint.LbEndpoint{
		HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
			Endpoint: &envoyendpoint.Endpoint{
				Address: makeAddressPtr(host, port),
			},
		},
	}
}

type loadAssignmentEndpointGroup struct {
	Endpoints      structs.CheckServiceNodes
	OnlyPassing    bool
	OverrideHealth envoycore.HealthStatus
}

func makeLoadAssignment(clusterName string, endpointGroups []loadAssignmentEndpointGroup, localDatacenter string) *envoy.ClusterLoadAssignment {
	cla := &envoy.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   make([]envoyendpoint.LocalityLbEndpoints, 0, len(endpointGroups)),
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
		es := make([]envoyendpoint.LbEndpoint, 0, len(endpoints))

		for _, ep := range endpoints {
			// TODO (mesh-gateway) - should we respect the translate_wan_addrs configuration here or just always use the wan for cross-dc?
			addr, port := ep.BestAddress(localDatacenter != ep.Node.Datacenter)
			healthStatus, weight := calculateEndpointHealthAndWeight(ep, endpointGroup.OnlyPassing)

			if endpointGroup.OverrideHealth != envoycore.HealthStatus_UNKNOWN {
				healthStatus = endpointGroup.OverrideHealth
			}

			es = append(es, envoyendpoint.LbEndpoint{
				HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
					Endpoint: &envoyendpoint.Endpoint{
						Address: makeAddressPtr(addr, port),
					},
				},
				HealthStatus:        healthStatus,
				LoadBalancingWeight: makeUint32Value(weight),
			})
		}

		cla.Endpoints = append(cla.Endpoints, envoyendpoint.LocalityLbEndpoints{
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
