package builder

import (
	"testing"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/stretchr/testify/require"
)

func Test_newServicePortInfo(t *testing.T) {
	cases := map[string]struct {
		serviceEndpoints *pbcatalog.ServiceEndpoints
		expectedResult   *servicePortInfo
	}{
		"address specific ports union between endpoints": {
			// this test case shows endpoints 1 and endpoints 2 each having an effective port of api-port
			// which leads to the service having an effective port of api-port
			serviceEndpoints: &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					// effective ports = api-port
					{
						Addresses: []*pbcatalog.WorkloadAddress{
							{Host: "10.0.0.1", Ports: []string{"api-port"}},
						},
						Ports: map[string]*pbcatalog.WorkloadPort{
							"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
						},
					},
					// effective ports = api-port
					{
						Addresses: []*pbcatalog.WorkloadAddress{
							{Host: "10.0.0.2", Ports: []string{"api-port"}},
						},
						Ports: map[string]*pbcatalog.WorkloadPort{
							"api-port": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"mesh":     {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
						},
					},
				},
			},

			// cumulative effective ports = api-port
			expectedResult: &servicePortInfo{
				meshPortName: "mesh",
				meshPort:     &pbcatalog.WorkloadPort{Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				servicePorts: map[string]*pbcatalog.WorkloadPort{
					"api-port": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				},
			},
		},
		"address specific ports union with ports aggregated across workload addresses": {
			// this test case shows:
			// - endpoint 1 has two address with specific ports and the effective ports are the combination of the two,
			// not the union and also excludes a endpoint port that is not exposed on any address.
			// - endpoint 2 has an address with no specific ports, so its effective ports are all non-"mesh" ports
			// defined on the endpoint.
			// - the cumulative effect is then the union of endpoints 1 and 2.
			serviceEndpoints: &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					// effective ports = api-port, admin-port
					{
						Addresses: []*pbcatalog.WorkloadAddress{
							{Host: "10.0.0.1", Ports: []string{"api-port"}},
							{Host: "10.0.0.2", Ports: []string{"admin-port"}},
						},
						Ports: map[string]*pbcatalog.WorkloadPort{
							"some-port":  {Port: 6060, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
						},
					},
					// effective ports = api-port, admin-port, another-port
					{
						Addresses: []*pbcatalog.WorkloadAddress{
							{Host: "10.0.0.3"},
						},
						Ports: map[string]*pbcatalog.WorkloadPort{
							"another-port": {Port: 7070, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"admin-port":   {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"api-port":     {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"mesh":         {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
						},
					},
				},
			},
			// cumulative effective ports = admin-port, api-port
			expectedResult: &servicePortInfo{
				meshPortName: "mesh",
				meshPort:     &pbcatalog.WorkloadPort{Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				servicePorts: map[string]*pbcatalog.WorkloadPort{
					"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				},
			},
		},
		"no address specific ports create union of workload ports": {
			// this test case shows:
			// - endpoint 1 has one address with no specific ports.
			// - endpoint 2 has one address with no specific ports.
			// - the cumulative effect is then the union of endpoints 1 and 2.
			serviceEndpoints: &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					{
						Addresses: []*pbcatalog.WorkloadAddress{
							{Host: "10.0.0.1"},
						},
						Ports: map[string]*pbcatalog.WorkloadPort{
							"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
						},
					},
					{
						Addresses: []*pbcatalog.WorkloadAddress{
							{Host: "10.0.0.2"},
						},
						Ports: map[string]*pbcatalog.WorkloadPort{
							"api-port": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
							"mesh":     {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
						},
					},
				},
			},
			expectedResult: &servicePortInfo{
				meshPortName: "mesh",
				meshPort:     &pbcatalog.WorkloadPort{Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				servicePorts: map[string]*pbcatalog.WorkloadPort{
					"api-port": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expectedResult, newServicePortInfo(tc.serviceEndpoints))
		})
	}
}
