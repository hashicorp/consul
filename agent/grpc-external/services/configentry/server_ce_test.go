// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGetResolvedExportedServices(t *testing.T) {
	authorizer := acl.MockAuthorizer{}
	authorizer.On("MeshRead", mock.Anything).Return(acl.Allow)

	backend := &MockBackend{authorizer: &authorizer}
	backend.On("EnterpriseCheckPartitions", mock.Anything).Return(nil)

	fakeFSM := testutils.NewFakeBlockingFSM(t)

	c := Config{
		Backend:    backend,
		Logger:     hclog.New(nil),
		ForwardRPC: doForwardRPC,
		FSMServer:  fakeFSM,
	}
	server := NewServer(c)

	// Add config entry
	entry := &structs.ExportedServicesConfigEntry{
		Name: "default",
		Services: []structs.ExportedService{
			{
				Name: "db",
				Consumers: []structs.ServiceConsumer{
					{
						Peer: "east",
					},
					{
						Peer: "west",
					},
				},
			},
			{
				Name: "cache",
				Consumers: []structs.ServiceConsumer{
					{
						Peer: "east",
					},
				},
			},
		},
	}
	fakeFSM.GetState().EnsureConfigEntry(1, entry)

	expected := []*pbconfigentry.ResolvedExportedService{
		{
			Service: "cache",
			Consumers: &pbconfigentry.Consumers{
				Peers: []string{"east"},
			},
		},
		{
			Service: "db",
			Consumers: &pbconfigentry.Consumers{
				Peers: []string{"east", "west"},
			},
		},
	}

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), &testutils.MockServerTransportStream{})
	resp, err := server.GetResolvedExportedServices(ctx, &pbconfigentry.GetResolvedExportedServicesRequest{})
	require.NoError(t, err)
	require.Equal(t, expected, resp.Services)
}
