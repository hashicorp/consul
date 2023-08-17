// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxystateconverter

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func makeLbEndpoint(addr string, port int, health pbproxystate.HealthStatus, weight int) *pbproxystate.Endpoint {
	ep := &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: addr,
				Port: uint32(port),
			},
		},
	}
	ep.HealthStatus = health
	ep.LoadBalancingWeight = &wrapperspb.UInt32Value{Value: uint32(weight)}
	return ep
}

// used in clusters.go
func makeHostPortEndpoint(host string, port int) *pbproxystate.Endpoint {
	return &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: host,
				Port: uint32(port),
			},
		},
	}
}

func makeUnixSocketEndpoint(path string) *pbproxystate.Endpoint {
	return &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_UnixSocket{
			UnixSocket: &pbproxystate.UnixSocketAddress{
				Path: path,
				// envoy's mode is particular to a pipe address and is uint32.
				// it also says "The mode for the Pipe. Not applicable for abstract sockets."
				// https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/core/v3/address.proto#config-core-v3-pipe
				Mode: "0",
			},
		},
	}
}

func calculateEndpointHealthAndWeight(
	ep structs.CheckServiceNode,
	onlyPassing bool,
) (pbproxystate.HealthStatus, int) {
	healthStatus := pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY
	weight := 1
	if ep.Service.Weights != nil {
		weight = ep.Service.Weights.Passing
	}

	for _, chk := range ep.Checks {
		if chk.Status == api.HealthCritical {
			healthStatus = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
		}
		if onlyPassing && chk.Status != api.HealthPassing {
			healthStatus = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
		}
		if chk.Status == api.HealthWarning && ep.Service.Weights != nil {
			weight = ep.Service.Weights.Warning
		}
	}
	// Make weights fit Envoy's limits. A zero weight means that either Warning
	// (likely) or Passing (weirdly) weight has been set to 0 effectively making
	// this instance unhealthy and should not be sent traffic.
	if weight < 1 {
		healthStatus = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
		weight = 1
	}
	if weight > 128 {
		weight = 128
	}
	return healthStatus, weight
}
