// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMakeProxyStateEndpointsFromServiceEndpoints(t *testing.T) {
	type test struct {
		name                        string
		serviceEndpointsData        *ServiceEndpointsData
		meshPortName                string
		routePortName               string
		expErr                      string
		expectedProxyStateEndpoints *pbproxystate.Endpoints
	}
	cases := []test{
		{
			name:                 "endpoints with passing health",
			serviceEndpointsData: serviceEndpointsData("passing"),
			meshPortName:         "mesh",
			routePortName:        "api",
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
			name:                 "endpoints with passing health for admin port only",
			serviceEndpointsData: serviceEndpointsData("passing"),
			meshPortName:         "mesh",
			routePortName:        "admin",
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
				},
			},
		},
		{
			name:                 "endpoints with critical health",
			serviceEndpointsData: serviceEndpointsData("critical"),
			meshPortName:         "mesh",
			routePortName:        "api",
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
			name:                 "endpoints with critical health for admin port only",
			serviceEndpointsData: serviceEndpointsData("critical"),
			meshPortName:         "mesh",
			routePortName:        "admin",
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
				},
			},
		},
		{
			name:                 "endpoints with any health are considered healthy",
			serviceEndpointsData: serviceEndpointsData("any"),
			meshPortName:         "mesh",
			routePortName:        "api",
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
			name:                 "endpoints with any health are considered healthy for admin port only",
			serviceEndpointsData: serviceEndpointsData("any"),
			meshPortName:         "mesh",
			routePortName:        "admin",
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
				},
			},
		},
		{
			name:                 "endpoints with missing ports returns an error",
			serviceEndpointsData: serviceEndpointsData("missing port lookup"),
			meshPortName:         "mesh",
			routePortName:        "api",
			expErr:               "could not find meshPort",
		},
		{
			name:                 "nil endpoints returns an error",
			serviceEndpointsData: serviceEndpointsData("nil endpoints"),
			meshPortName:         "mesh",
			routePortName:        "api",
			expErr:               "service endpoints requires both endpoints and resource",
		},
		{
			name:                 "nil resource returns an error",
			serviceEndpointsData: serviceEndpointsData("nil resource"),
			meshPortName:         "mesh",
			routePortName:        "api",
			expErr:               "service endpoints requires both endpoints and resource",
		},
		{
			name:                 "meshPortName doesn't exist in endpoints results in empty endpoints",
			serviceEndpointsData: serviceEndpointsData("passing"),
			meshPortName:         "does-not-exist",
			routePortName:        "api",
			expectedProxyStateEndpoints: &pbproxystate.Endpoints{
				Endpoints: []*pbproxystate.Endpoint{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actualEndpoints, err := generateProxyStateEndpoints(tc.serviceEndpointsData, tc.routePortName, tc.meshPortName)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			} else {
				prototest.AssertDeepEqual(t, tc.expectedProxyStateEndpoints, actualEndpoints)
			}
		})
	}
}

// serviceEndpointsData returns a service endpoints where all addresses
// implement the mesh and api ports, and only 2 of the addresses implement the
// admin port.
func serviceEndpointsData(variation string) *ServiceEndpointsData {
	r := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "test").Build()
	eps := &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
					"admin": {
						Port:     1234,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
					"api": {
						Port:     2234,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.1.1.1",
						Ports: []string{"mesh", "admin", "api"},
					},
					{
						Host:  "10.2.2.2",
						Ports: []string{"mesh", "admin", "api"},
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
					"admin": {
						Port:     1234,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
					"api": {
						Port:     2234,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "10.3.3.3",
						// does not implement admin port
						Ports: []string{"mesh", "api"},
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
