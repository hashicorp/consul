package xds

import (
	"errors"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// endpointsFromSnapshot returns the xDS API reprepsentation of the "endpoints"
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

func makeEndpoint(clusterName, host string, port int) endpoint.LbEndpoint {
	return endpoint.LbEndpoint{
		Endpoint: &endpoint.Endpoint{
			Address: makeAddressPtr(host, port),
		},
	}
}

func makeLoadAssignment(clusterName string, endpoints structs.CheckServiceNodes) *v2.ClusterLoadAssignment {
	es := make([]endpoint.LbEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		addr := ep.Service.Address
		if addr == "" {
			addr = ep.Node.Address
		}
		es = append(es, endpoint.LbEndpoint{
			Endpoint: &endpoint.Endpoint{
				Address: makeAddressPtr(addr, ep.Service.Port),
			},
		})
	}
	return &v2.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []endpoint.LocalityLbEndpoints{{
			LbEndpoints: es,
		}},
	}
}
