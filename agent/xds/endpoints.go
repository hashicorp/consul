package xds

import (
	"errors"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// endpointsFromSnapshot returns the xDS API representation of the "endpoints"
// (upstream instances) in the snapshot.
func endpointsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}
	resources := make([]proto.Message, 0, len(cfgSnap.UpstreamEndpoints))
	for id, endpoints := range cfgSnap.UpstreamEndpoints {
		la := makeLoadAssignment(id, endpoints)
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

func makeLoadAssignment(clusterName string, endpoints structs.CheckServiceNodes) *envoy.ClusterLoadAssignment {
	es := make([]envoyendpoint.LbEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		addr := ep.Service.Address
		if addr == "" {
			addr = ep.Node.Address
		}
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
					Address: makeAddressPtr(addr, ep.Service.Port),
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
