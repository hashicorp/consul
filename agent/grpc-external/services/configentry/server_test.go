// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type MockBackend struct {
	mock.Mock
	authorizer acl.Authorizer
}

func (m *MockBackend) ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (resolver.Result, error) {
	return resolver.Result{Authorizer: m.authorizer}, nil
}

func (m *MockBackend) EnterpriseCheckPartitions(partition string) error {
	called := m.Called(partition)
	ret := called.Get(0)

	if ret == nil {
		return nil
	} else {
		return ret.(error)
	}
}

func TestGetResolvedExportedServices_ACL_Deny(t *testing.T) {
	authorizer := acl.MockAuthorizer{}
	authorizer.On("MeshRead", mock.Anything).Return(acl.Deny)

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

	_, err := server.GetResolvedExportedServices(context.Background(), &pbconfigentry.GetResolvedExportedServicesRequest{})
	require.Error(t, err)
}

func TestGetResolvedExportedServices_AC_Allow(t *testing.T) {
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

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), &testutils.MockServerTransportStream{})
	_, err := server.GetResolvedExportedServices(ctx, &pbconfigentry.GetResolvedExportedServicesRequest{})
	require.NoError(t, err)
}

func TestGetResolvedExportedServices_PartitionCheck(t *testing.T) {
	authorizer := acl.MockAuthorizer{}
	authorizer.On("MeshRead", mock.Anything).Return(acl.Allow)

	backend := &MockBackend{authorizer: &authorizer}
	backend.On("EnterpriseCheckPartitions", mock.Anything).Return(fmt.Errorf("partition not supported"))

	fakeFSM := testutils.NewFakeBlockingFSM(t)

	c := Config{
		Backend:    backend,
		Logger:     hclog.New(nil),
		ForwardRPC: doForwardRPC,
		FSMServer:  fakeFSM,
	}

	server := NewServer(c)

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), &testutils.MockServerTransportStream{})

	resp, err := server.GetResolvedExportedServices(ctx, &pbconfigentry.GetResolvedExportedServicesRequest{})
	require.EqualError(t, err, "rpc error: code = InvalidArgument desc = partition not supported")
	require.Nil(t, resp)
}

func TestGetResolvedExportedServices_Index(t *testing.T) {
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

	headerStream := &testutils.MockServerTransportStream{}

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), headerStream)
	resp, err := server.GetResolvedExportedServices(ctx, &pbconfigentry.GetResolvedExportedServicesRequest{})
	require.NoError(t, err)
	require.Equal(t, 2, len(resp.Services))
	require.Equal(t, []string{"1"}, headerStream.MD.Get("index"))

	// Updating the index
	fakeFSM.GetState().EnsureConfigEntry(2, entry)

	headerStream = &testutils.MockServerTransportStream{}

	ctx = grpc.NewContextWithServerTransportStream(context.Background(), headerStream)
	resp, err = server.GetResolvedExportedServices(ctx, &pbconfigentry.GetResolvedExportedServicesRequest{})
	require.NoError(t, err)
	require.Equal(t, 2, len(resp.Services))
	require.Equal(t, []string{"2"}, headerStream.MD.Get("index"))
}

func TestGetResolvedExportedServices_Metrics(t *testing.T) {
	sink := metrics.NewInmemSink(5*time.Second, time.Minute)
	cfg := metrics.DefaultConfig("consul")
	metrics.NewGlobal(cfg, sink)

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

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), &testutils.MockServerTransportStream{})
	resp, err := server.GetResolvedExportedServices(ctx, &pbconfigentry.GetResolvedExportedServicesRequest{})
	require.NoError(t, err)
	require.Equal(t, 2, len(resp.Services))

	// Checking if metrics were added
	require.NotNil(t, sink.Data()[0].Samples[`consul.configentry.get_resolved_exported_services`])
}

func doForwardRPC(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error) {
	return false, nil
}
