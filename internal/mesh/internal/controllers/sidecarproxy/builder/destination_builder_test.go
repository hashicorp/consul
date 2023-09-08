package builder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
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
)

func TestBuildExplicitDestinations(t *testing.T) {
	api1Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
		WithData(t, endpointsData).WithTenancy(resource.DefaultNamespacedTenancy()).Build()

	api2Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-2").
		WithData(t, endpointsData).WithTenancy(resource.DefaultNamespacedTenancy()).Build()

	api1Identity := &pbresource.Reference{
		Name:    "api1-identity",
		Tenancy: api1Endpoints.Id.Tenancy,
	}

	api2Identity := &pbresource.Reference{
		Name:    "api2-identity",
		Tenancy: api2Endpoints.Id.Tenancy,
	}

	destinationIpPort := &intermediate.Destination{
		Explicit: &pbmesh.Upstream{
			DestinationRef:  resource.Reference(api1Endpoints.Id, ""),
			DestinationPort: "tcp",
			Datacenter:      "dc1",
			ListenAddr: &pbmesh.Upstream_IpPort{
				IpPort: &pbmesh.IPPortAddress{Ip: "1.1.1.1", Port: 1234},
			},
		},
		ServiceEndpoints: &intermediate.ServiceEndpoints{
			Resource:  api1Endpoints,
			Endpoints: endpointsData,
		},
		Identities: []*pbresource.Reference{api1Identity},
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
		ServiceEndpoints: &intermediate.ServiceEndpoints{
			Resource:  api2Endpoints,
			Endpoints: endpointsData,
		},
		Identities: []*pbresource.Reference{api2Identity},
	}

	cases := map[string]struct {
		destinations []*intermediate.Destination
	}{
		"l4-single-destination-ip-port-bind-address": {
			destinations: []*intermediate.Destination{destinationIpPort},
		},
		"l4-single-destination-unix-socket-bind-address": {
			destinations: []*intermediate.Destination{destinationUnix},
		},
		"l4-multi-destination": {
			destinations: []*intermediate.Destination{destinationIpPort, destinationUnix},
		},
	}

	for name, c := range cases {
		proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", nil).
			BuildDestinations(c.destinations).
			Build()

		actual := protoToJSON(t, proxyTmpl)
		expected := golden.Get(t, actual, name)

		require.JSONEq(t, expected, actual)
	}
}

func TestBuildImplicitDestinations(t *testing.T) {
	api1Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
		WithOwner(
			resourcetest.Resource(catalog.ServiceType, "api-1").
				WithTenancy(resource.DefaultNamespacedTenancy()).ID()).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, endpointsData).Build()

	api2Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-2").
		WithOwner(resourcetest.Resource(catalog.ServiceType, "api-2").
			WithTenancy(resource.DefaultNamespacedTenancy()).ID()).
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

	proxyCfg := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
			TransparentProxy: &pbmesh.TransparentProxy{
				OutboundListenerPort: 15001,
			},
		},
	}

	destination1 := &intermediate.Destination{
		ServiceEndpoints: &intermediate.ServiceEndpoints{
			Resource:  api1Endpoints,
			Endpoints: endpointsData,
		},
		Identities: []*pbresource.Reference{api1Identity},
		VirtualIPs: []string{"1.1.1.1"},
	}

	destination2 := &intermediate.Destination{
		ServiceEndpoints: &intermediate.ServiceEndpoints{
			Resource:  api2Endpoints,
			Endpoints: endpointsData,
		},
		Identities: []*pbresource.Reference{api2Identity},
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
		ServiceEndpoints: &intermediate.ServiceEndpoints{
			Resource:  api1Endpoints,
			Endpoints: endpointsData,
		},
		Identities: []*pbresource.Reference{api1Identity},
	}

	cases := map[string]struct {
		destinations []*intermediate.Destination
	}{
		"l4-single-implicit-destination-tproxy": {
			destinations: []*intermediate.Destination{destination1},
		},
		"l4-multiple-implicit-destinations-tproxy": {
			destinations: []*intermediate.Destination{destination1, destination2},
		},
		"l4-implicit-and-explicit-destinations-tproxy": {
			destinations: []*intermediate.Destination{destination2, destination3},
		},
	}

	for name, c := range cases {
		proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", proxyCfg).
			BuildDestinations(c.destinations).
			Build()

		actual := protoToJSON(t, proxyTmpl)
		expected := golden.Get(t, actual, name+".golden")

		require.JSONEq(t, expected, actual)
	}
}
