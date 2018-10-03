package xds

import (
	"errors"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// endpointsFromSnapshot returns the xDS API representation of the "endpoints"
// (upstream instances) in the snapshot.
func endpointsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}
	resources := make([]proto.Message, 0, len(cfgSnap.UpstreamEndpoints))
	for id, endpoints := range cfgSnap.UpstreamEndpoints {
		if len(endpoints) < 1 {
			continue
		}
		la := makeLoadAssignment(id, endpoints)
		resources = append(resources, la)
	}
	return resources, nil
}

func makeEndpoint(clusterName, host string, port int) envoyendpoint.LbEndpoint {
	return envoyendpoint.LbEndpoint{
		Endpoint: &envoyendpoint.Endpoint{
			Address: makeAddressPtr(host, port),
		},
	}
}

func makeLoadAssignment(clusterName string, endpoints structs.CheckServiceNodes) *envoy.ClusterLoadAssignment {
	es := make([]envoyendpoint.LbEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		addr := ep.Service.Address
		if addr == "" {
			addr = ep.Node.Address
		}
		es = append(es, envoyendpoint.LbEndpoint{
			Endpoint: &envoyendpoint.Endpoint{
				Address: makeAddressPtr(addr, ep.Service.Port),
			},
		})
	}
	return &envoy.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []envoyendpoint.LocalityLbEndpoints{{
			LbEndpoints: es,
		}},
	}
}
