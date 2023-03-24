package connectca

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
)

func TestSign_ConnectDisabled(t *testing.T) {
	server := NewServer(Config{ConnectEnabled: false})

	_, err := server.Sign(context.Background(), &pbconnectca.SignRequest{})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
	require.Contains(t, status.Convert(err).Message(), "Connect")
}

func TestSign_Validation(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil)

	server := NewServer(Config{
		Logger:         hclog.NewNullLogger(),
		ACLResolver:    aclResolver,
		ForwardRPC:     noopForwardRPC,
		ConnectEnabled: true,
	})

	testCases := map[string]struct {
		csr, err string
	}{
		"no csr": {
			csr: "",
			err: "CSR is required",
		},
		"invalid csr": {
			csr: "bogus",
			err: "no PEM-encoded data found",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			_, err := server.Sign(context.Background(), &pbconnectca.SignRequest{
				Csr: tc.csr,
			})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Equal(t, tc.err, status.Convert(err).Message())
		})
	}
}

func TestSign_Unauthenticated(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(resolver.Result{}, acl.ErrNotFound)

	server := NewServer(Config{
		Logger:         hclog.NewNullLogger(),
		ACLResolver:    aclResolver,
		ForwardRPC:     noopForwardRPC,
		ConnectEnabled: true,
	})

	csr, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))

	_, err := server.Sign(context.Background(), &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated.String(), status.Code(err).String())
}

func TestSign_PermissionDenied(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil)

	caManager := &MockCAManager{}
	caManager.On("AuthorizeAndSignCertificate", mock.Anything, mock.Anything).
		Return(nil, acl.ErrPermissionDenied)

	server := NewServer(Config{
		Logger:         hclog.NewNullLogger(),
		ACLResolver:    aclResolver,
		CAManager:      caManager,
		ForwardRPC:     noopForwardRPC,
		ConnectEnabled: true,
	})

	csr, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))

	_, err := server.Sign(context.Background(), &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
}

func TestSign_InvalidCSR(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil)

	caManager := &MockCAManager{}
	caManager.On("AuthorizeAndSignCertificate", mock.Anything, mock.Anything).
		Return(nil, connect.InvalidCSRError("nope"))

	server := NewServer(Config{
		Logger:         hclog.NewNullLogger(),
		ACLResolver:    aclResolver,
		CAManager:      caManager,
		ForwardRPC:     noopForwardRPC,
		ConnectEnabled: true,
	})

	csr, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))

	_, err := server.Sign(context.Background(), &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
}

func TestSign_RateLimited(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil)

	caManager := &MockCAManager{}
	caManager.On("AuthorizeAndSignCertificate", mock.Anything, mock.Anything).
		Return(nil, errors.New("Rate limit reached, try again later"))

	server := NewServer(Config{
		Logger:         hclog.NewNullLogger(),
		ACLResolver:    aclResolver,
		CAManager:      caManager,
		ForwardRPC:     noopForwardRPC,
		ConnectEnabled: true,
	})

	csr, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))

	_, err := server.Sign(context.Background(), &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.Error(t, err)
	require.Equal(t, codes.ResourceExhausted.String(), status.Code(err).String())
}

func TestSign_InternalError(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil)

	caManager := &MockCAManager{}
	caManager.On("AuthorizeAndSignCertificate", mock.Anything, mock.Anything).
		Return(nil, errors.New("something went very wrong"))

	server := NewServer(Config{
		Logger:         hclog.NewNullLogger(),
		ACLResolver:    aclResolver,
		CAManager:      caManager,
		ForwardRPC:     noopForwardRPC,
		ConnectEnabled: true,
	})

	csr, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))

	_, err := server.Sign(context.Background(), &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal.String(), status.Code(err).String())
}

func TestSign_Success(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil)

	caManager := &MockCAManager{}
	caManager.On("AuthorizeAndSignCertificate", mock.Anything, mock.Anything).
		Return(&structs.IssuedCert{CertPEM: "this is the PEM"}, nil)

	server := NewServer(Config{
		Logger:         hclog.NewNullLogger(),
		ACLResolver:    aclResolver,
		CAManager:      caManager,
		ForwardRPC:     noopForwardRPC,
		ConnectEnabled: true,
	})

	csr, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))

	rsp, err := server.Sign(context.Background(), &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.NoError(t, err)
	require.Equal(t, "this is the PEM", rsp.CertPem)
}

func TestSign_RPCForwarding(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil)

	caManager := &MockCAManager{}
	caManager.On("AuthorizeAndSignCertificate", mock.Anything, mock.Anything).
		Return(&structs.IssuedCert{CertPEM: "leader response"}, nil)

	leader := NewServer(Config{
		Logger:         hclog.NewNullLogger(),
		ACLResolver:    aclResolver,
		CAManager:      caManager,
		ForwardRPC:     noopForwardRPC,
		ConnectEnabled: true,
	})
	//nolint:staticcheck
	leaderConn, err := grpc.Dial(testutils.RunTestServer(t, leader).String(), grpc.WithInsecure())
	require.NoError(t, err)

	follower := NewServer(Config{
		Logger: hclog.NewNullLogger(),
		ForwardRPC: func(_ structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			return true, fn(leaderConn)
		},
		ConnectEnabled: true,
	})

	csr, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))

	rsp, err := follower.Sign(context.Background(), &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.NoError(t, err)
	require.Equal(t, "leader response", rsp.CertPem)
}
