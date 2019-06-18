package xds

import (
	"errors"
	"fmt"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
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
	resources := make([]proto.Message, 0, len(cfgSnap.ConnectProxy.UpstreamEndpoints))
	for id, endpoints := range cfgSnap.ConnectProxy.UpstreamEndpoints {
		la := makeLoadAssignment(id, endpoints, cfgSnap.Datacenter)
		resources = append(resources, la)
	}
	return resources, nil
}

func (s *Server) endpointsFromSnapshotMeshGateway(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	resources := make([]proto.Message, 0, len(cfgSnap.MeshGateway.GatewayGroups)+len(cfgSnap.MeshGateway.ServiceGroups))

	// generate the endpoints for the gateways in the remote datacenters
	for dc, endpoints := range cfgSnap.MeshGateway.GatewayGroups {
		clusterName := DatacenterSNI(dc, cfgSnap)
		la := makeLoadAssignment(clusterName, endpoints, cfgSnap.Datacenter)
		resources = append(resources, la)
	}

	// generate the endpoints for the local service groups
	for svc, endpoints := range cfgSnap.MeshGateway.ServiceGroups {
		clusterName := ServiceSNI(svc, "default", cfgSnap.Datacenter, cfgSnap)
		la := makeLoadAssignment(clusterName, endpoints, cfgSnap.Datacenter)
		resources = append(resources, la)
	}

	return resources, nil
}

func makeEndpoint(clusterName, host string, port int) envoyendpoint.LbEndpoint {
	return envoyendpoint.LbEndpoint{
		HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{Endpoint: &envoyendpoint.Endpoint{
			Address: makeAddressPtr(host, port),
		},
		}}
}

func makeLoadAssignment(clusterName string, endpoints structs.CheckServiceNodes, localDatacenter string) *envoy.ClusterLoadAssignment {
	es := make([]envoyendpoint.LbEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		// TODO (mesh-gateway) - should we respect the translate_wan_addrs configuration here or just always use the wan for cross-dc?
		addr, port := ep.BestAddress(localDatacenter != ep.Node.Datacenter)
		healthStatus := envoycore.HealthStatus_HEALTHY
		weight := 1
		if ep.Service.Weights != nil {
			weight = ep.Service.Weights.Passing
		}

		for _, chk := range ep.Checks {
			if chk.Status == api.HealthCritical {
				// This can't actually happen now because health always filters critical
				// but in the future it may not so set this correctly!
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
		es = append(es, envoyendpoint.LbEndpoint{
			HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
				Endpoint: &envoyendpoint.Endpoint{
					Address: makeAddressPtr(addr, port),
				}},
			HealthStatus:        healthStatus,
			LoadBalancingWeight: makeUint32Value(weight),
		})
	}
	return &envoy.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []envoyendpoint.LocalityLbEndpoints{{
			LbEndpoints: es,
		}},
	}
}
