// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package operator

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pboperator"
)

type MockBackend struct {
	mock.Mock
	authorizer acl.Authorizer
}

func (m *MockBackend) TransferLeader(ctx context.Context, request *pboperator.TransferLeaderRequest) (*pboperator.TransferLeaderResponse, error) {
	called := m.Called(ctx, request)
	ret := called.Get(0)
	if ret == nil {
		return nil, called.Error(1)
	}
	return ret.(*pboperator.TransferLeaderResponse), called.Error(1)
}

func (m *MockBackend) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzCtx *acl.AuthorizerContext) (resolver.Result, error) {
	return resolver.Result{Authorizer: m.authorizer}, nil
}

func TestLeaderTransfer_ACL_Deny(t *testing.T) {
	authorizer := acl.MockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(acl.Deny)
	server := NewServer(Config{Datacenter: "dc1", Backend: &MockBackend{authorizer: &authorizer}, Logger: hclog.New(nil), ForwardRPC: doForwardRPC})

	_, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.Error(t, err)
	require.Equal(t, "Permission denied: token with AccessorID '' lacks permission 'operator:write'", err.Error())
}

func TestLeaderTransfer_ACL_Allowed(t *testing.T) {
	authorizer := &acl.MockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(acl.Allow)

	backend := &MockBackend{authorizer: authorizer}
	backend.On("TransferLeader", mock.Anything, mock.Anything).Return(nil, nil)
	server := NewServer(Config{Datacenter: "dc1", Backend: backend, Logger: hclog.New(nil), ForwardRPC: doForwardRPC})

	_, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.NoError(t, err)
}

func TestLeaderTransfer_LeaderTransfer_Fail(t *testing.T) {
	authorizer := &acl.MockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(acl.Allow)

	backend := &MockBackend{authorizer: authorizer}
	backend.On("TransferLeader", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("test"))
	server := NewServer(Config{Datacenter: "dc1", Backend: backend, Logger: hclog.New(nil), ForwardRPC: doForwardRPC})

	_, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.Error(t, err)
	require.Equal(t, "test", err.Error())
}

func TestLeaderTransfer_LeaderTransfer_Success(t *testing.T) {
	authorizer := &acl.MockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(acl.Allow)

	backend := &MockBackend{authorizer: authorizer}
	backend.On("TransferLeader", mock.Anything, mock.Anything).Return(&pboperator.TransferLeaderResponse{Success: true}, nil)
	server := NewServer(Config{Datacenter: "dc1", Backend: backend, Logger: hclog.New(nil), ForwardRPC: doForwardRPC})

	ret, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.NoError(t, err)
	require.NotNil(t, ret)
	require.True(t, ret.Success)
}

func TestLeaderTransfer_LeaderTransfer_ForwardRPC(t *testing.T) {
	authorizer := &acl.MockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(acl.Allow)

	backend := &MockBackend{authorizer: authorizer}
	backend.On("TransferLeader", mock.Anything, mock.Anything).Return(&pboperator.TransferLeaderResponse{}, nil)
	server := NewServer(Config{Datacenter: "dc1", Backend: backend, Logger: hclog.New(nil), ForwardRPC: noopForwardRPC})

	ret, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.NoError(t, err)
	require.NotNil(t, ret)
	require.False(t, ret.Success)
}
func noopForwardRPC(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error) {
	return true, nil
}

func doForwardRPC(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error) {
	return false, nil
}
