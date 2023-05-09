package acl

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbacl"
)

const bearerToken = "bearer-token"

func TestServer_Login_Success(t *testing.T) {
	authMethod := &structs.ACLAuthMethod{}
	identity := &authmethod.Identity{}

	validator := NewMockValidator(t)
	validator.On("ValidateLogin", mock.Anything, bearerToken).
		Return(identity, nil)

	token := &structs.ACLToken{
		AccessorID: "accessor-id",
		SecretID:   "secret-id",
	}

	login := NewMockLogin(t)
	login.On("TokenForVerifiedIdentity", identity, authMethod, "token created via login").
		Return(token, nil)

	server := NewServer(Config{
		ACLsEnabled: true,
		Logger:      hclog.NewNullLogger(),
		LoadAuthMethod: func(methodName string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, Validator, error) {
			return authMethod, validator, nil
		},
		ForwardRPC:                noopForwardRPC,
		ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
		LocalTokensEnabled:        noopLocalTokensEnabled,
		NewLogin:                  func() Login { return login },
	})

	rsp, err := server.Login(context.Background(), &pbacl.LoginRequest{
		BearerToken: bearerToken,
	})
	require.NoError(t, err)
	require.Equal(t, token.AccessorID, rsp.Token.AccessorId)
	require.Equal(t, token.SecretID, rsp.Token.SecretId)
}

func TestServer_Login_LoadAuthMethodErrors(t *testing.T) {
	testCases := map[string]struct {
		error error
		code  codes.Code
	}{
		"auth method not found": {
			// Note: we wrap the error here to make sure we correctly unwrap it in the handler.
			error: fmt.Errorf("%w auth method not found", acl.ErrNotFound),
			code:  codes.InvalidArgument,
		},
		"unexpected error": {
			error: errors.New("BOOM"),
			code:  codes.Internal,
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			server := NewServer(Config{
				ACLsEnabled: true,
				Logger:      hclog.NewNullLogger(),
				LoadAuthMethod: func(methodName string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, Validator, error) {
					return nil, nil, tc.error
				},
				ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
				LocalTokensEnabled:        noopLocalTokensEnabled,
				ForwardRPC:                noopForwardRPC,
			})
			_, err := server.Login(context.Background(), &pbacl.LoginRequest{
				BearerToken: bearerToken,
			})
			require.Error(t, err)
			require.Equal(t, tc.code.String(), status.Code(err).String())
		})
	}
}

func TestServer_Login_ValidateEnterpriseRequest(t *testing.T) {
	server := NewServer(Config{
		ACLsEnabled:               true,
		Logger:                    hclog.NewNullLogger(),
		ValidateEnterpriseRequest: func(*acl.EnterpriseMeta, bool) error { return errors.New("BOOM") },
		ForwardRPC:                noopForwardRPC,
	})

	_, err := server.Login(context.Background(), &pbacl.LoginRequest{
		BearerToken: bearerToken,
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal.String(), status.Code(err).String())
}

func TestServer_Login_ACLsDisabled(t *testing.T) {
	server := NewServer(Config{
		ACLsEnabled:               false,
		Logger:                    hclog.NewNullLogger(),
		ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
		ForwardRPC:                noopForwardRPC,
		LocalTokensEnabled:        noopLocalTokensEnabled,
	})

	_, err := server.Login(context.Background(), &pbacl.LoginRequest{
		BearerToken: bearerToken,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
}

func TestServer_Login_LocalTokensDisabled(t *testing.T) {
	server := NewServer(Config{
		ACLsEnabled:               true,
		Logger:                    hclog.NewNullLogger(),
		ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
		ForwardRPC:                noopForwardRPC,
		LocalTokensEnabled:        func() bool { return false },
	})

	_, err := server.Login(context.Background(), &pbacl.LoginRequest{
		BearerToken: bearerToken,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
}

func TestServer_Login_ValidateLoginError(t *testing.T) {
	validator := NewMockValidator(t)
	validator.On("ValidateLogin", mock.Anything, bearerToken).
		Return(nil, errors.New("BOOM"))

	server := NewServer(Config{
		ACLsEnabled: true,
		Logger:      hclog.NewNullLogger(),
		LoadAuthMethod: func(methodName string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, Validator, error) {
			return &structs.ACLAuthMethod{}, validator, nil
		},
		ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
		LocalTokensEnabled:        noopLocalTokensEnabled,
		ForwardRPC:                noopForwardRPC,
	})

	_, err := server.Login(context.Background(), &pbacl.LoginRequest{
		BearerToken: bearerToken,
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated.String(), status.Code(err).String())
}

func TestServer_Login_TokenForVerifiedIdentityErrors(t *testing.T) {
	testCases := map[string]struct {
		error error
		code  codes.Code
	}{
		"permission denied": {
			error: acl.ErrPermissionDenied,
			code:  codes.PermissionDenied,
		},
		"unexpected error": {
			error: errors.New("BOOM"),
			code:  codes.Internal,
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			validator := NewMockValidator(t)
			validator.On("ValidateLogin", mock.Anything, bearerToken).
				Return(&authmethod.Identity{}, nil)

			login := NewMockLogin(t)
			login.On("TokenForVerifiedIdentity", mock.Anything, mock.Anything, mock.Anything).
				Return(nil, tc.error)

			server := NewServer(Config{
				ACLsEnabled: true,
				Logger:      hclog.NewNullLogger(),
				LoadAuthMethod: func(methodName string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, Validator, error) {
					return &structs.ACLAuthMethod{}, validator, nil
				},
				ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
				LocalTokensEnabled:        noopLocalTokensEnabled,
				ForwardRPC:                noopForwardRPC,
				NewLogin:                  func() Login { return login },
			})

			_, err := server.Login(context.Background(), &pbacl.LoginRequest{
				BearerToken: bearerToken,
			})
			require.Error(t, err)
			require.Equal(t, tc.code.String(), status.Code(err).String())
		})
	}
}

func TestServer_Login_RPCForwarding(t *testing.T) {
	validator := NewMockValidator(t)
	validator.On("ValidateLogin", mock.Anything, mock.Anything).
		Return(&authmethod.Identity{}, nil)

	login := NewMockLogin(t)
	login.On("TokenForVerifiedIdentity", mock.Anything, mock.Anything, mock.Anything).
		Return(&structs.ACLToken{AccessorID: "leader response"}, nil)

	dc2 := NewServer(Config{
		ACLsEnabled: true,
		Logger:      hclog.NewNullLogger(),
		LoadAuthMethod: func(methodName string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, Validator, error) {
			return &structs.ACLAuthMethod{}, validator, nil
		},
		ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
		LocalTokensEnabled:        noopLocalTokensEnabled,
		ForwardRPC:                noopForwardRPC,
		NewLogin:                  func() Login { return login },
	})

	//nolint:staticcheck
	leaderConn, err := grpc.Dial(testutils.RunTestServer(t, dc2).String(), grpc.WithInsecure())
	require.NoError(t, err)

	dc1 := NewServer(Config{
		ACLsEnabled: true,
		Logger:      hclog.NewNullLogger(),
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			if dc := info.RequestDatacenter(); dc != "dc2" {
				return false, fmt.Errorf("unexpected target datacenter: %s", dc)
			}
			return true, fn(leaderConn)
		},
		ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
	})

	rsp, err := dc1.Login(context.Background(), &pbacl.LoginRequest{
		BearerToken: bearerToken,
		Datacenter:  "dc2",
	})
	require.NoError(t, err)
	require.Equal(t, "leader response", rsp.Token.AccessorId)
}
