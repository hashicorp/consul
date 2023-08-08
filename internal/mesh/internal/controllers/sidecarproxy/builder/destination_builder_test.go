package builder

import (
	"testing"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
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
				},
			},
		},
	}
)

func TestBuildExplicitDestinations(t *testing.T) {
	api1Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
		WithData(t, endpointsData).Build()

	api2Endpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-2").
		WithData(t, endpointsData).Build()

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
		proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul").
			BuildDestinations(c.destinations).
			Build()

		actual := protoToJSON(t, proxyTmpl)
		expected := goldenValue(t, name, actual, *update)

		require.Equal(t, expected, actual)
	}

}
