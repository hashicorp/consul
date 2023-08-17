// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"google.golang.org/protobuf/proto"
)

func makeEnvoyLbEndpoint(endpoint *pbproxystate.Endpoint) *envoy_endpoint_v3.LbEndpoint {
	hs := int32(endpoint.GetHealthStatus().Number())
	return &envoy_endpoint_v3.LbEndpoint{
		HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
			Endpoint: makeEnvoyEndpoint(endpoint),
		},
		HealthStatus:        envoy_core_v3.HealthStatus(hs),
		LoadBalancingWeight: endpoint.GetLoadBalancingWeight(),
	}
}

func makeEnvoyEndpoint(endpoint *pbproxystate.Endpoint) *envoy_endpoint_v3.Endpoint {
	var address *envoy_core_v3.Address
	if endpoint.GetUnixSocket() != nil {
		address = response.MakePipeAddress(endpoint.GetUnixSocket().GetPath(), 0)
	} else {
		address = response.MakeAddress(endpoint.GetHostPort().GetHost(), int(endpoint.GetHostPort().Port))
	}

	return &envoy_endpoint_v3.Endpoint{
		Address: address,
	}
}

func makeEnvoyClusterLoadAssignment(clusterName string, endpoints []*pbproxystate.Endpoint) *envoy_endpoint_v3.ClusterLoadAssignment {
	localityLbEndpoints := &envoy_endpoint_v3.LocalityLbEndpoints{}
	for _, endpoint := range endpoints {
		localityLbEndpoints.LbEndpoints = append(localityLbEndpoints.LbEndpoints, makeEnvoyLbEndpoint(endpoint))
	}
	return &envoy_endpoint_v3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   []*envoy_endpoint_v3.LocalityLbEndpoints{localityLbEndpoints},
	}
}

func (pr *ProxyResources) makeXDSEndpoints() ([]proto.Message, error) {
	endpoints := make([]proto.Message, 0)

	for clusterName, eps := range pr.proxyState.GetEndpoints() {
		// TODO(jm):  this does not seem like the best way.
		if clusterName != xdscommon.LocalAppClusterName {
			protoEndpoint := makeEnvoyClusterLoadAssignment(clusterName, eps.Endpoints)
			endpoints = append(endpoints, protoEndpoint)
		}
	}

	return endpoints, nil
}
