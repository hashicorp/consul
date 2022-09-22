package operator

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pboperator"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"testing"

	"github.com/stretchr/testify/require"
)

type MockBackend struct {
	mock.Mock
	authorizer acl.Authorizer
}

type mockAuthorizer struct {
	mock.Mock
}

func (m *mockAuthorizer) ACLRead(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) ACLWrite(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) AgentRead(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) AgentWrite(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) EventRead(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) EventWrite(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) IntentionDefaultAllow(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) IntentionRead(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) IntentionWrite(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) KeyList(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) KeyRead(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) KeyWrite(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) KeyWritePrefix(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) KeyringRead(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) KeyringWrite(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) MeshRead(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) MeshWrite(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) PeeringRead(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) PeeringWrite(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) NodeRead(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) NodeReadAll(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) NodeWrite(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) OperatorRead(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) OperatorWrite(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	called := m.Called(authorizerContext)
	return acl.EnforcementDecision(called.Int(0))
}

func (m *mockAuthorizer) PreparedQueryRead(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) PreparedQueryWrite(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) ServiceRead(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) ServiceReadAll(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) ServiceWrite(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) ServiceWriteAny(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) SessionRead(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) SessionWrite(s string, authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) Snapshot(authorizerContext *acl.AuthorizerContext) acl.EnforcementDecision {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthorizer) ToAllowAuthorizer() acl.AllowAuthorizer {
	//TODO implement me
	panic("implement me")
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

func TestLeaderTransfer_ACL_NotAllowed(t *testing.T) {
	authorizer := mockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(int(acl.Deny))
	server := NewServer(Config{Datacenter: "dc1", Backend: &MockBackend{authorizer: &authorizer}, Logger: hclog.New(nil), ForwardRPC: doForwardRPC})

	_, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.Error(t, err)
	require.Equal(t, "Permission denied: provided token lacks permission 'operator:write'", err.Error())
}

func TestLeaderTransfer_ACL_Allowed(t *testing.T) {
	authorizer := &mockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(1)

	backend := &MockBackend{authorizer: authorizer}
	backend.On("TransferLeader", mock.Anything, mock.Anything).Return(nil, nil)
	server := NewServer(Config{Datacenter: "dc1", Backend: backend, Logger: hclog.New(nil), ForwardRPC: doForwardRPC})

	_, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.NoError(t, err)
}

func TestLeaderTransfer_LeaderTransfer_Fail(t *testing.T) {
	authorizer := &mockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(1)

	backend := &MockBackend{authorizer: authorizer}
	backend.On("TransferLeader", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("test"))
	server := NewServer(Config{Datacenter: "dc1", Backend: backend, Logger: hclog.New(nil), ForwardRPC: doForwardRPC})

	_, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.Error(t, err)
	require.Equal(t, "test", err.Error())
}

func TestLeaderTransfer_LeaderTransfer_Success(t *testing.T) {
	authorizer := &mockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(1)

	backend := &MockBackend{authorizer: authorizer}
	backend.On("TransferLeader", mock.Anything, mock.Anything).Return(&pboperator.TransferLeaderResponse{Success: true}, nil)
	server := NewServer(Config{Datacenter: "dc1", Backend: backend, Logger: hclog.New(nil), ForwardRPC: doForwardRPC})

	ret, err := server.TransferLeader(context.Background(), &pboperator.TransferLeaderRequest{})
	require.NoError(t, err)
	require.NotNil(t, ret)
	require.True(t, ret.Success)
}

func TestLeaderTransfer_LeaderTransfer_ForwardRPC(t *testing.T) {
	authorizer := &mockAuthorizer{}
	authorizer.On("OperatorWrite", mock.Anything).Return(1)

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
