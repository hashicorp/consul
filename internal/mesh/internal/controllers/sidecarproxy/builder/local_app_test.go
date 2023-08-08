package builder

import (
	"testing"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
)

func TestBuildLocalApp(t *testing.T) {
	cases := map[string]struct {
		workload *pbcatalog.Workload
	}{
		"l4-single-workload-address-without-ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "10.0.0.1",
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
		"l4-multiple-workload-addresses-without-ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "10.0.0.1",
					},
					{
						Host: "10.0.0.2",
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
		"l4-multiple-workload-addresses-with-specific-ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "127.0.0.1",
						Ports: []string{"port1"},
					},
					{
						Host:  "10.0.0.2",
						Ports: []string{"port2"},
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul").BuildLocalApp(c.workload).
				Build()
			actual := protoToJSON(t, proxyTmpl)
			expected := goldenValue(t, name, actual, *update)

			require.Equal(t, expected, actual)
		})
	}
}

func testProxyStateTemplateID() *pbresource.ID {
	return resourcetest.Resource(types.ProxyStateTemplateType, "test").ID()
}

func testIdentityRef() *pbresource.Reference {
	return &pbresource.Reference{
		Name: "test-identity",
		Tenancy: &pbresource.Tenancy{
			Namespace: "default",
			Partition: "default",
			PeerName:  "local",
		},
	}
}
