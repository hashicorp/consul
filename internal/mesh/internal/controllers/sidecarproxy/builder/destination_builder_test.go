// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/testing/golden"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	endpointsData = &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "10.0.0.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp":  {Port: 7070, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"tcp2": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
	}

	serviceData = &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort:  "tcp",
				VirtualPort: 7070,
				Protocol:    pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort:  "tcp2",
				VirtualPort: 8081,
				Protocol:    pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort:  "http",
				VirtualPort: 8080,
				Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
			},
			{
				TargetPort:  "mesh",
				VirtualPort: 20000,
				Protocol:    pbcatalog.Protocol_PROTOCOL_MESH,
			},
		},
	}
)

func TestBuildExplicitDestinations(t *testing.T) {
	registry := resource.NewRegistry()
	types.Register(registry)
	catalog.RegisterTypes(registry)

	api1Service := resourcetest.Resource(pbcatalog.ServiceType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api2Service := resourcetest.Resource(pbcatalog.ServiceType, "api-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api3Service := resourcetest.Resource(pbcatalog.ServiceType, "api-3").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	backup1Service := resourcetest.Resource(pbcatalog.ServiceType, "backup-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	for _, res := range []*pbresource.Resource{
		api1Service, api2Service, api3Service, backup1Service,
	} {
		resourcetest.ValidateAndNormalize(t, registry, res)
	}

	api1Endpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).
		Build()

	api2Endpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).
		Build()

	backup1Endpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "backup-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).
		Build()

	for _, res := range []*pbresource.Resource{
		api1Endpoints, api2Endpoints, backup1Endpoints,
	} {
		resourcetest.ValidateAndNormalize(t, registry, res)
	}

	api1Identity := &pbresource.Reference{
		Name:    "api1-identity",
		Tenancy: api1Endpoints.Id.Tenancy,
	}

	api2Identity := &pbresource.Reference{
		Name:    "api2-identity",
		Tenancy: api2Endpoints.Id.Tenancy,
	}

	backup1Identity := &pbresource.Reference{
		Name:    "backup1-identity",
		Tenancy: backup1Endpoints.Id.Tenancy,
	}

	api1DestPolicy := resourcetest.Resource(pbmesh.DestinationPolicyType, api1Service.Id.Name).
		WithTenancy(api1Service.Id.GetTenancy()).
		WithData(t, &pbmesh.DestinationPolicy{
			PortConfigs: map[string]*pbmesh.DestinationConfig{
				"http": {
					ConnectTimeout: durationpb.New(55 * time.Second),
					RequestTimeout: durationpb.New(77 * time.Second),
					// LoadBalancer *LoadBalancer `protobuf:"bytes,3,opt,name=load_balancer,json=loadBalancer,proto3" json:"load_balancer,omitempty"`
				},
			},
		}).
		Build()

	api1HTTPRoute := resourcetest.Resource(pbmesh.HTTPRouteType, "api-1-http-route").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{{
				Ref:  resource.Reference(api1Service.Id, ""),
				Port: "http",
			}},
			Rules: []*pbmesh.HTTPRouteRule{
				{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
							Value: "/split",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{
						{
							BackendRef: &pbmesh.BackendReference{
								Ref: resource.Reference(api2Service.Id, ""),
							},
							Weight: 60,
						},
						{
							BackendRef: &pbmesh.BackendReference{
								Ref: resource.Reference(api1Service.Id, ""),
							},
							Weight: 40,
						},
						{
							BackendRef: &pbmesh.BackendReference{
								Ref: resource.Reference(api3Service.Id, ""),
							},
							Weight: 10,
						},
					},
				},
				{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
							Value: "/",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: &pbmesh.BackendReference{
							Ref: resource.Reference(api1Service.Id, ""),
						},
					}},
					Timeouts: &pbmesh.HTTPRouteTimeouts{
						Request: durationpb.New(606 * time.Second), // differnet than the 77s
					},
					Retries: &pbmesh.HTTPRouteRetries{
						Number:           wrapperspb.UInt32(4),
						OnConnectFailure: true,
					},
				},
			},
		}).
		Build()
	resourcetest.ValidateAndNormalize(t, registry, api1HTTPRoute)

	api1FailoverPolicy := resourcetest.Resource(pbcatalog.FailoverPolicyType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.FailoverPolicy{
			PortConfigs: map[string]*pbcatalog.FailoverConfig{
				"http": {
					Destinations: []*pbcatalog.FailoverDestination{{
						Ref:  resource.Reference(backup1Service.Id, ""),
						Port: "http",
					}},
				},
			},
		}).
		Build()
	resourcetest.ValidateAndNormalize(t, registry, api1FailoverPolicy)

	api1TCPRoute := resourcetest.Resource(pbmesh.TCPRouteType, "api-1-tcp-route").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbmesh.TCPRoute{
			ParentRefs: []*pbmesh.ParentReference{{
				Ref:  resource.Reference(api1Service.Id, ""),
				Port: "tcp",
			}},
			Rules: []*pbmesh.TCPRouteRule{{
				BackendRefs: []*pbmesh.TCPBackendRef{
					{
						BackendRef: &pbmesh.BackendReference{
							Ref: resource.Reference(api2Service.Id, ""),
						},
						Weight: 60,
					},
					{
						BackendRef: &pbmesh.BackendReference{
							Ref: resource.Reference(api1Service.Id, ""),
						},
						Weight: 40,
					},
					{
						BackendRef: &pbmesh.BackendReference{
							Ref: resource.Reference(api3Service.Id, ""),
						},
						Weight: 10,
					},
				},
			}},
		}).
		Build()
	resourcetest.ValidateAndNormalize(t, registry, api1TCPRoute)

	api1TCP2Route := resourcetest.Resource(pbmesh.TCPRouteType, "api-1-tcp2-route").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbmesh.TCPRoute{
			ParentRefs: []*pbmesh.ParentReference{{
				Ref:  resource.Reference(api1Service.Id, ""),
				Port: "tcp2",
			}},
			Rules: []*pbmesh.TCPRouteRule{{
				BackendRefs: []*pbmesh.TCPBackendRef{
					{
						BackendRef: &pbmesh.BackendReference{
							Ref: resource.Reference(api2Service.Id, ""),
						},
						Weight: 60,
					},
					{
						BackendRef: &pbmesh.BackendReference{
							Ref: resource.Reference(api1Service.Id, ""),
						},
						Weight: 40,
					},
					{
						BackendRef: &pbmesh.BackendReference{
							Ref: resource.Reference(api3Service.Id, ""),
						},
						Weight: 10,
					},
				},
			}},
		}).
		Build()

	api1ComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, api1Service.Id)
	api1ComputedRoutes := routestest.BuildComputedRoutes(t, api1ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
		resourcetest.MustDecode[*pbcatalog.Service](t, backup1Service),
		// notably we do NOT include api3Service here so we trigger a null route to be generated
		resourcetest.MustDecode[*pbmesh.DestinationPolicy](t, api1DestPolicy),
		resourcetest.MustDecode[*pbmesh.HTTPRoute](t, api1HTTPRoute),
		resourcetest.MustDecode[*pbmesh.TCPRoute](t, api1TCPRoute),
		resourcetest.MustDecode[*pbcatalog.FailoverPolicy](t, api1FailoverPolicy),
		resourcetest.MustDecode[*pbmesh.TCPRoute](t, api1TCP2Route),
	)
	require.NotNil(t, api1ComputedRoutes)

	api2ComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, api2Service.Id)
	api2ComputedRoutes := routestest.BuildComputedRoutes(t, api2ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
	)
	require.NotNil(t, api2ComputedRoutes)

	destinationIpPort := &intermediate.Destination{
		Explicit: &pbmesh.Destination{
			DestinationRef:  resource.Reference(api1Endpoints.Id, ""),
			DestinationPort: "tcp",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Destination_IpPort{
				IpPort: &pbmesh.IPPortAddress{Ip: "1.1.1.1", Port: 1234},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		ComputedPortRoutes: routestest.MutateTargets(t, api1ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
			switch {
			case resource.ReferenceOrIDMatch(api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
				details.ServiceEndpointsId = api1Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api1Identity}
			case resource.ReferenceOrIDMatch(api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
				details.ServiceEndpointsId = api2Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api2Identity}
			}
		}),
	}

	destinationIpPort2 := &intermediate.Destination{
		Explicit: &pbmesh.Destination{
			DestinationRef:  resource.Reference(api1Endpoints.Id, ""),
			DestinationPort: "tcp2",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Destination_IpPort{
				IpPort: &pbmesh.IPPortAddress{Ip: "1.1.1.1", Port: 2345},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		ComputedPortRoutes: routestest.MutateTargets(t, api1ComputedRoutes.Data, "tcp2", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
			switch {
			case resource.ReferenceOrIDMatch(api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp2":
				details.ServiceEndpointsId = api1Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api1Identity}
			case resource.ReferenceOrIDMatch(api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp2":
				details.ServiceEndpointsId = api2Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api2Identity}
			}
		}),
	}

	destinationUnix := &intermediate.Destination{
		Explicit: &pbmesh.Destination{
			DestinationRef:  resource.Reference(api2Endpoints.Id, ""),
			DestinationPort: "tcp",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Destination_Unix{
				Unix: &pbmesh.UnixSocketAddress{Path: "/path/to/socket", Mode: "0666"},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
		ComputedPortRoutes: routestest.MutateTargets(t, api2ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
			switch {
			case resource.ReferenceOrIDMatch(api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
				details.ServiceEndpointsId = api2Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api2Identity}
			}
		}),
	}

	destinationUnix2 := &intermediate.Destination{
		Explicit: &pbmesh.Destination{
			DestinationRef:  resource.Reference(api2Endpoints.Id, ""),
			DestinationPort: "tcp2",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Destination_Unix{
				Unix: &pbmesh.UnixSocketAddress{Path: "/path/to/socket", Mode: "0666"},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
		ComputedPortRoutes: routestest.MutateTargets(t, api2ComputedRoutes.Data, "tcp2", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
			switch {
			case resource.ReferenceOrIDMatch(api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp2":
				details.ServiceEndpointsId = api2Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api2Identity}
			}
		}),
	}
	destinationIpPortHTTP := &intermediate.Destination{
		Explicit: &pbmesh.Destination{
			DestinationRef:  resource.Reference(api1Endpoints.Id, ""),
			DestinationPort: "http",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Destination_IpPort{
				IpPort: &pbmesh.IPPortAddress{Ip: "1.1.1.1", Port: 1234},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		ComputedPortRoutes: routestest.MutateTargets(t, api1ComputedRoutes.Data, "http", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
			switch {
			case resource.ReferenceOrIDMatch(api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "http":
				details.ServiceEndpointsId = api1Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api1Identity}
			case resource.ReferenceOrIDMatch(api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "http":
				details.ServiceEndpointsId = api2Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api2Identity}
			case resource.ReferenceOrIDMatch(backup1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "http":
				details.ServiceEndpointsId = backup1Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{backup1Identity}
			}
		}),
	}
	_ = backup1Identity

	cases := map[string]struct {
		destinations []*intermediate.Destination
	}{
		"destination/l4-single-destination-ip-port-bind-address": {
			destinations: []*intermediate.Destination{destinationIpPort},
		},
		"destination/l4-single-destination-unix-socket-bind-address": {
			destinations: []*intermediate.Destination{destinationUnix},
		},
		"destination/l4-multi-destination": {
			destinations: []*intermediate.Destination{destinationIpPort, destinationUnix, destinationIpPort2, destinationUnix2},
		},
		"destination/mixed-multi-destination": {
			destinations: []*intermediate.Destination{destinationIpPort, destinationUnix, destinationIpPortHTTP},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", false, nil).
				BuildDestinations(c.destinations).
				Build()

			actual := protoToJSON(t, proxyTmpl)
			expected := golden.Get(t, actual, name+".golden")

			require.JSONEq(t, expected, actual)
		})
	}
}

func TestBuildImplicitDestinations(t *testing.T) {
	api1Service := resourcetest.Resource(pbcatalog.ServiceType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api2Service := resourcetest.Resource(pbcatalog.ServiceType, "api-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api1Endpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-1").
		WithOwner(api1Service.Id).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).Build()

	api2Endpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-2").
		WithOwner(api2Service.Id).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).Build()

	api1Identity := &pbresource.Reference{
		Name:    "api1-identity",
		Tenancy: api1Endpoints.Id.Tenancy,
	}

	api2Identity := &pbresource.Reference{
		Name:    "api2-identity",
		Tenancy: api2Endpoints.Id.Tenancy,
	}

	api1ComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, api1Service.Id)
	api1ComputedRoutes := routestest.BuildComputedRoutes(t, api1ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
	)
	require.NotNil(t, api1ComputedRoutes)

	api2ComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, api2Service.Id)
	api2ComputedRoutes := routestest.BuildComputedRoutes(t, api2ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
	)
	require.NotNil(t, api2ComputedRoutes)

	proxyCfg := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
			TransparentProxy: &pbmesh.TransparentProxy{
				OutboundListenerPort: 15001,
			},
		},
	}

	destination1 := &intermediate.Destination{
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		ComputedPortRoutes: routestest.MutateTargets(t, api1ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
			switch {
			case resource.ReferenceOrIDMatch(api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
				details.ServiceEndpointsId = api1Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api1Identity}
			}
		}),
		VirtualIPs: []string{"1.1.1.1"},
	}

	destination2 := &intermediate.Destination{
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
		ComputedPortRoutes: routestest.MutateTargets(t, api2ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
			switch {
			case resource.ReferenceOrIDMatch(api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
				details.ServiceEndpointsId = api2Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api2Identity}
			}
		}),
		VirtualIPs: []string{"2.2.2.2", "3.3.3.3"},
	}

	destination3 := &intermediate.Destination{
		Explicit: &pbmesh.Destination{
			DestinationRef:  resource.Reference(api1Endpoints.Id, ""),
			DestinationPort: "tcp",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Destination_IpPort{
				IpPort: &pbmesh.IPPortAddress{Ip: "1.1.1.1", Port: 1234},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		ComputedPortRoutes: routestest.MutateTargets(t, api1ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
			switch {
			case resource.ReferenceOrIDMatch(api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
				details.ServiceEndpointsId = api1Endpoints.Id
				details.ServiceEndpoints = endpointsData
				details.IdentityRefs = []*pbresource.Reference{api1Identity}
			}
		}),
	}

	cases := map[string]struct {
		destinations []*intermediate.Destination
	}{
		"destination/l4-single-implicit-destination-tproxy": {
			destinations: []*intermediate.Destination{destination1},
		},
		"destination/l4-multiple-implicit-destinations-tproxy": {
			destinations: []*intermediate.Destination{destination1, destination2},
		},
		"destination/l4-implicit-and-explicit-destinations-tproxy": {
			destinations: []*intermediate.Destination{destination2, destination3},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", false, proxyCfg).
				BuildDestinations(c.destinations).
				Build()

			actual := protoToJSON(t, proxyTmpl)
			expected := golden.Get(t, actual, name+".golden")

			require.JSONEq(t, expected, actual)
		})
	}
}
