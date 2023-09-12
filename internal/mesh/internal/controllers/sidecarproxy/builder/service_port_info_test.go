// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"testing"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/stretchr/testify/require"
)

// Test_newServicePortInfo test case shows:
// - endpoint 1 has one address with no specific ports.
// - endpoint 2 has one address with no specific ports.
// - the cumulative effect is then the union of endpoints 1 and 2.
func Test_newServicePortInfo(t *testing.T) {
	serviceEndpoints := &pbcatalog.ServiceEndpoints{
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
	}
	expectedResult := &servicePortInfo{
		meshPortName: "mesh",
		meshPort:     &pbcatalog.WorkloadPort{Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		servicePorts: map[string]*pbcatalog.WorkloadPort{
			"api-port": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
		},
	}
	require.Equal(t, expectedResult, newServicePortInfo(serviceEndpoints))
}
