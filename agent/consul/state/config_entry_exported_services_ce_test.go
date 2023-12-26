// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
	"github.com/stretchr/testify/require"
)

func TestStore_prepareExportedServicesResponse(t *testing.T) {
	var exportedServices = make(map[structs.ServiceName]map[structs.ServiceConsumer]struct{})

	svc1 := structs.NewServiceName("db", nil)
	exportedServices[svc1] = make(map[structs.ServiceConsumer]struct{})
	exportedServices[svc1][structs.ServiceConsumer{Peer: "west"}] = struct{}{}
	exportedServices[svc1][structs.ServiceConsumer{Peer: "east"}] = struct{}{}

	// Adding partition to ensure that it's not included in response
	exportedServices[svc1][structs.ServiceConsumer{Partition: "east"}] = struct{}{}

	svc2 := structs.NewServiceName("web", nil)
	exportedServices[svc2] = make(map[structs.ServiceConsumer]struct{})
	exportedServices[svc2][structs.ServiceConsumer{Peer: "peer-a"}] = struct{}{}
	exportedServices[svc2][structs.ServiceConsumer{Peer: "peer-b"}] = struct{}{}

	resp := prepareExportedServicesResponse(exportedServices)

	expected := []*pbconfigentry.ResolvedExportedService{
		{
			Service: "db",
			Consumers: &pbconfigentry.Consumers{
				Peers: []string{"east", "west"},
			},
		},
		{
			Service: "web",
			Consumers: &pbconfigentry.Consumers{
				Peers: []string{"peer-a", "peer-b"},
			},
		},
	}

	require.ElementsMatch(t, expected, resp)
}
