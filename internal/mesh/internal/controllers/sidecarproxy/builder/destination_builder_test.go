// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/testing/golden"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
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
					"tcp":  {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
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
				VirtualPort: 8080,
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
	api1Service := resourcetest.Resource(catalog.ServiceType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api2Service := resourcetest.Resource(catalog.ServiceType, "api-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api3Service := resourcetest.Resource(catalog.ServiceType, "api-3").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api1Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).
		Build()

	api2Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).
		Build()

	api1Identity := &pbresource.Reference{
		Name:    "api1-identity",
		Tenancy: api1Endpoints.Id.Tenancy,
	}

	api2Identity := &pbresource.Reference{
		Name:    "api2-identity",
		Tenancy: api2Endpoints.Id.Tenancy,
	}

	api1HTTPRoute := resourcetest.Resource(types.HTTPRouteType, "api-1-http-route").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{{
				Ref:  resource.Reference(api1Service.Id, ""),
				Port: "http",
			}},
			Rules: []*pbmesh.HTTPRouteRule{{
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
			}},
		}).
		Build()

	api1TCPRoute := resourcetest.Resource(types.TCPRouteType, "api-1-tcp-route").
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

	api1ComputedRoutesID := resource.ReplaceType(types.ComputedRoutesType, api1Service.Id)
	api1ComputedRoutes := routestest.BuildComputedRoutes(t, api1ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
		// notably we do NOT include api3Service here so we trigger a null route to be generated
		resourcetest.MustDecode[*pbmesh.HTTPRoute](t, api1HTTPRoute),
		resourcetest.MustDecode[*pbmesh.TCPRoute](t, api1TCPRoute),
	)
	require.NotNil(t, api1ComputedRoutes)

	api2ComputedRoutesID := resource.ReplaceType(types.ComputedRoutesType, api2Service.Id)
	api2ComputedRoutes := routestest.BuildComputedRoutes(t, api2ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
	)
	require.NotNil(t, api2ComputedRoutes)

	destinationIpPort := &intermediate.Destination{
		Explicit: &pbmesh.Upstream{
			DestinationRef:  resource.Reference(api1Endpoints.Id, ""),
			DestinationPort: "tcp",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Upstream_IpPort{
				IpPort: &pbmesh.IPPortAddress{Ip: "1.1.1.1", Port: 1234},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		ComputedPortRoutes: routestest.MutateTarget(t, api1ComputedRoutes.Data.PortedConfigs["tcp"], api1Service.Id, "tcp", func(details *pbmesh.BackendTargetDetails) {
			details.ServiceEndpointsId = api1Endpoints.Id
			details.ServiceEndpoints = endpointsData
			details.IdentityRefs = []*pbresource.Reference{api1Identity}
		}),
	}

	destinationUnix := &intermediate.Destination{
		Explicit: &pbmesh.Upstream{
			DestinationRef:  resource.Reference(api2Endpoints.Id, ""),
			DestinationPort: "tcp",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Upstream_Unix{
				Unix: &pbmesh.UnixSocketAddress{Path: "/path/to/socket", Mode: "0666"},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
		ComputedPortRoutes: routestest.MutateTarget(t, api2ComputedRoutes.Data.PortedConfigs["tcp"], api2Service.Id, "tcp", func(details *pbmesh.BackendTargetDetails) {
			details.ServiceEndpointsId = api2Endpoints.Id
			details.ServiceEndpoints = endpointsData
			details.IdentityRefs = []*pbresource.Reference{api2Identity}
		}),
	}

	destinationIpPortHTTP := &intermediate.Destination{
		Explicit: &pbmesh.Upstream{
			DestinationRef:  resource.Reference(api1Endpoints.Id, ""),
			DestinationPort: "http",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Upstream_IpPort{
				IpPort: &pbmesh.IPPortAddress{Ip: "1.1.1.1", Port: 1234},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		ComputedPortRoutes: routestest.MutateTarget(t, api1ComputedRoutes.Data.PortedConfigs["http"], api1Service.Id, "http", func(details *pbmesh.BackendTargetDetails) {
			details.ServiceEndpointsId = api1Endpoints.Id
			details.ServiceEndpoints = endpointsData
			details.IdentityRefs = []*pbresource.Reference{api1Identity}
		}),
	}

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
			destinations: []*intermediate.Destination{destinationIpPort, destinationUnix},
		},
		"destination/mixed-multi-destination": {
			destinations: []*intermediate.Destination{destinationIpPort, destinationUnix, destinationIpPortHTTP},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", nil).
				BuildDestinations(c.destinations).
				Build()

			actual := protoToJSON(t, proxyTmpl)
			expected := golden.Get(t, actual, name+".golden")

			require.JSONEq(t, expected, actual)
		})
	}
}

func TestBuildImplicitDestinations(t *testing.T) {
	api1Service := resourcetest.Resource(catalog.ServiceType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api2Service := resourcetest.Resource(catalog.ServiceType, "api-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	api1Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
		WithOwner(api1Service.Id).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).Build()

	api2Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-2").
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

	api1ComputedRoutesID := resource.ReplaceType(types.ComputedRoutesType, api1Service.Id)
	api1ComputedRoutes := routestest.BuildComputedRoutes(t, api1ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
	)
	require.NotNil(t, api1ComputedRoutes)

	api2ComputedRoutesID := resource.ReplaceType(types.ComputedRoutesType, api2Service.Id)
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
		ComputedPortRoutes: routestest.MutateTarget(t, api1ComputedRoutes.Data.PortedConfigs["tcp"], api1Service.Id, "tcp", func(details *pbmesh.BackendTargetDetails) {
			details.ServiceEndpointsId = api1Endpoints.Id
			details.ServiceEndpoints = endpointsData
			details.IdentityRefs = []*pbresource.Reference{api1Identity}
		}),
		VirtualIPs: []string{"1.1.1.1"},
	}

	destination2 := &intermediate.Destination{
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api2Service),
		ComputedPortRoutes: routestest.MutateTarget(t, api2ComputedRoutes.Data.PortedConfigs["tcp"], api2Service.Id, "tcp", func(details *pbmesh.BackendTargetDetails) {
			details.ServiceEndpointsId = api2Endpoints.Id
			details.ServiceEndpoints = endpointsData
			details.IdentityRefs = []*pbresource.Reference{api2Identity}
		}),
		VirtualIPs: []string{"2.2.2.2", "3.3.3.3"},
	}

	destination3 := &intermediate.Destination{
		Explicit: &pbmesh.Upstream{
			DestinationRef:  resource.Reference(api1Endpoints.Id, ""),
			DestinationPort: "tcp",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Upstream_IpPort{
				IpPort: &pbmesh.IPPortAddress{Ip: "1.1.1.1", Port: 1234},
			},
		},
		Service: resourcetest.MustDecode[*pbcatalog.Service](t, api1Service),
		ComputedPortRoutes: routestest.MutateTarget(t, api1ComputedRoutes.Data.PortedConfigs["tcp"], api1Service.Id, "tcp", func(details *pbmesh.BackendTargetDetails) {
			details.ServiceEndpointsId = api1Endpoints.Id
			details.ServiceEndpoints = endpointsData
			details.IdentityRefs = []*pbresource.Reference{api1Identity}
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
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", proxyCfg).
				BuildDestinations(c.destinations).
				Build()

			actual := protoToJSON(t, proxyTmpl)
			expected := golden.Get(t, actual, name+".golden")

			require.JSONEq(t, expected, actual)
		})
	}
}
