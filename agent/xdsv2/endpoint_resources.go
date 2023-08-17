// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

func makeEnvoyEndpoint(endpoint *pbproxystate.Endpoint) *envoy_endpoint_v3.LbEndpoint {
	var address *envoy_core_v3.Address
	if endpoint.GetUnixSocket() != nil {
		address = response.MakePipeAddress(endpoint.GetUnixSocket().GetPath(), 0)
	} else {
		address = response.MakeAddress(endpoint.GetHostPort().GetHost(), int(endpoint.GetHostPort().Port))
	}
	return &envoy_endpoint_v3.LbEndpoint{
		HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
			Endpoint: &envoy_endpoint_v3.Endpoint{
				Address: address,
			},
		},
	}
}

func makeEnvoyClusterLoadAssignment(clusterName string, endpoints []*pbproxystate.Endpoint) *envoy_endpoint_v3.ClusterLoadAssignment {
	localityLbEndpoints := make([]*envoy_endpoint_v3.LocalityLbEndpoints, 0, len(endpoints))
	for _, endpoint := range endpoints {
		localityLbEndpoints = append(localityLbEndpoints, &envoy_endpoint_v3.LocalityLbEndpoints{
			LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{makeEnvoyEndpoint(endpoint)},
		})
	}
	return &envoy_endpoint_v3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   localityLbEndpoints,
	}

}
