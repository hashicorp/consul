package xds

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMakeProxyStateEndpointsFromServiceEndpoints(t *testing.T) {
	type test struct {
		name                        string
		serviceEndpointsData        *ServiceEndpointsData
		portName                    string
		expErr                      string
		expectedProxyStateEndpoints *pbproxystate.Endpoints
	}
	cases := []test{
		{
			name:                 "endpoints with passing health",
			serviceEndpointsData: serviceEndpointsData("passing"),
			portName:             "mesh",
			expectedProxyStateEndpoints: &pbproxystate.Endpoints{
				Endpoints: []*pbproxystate.Endpoint{
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.1.1.1",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
					},
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.2.2.2",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
					},
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.3.3.3",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
					},
				},
			},
		},
		{
			name:                 "endpoints with critical health",
			serviceEndpointsData: serviceEndpointsData("critical"),
			portName:             "mesh",
			expectedProxyStateEndpoints: &pbproxystate.Endpoints{
				Endpoints: []*pbproxystate.Endpoint{
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.1.1.1",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY,
					},
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.2.2.2",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY,
					},
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.3.3.3",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY,
					},
				},
			},
		},
		{
			name:                 "endpoints with any health are considered healthy",
			serviceEndpointsData: serviceEndpointsData("any"),
			portName:             "mesh",
			expectedProxyStateEndpoints: &pbproxystate.Endpoints{
				Endpoints: []*pbproxystate.Endpoint{
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.1.1.1",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
					},
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.2.2.2",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
					},
					{
						Address: &pbproxystate.Endpoint_HostPort{
							HostPort: &pbproxystate.HostPortAddress{
								Host: "10.3.3.3",
								Port: 20000,
							},
						},
						HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
					},
				},
			},
		},
		{
			name:                 "endpoints with missing ports returns an error",
			serviceEndpointsData: serviceEndpointsData("missing port lookup"),
			portName:             "mesh",
			expErr:               "could not find portName",
		},
		{
			name:                 "nil endpoints returns an error",
			serviceEndpointsData: serviceEndpointsData("nil endpoints"),
			portName:             "mesh",
			expErr:               "service endpoints requires both endpoints and resource",
		},
		{
			name:                 "nil resource returns an error",
			serviceEndpointsData: serviceEndpointsData("nil resource"),
			portName:             "mesh",
			expErr:               "service endpoints requires both endpoints and resource",
		},
		{
			name:                 "portName doesn't exist in endpoints results in empty endpoints",
			serviceEndpointsData: serviceEndpointsData("passing"),
			portName:             "does-not-exist",
			expectedProxyStateEndpoints: &pbproxystate.Endpoints{
				Endpoints: []*pbproxystate.Endpoint{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actualEndpoints, err := generateProxyStateEndpoints(tc.serviceEndpointsData, tc.portName)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			} else {
				prototest.AssertDeepEqual(t, tc.expectedProxyStateEndpoints, actualEndpoints)
			}
		})
	}
}

func serviceEndpointsData(variation string) *ServiceEndpointsData {
	r := resourcetest.Resource(catalog.ServiceEndpointsType, "test").Build()
	eps := &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.1.1.1",
						Ports: []string{"mesh"},
					},
					{
						Host:  "10.2.2.2",
						Ports: []string{"mesh"},
					},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.3.3.3",
						Ports: []string{"mesh"},
					},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
		},
	}

	switch variation {
	case "passing":
	case "critical":
		eps.Endpoints[0].HealthStatus = pbcatalog.Health_HEALTH_CRITICAL
		eps.Endpoints[1].HealthStatus = pbcatalog.Health_HEALTH_CRITICAL
	case "any":
		eps.Endpoints[0].HealthStatus = pbcatalog.Health_HEALTH_ANY
		eps.Endpoints[1].HealthStatus = pbcatalog.Health_HEALTH_ANY
	case "missing port lookup":
		delete(eps.Endpoints[0].Ports, "mesh")
	case "nil endpoints":
		eps = nil
	case "nil resource":
		r = nil
	}

	return &ServiceEndpointsData{
		Resource:  r,
		Endpoints: eps,
	}
}
