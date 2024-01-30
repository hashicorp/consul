// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"net"

	"golang.org/x/exp/slices"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
)

func generateProxyStateEndpoints(serviceEndpoints *ServiceEndpointsData, routePort string, meshPort string) (*pbproxystate.Endpoints, error) {
	var psEndpoints []*pbproxystate.Endpoint

	if serviceEndpoints.Endpoints == nil || serviceEndpoints.Resource == nil {
		return nil, fmt.Errorf("service endpoints requires both endpoints and resource")
	}
	eps := serviceEndpoints.Endpoints.GetEndpoints()

	for _, ep := range eps {
		for _, addr := range ep.Addresses {
			// Each address on the endpoint has all of the ports that the address exposes. To route to an endpoint with
			// an address + port, that address will need to contain the port you are routing to and the mesh port. Only
			// if both are present, should the endpoint be added to the ProxyState endpoints. If the address contains
			// both ports, then it should be added to ProxyState endpoints with the mesh port. The route port will be
			// added by the Envoy cluster configuration in the alpn.
			if slices.Contains(addr.Ports, routePort) && slices.Contains(addr.Ports, meshPort) {
				// Lookup the mesh port number from the mesh port name.
				wlPort, ok := ep.Ports[meshPort]
				if !ok {
					// This should never happen, as it should be validated by the ServiceEndpoints controller.
					return nil, fmt.Errorf("could not find meshPort %q in endpoint %s", meshPort, serviceEndpoints.Resource.Id)
				}
				portNum := wlPort.Port

				psEndpoint, err := createProxyStateEndpoint(addr.Host, portNum, ep.HealthStatus)
				if err != nil {
					return nil, err
				}
				psEndpoints = append(psEndpoints, psEndpoint)
			}
		}
	}

	return &pbproxystate.Endpoints{Endpoints: psEndpoints}, nil
}

func createProxyStateEndpoint(host string, port uint32, health pbcatalog.Health) (*pbproxystate.Endpoint, error) {
	addr := net.ParseIP(host)
	if addr == nil {
		return nil, fmt.Errorf("host is not an ip")
	}

	psEndpoint := &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: host,
				Port: port,
			},
		},
		HealthStatus: endpointHealth(health),
		// TODO(xds): Weight will be added later. More information is potentially needed in the reference.
	}
	return psEndpoint, nil
}

func endpointHealth(catalogHealth pbcatalog.Health) pbproxystate.HealthStatus {
	health := pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY

	if catalogHealth == pbcatalog.Health_HEALTH_CRITICAL {
		health = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
	}
	return health
}
