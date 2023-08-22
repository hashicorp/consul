package xds

import (
	"fmt"
	"net"

	"golang.org/x/exp/slices"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

func generateProxyStateEndpoints(serviceEndpoints *ServiceEndpointsData, portName string) (*pbproxystate.Endpoints, error) {
	var psEndpoints []*pbproxystate.Endpoint

	if serviceEndpoints.Endpoints == nil || serviceEndpoints.Resource == nil {
		return nil, fmt.Errorf("service endpoints requires both endpoints and resource")
	}
	eps := serviceEndpoints.Endpoints.GetEndpoints()

	for _, ep := range eps {
		for _, addr := range ep.Addresses {
			// Check if the address is using the portName name this proxy state endpoints is for. If it does, create the
			// endpoint.
			if slices.Contains(addr.Ports, portName) {
				// Lookup the portName number from the portName name.
				wlPort, ok := ep.Ports[portName]
				if !ok {
					// This should never happen, as it should be validated by the ServiceEndpoints controller.
					return nil, fmt.Errorf("could not find portName %q in endpoint %s", portName, serviceEndpoints.Resource.Id)
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
