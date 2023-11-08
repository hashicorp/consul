// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/auth"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbacl"
)

func TestServer_Logout_Success(t *testing.T) {
	secretID := generateID(t)

	tokenWriter := NewMockTokenWriter(t)
	tokenWriter.On("Delete", secretID, true).Return(nil)

	server := NewServer(Config{
		ACLsEnabled:         true,
		InPrimaryDatacenter: true,
		ForwardRPC:          noopForwardRPC,
		LocalTokensEnabled:  noopLocalTokensEnabled,
		Logger:              hclog.NewNullLogger(),
		NewTokenWriter:      func() TokenWriter { return tokenWriter },
	})

	_, err := server.Logout(context.Background(), &pbacl.LogoutRequest{
		Token: secretID,
	})
	require.NoError(t, err)
}

func TestServer_Logout_EmptyToken(t *testing.T) {
	server := NewServer(Config{
		ACLsEnabled: true,
		Logger:      hclog.NewNullLogger(),
	})

	_, err := server.Logout(context.Background(), &pbacl.LogoutRequest{
		Token: "",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "token is required")
}

func TestServer_Logout_ACLsDisabled(t *testing.T) {
	server := NewServer(Config{
		ACLsEnabled:               false,
		Logger:                    hclog.NewNullLogger(),
		ValidateEnterpriseRequest: noopValidateEnterpriseRequest,
		ForwardRPC:                noopForwardRPC,
		LocalTokensEnabled:        noopLocalTokensEnabled,
	})

	_, err := server.Logout(context.Background(), &pbacl.LogoutRequest{
		Token: generateID(t),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
}

func TestServer_Logout_LocalTokensDisabled(t *testing.T) {
	server := NewServer(Config{
		ACLsEnabled:        true,
		Logger:             hclog.NewNullLogger(),
		ForwardRPC:         noopForwardRPC,
		LocalTokensEnabled: func() bool { return false },
	})

	_, err := server.Logout(context.Background(), &pbacl.LogoutRequest{
		Token: generateID(t),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "token replication is required")
}

func TestServer_Logout_NoSuchToken(t *testing.T) {
	tokenWriter := NewMockTokenWriter(t)
	tokenWriter.On("Delete", mock.Anything, true).Return(acl.ErrNotFound)

	server := NewServer(Config{
		ACLsEnabled:        true,
		Logger:             hclog.NewNullLogger(),
		ForwardRPC:         noopForwardRPC,
		LocalTokensEnabled: noopLocalTokensEnabled,
		NewTokenWriter:     func() TokenWriter { return tokenWriter },
	})

	_, err := server.Logout(context.Background(), &pbacl.LogoutRequest{
		Token: generateID(t),
	})
	require.NoError(t, err)
}

func TestServer_Logout_PermissionDenied(t *testing.T) {
	tokenWriter := NewMockTokenWriter(t)
	tokenWriter.On("Delete", mock.Anything, true).Return(acl.ErrPermissionDenied)

	server := NewServer(Config{
		ACLsEnabled:         true,
		InPrimaryDatacenter: true,
		ForwardRPC:          noopForwardRPC,
		LocalTokensEnabled:  noopLocalTokensEnabled,
		Logger:              hclog.NewNullLogger(),
		NewTokenWriter:      func() TokenWriter { return tokenWriter },
	})

	_, err := server.Logout(context.Background(), &pbacl.LogoutRequest{
		Token: generateID(t),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
}

func TestServer_Logout_RPCForwarding(t *testing.T) {
	tokenWriter := NewMockTokenWriter(t)
	tokenWriter.On("Delete", mock.Anything, true).Return(nil)

	dc1 := NewServer(Config{
		ACLsEnabled:        true,
		Logger:             hclog.NewNullLogger(),
		NewTokenWriter:     func() TokenWriter { return tokenWriter },
		ForwardRPC:         noopForwardRPC,
		LocalTokensEnabled: func() bool { return true },
	})

	dc1Conn, err := grpc.Dial(
		testutils.RunTestServer(t, dc1).String(),
		//nolint:staticcheck
		grpc.WithInsecure(),
	)
	require.NoError(t, err)

	dc2 := NewServer(Config{
		ACLsEnabled: true,
		Logger:      hclog.NewNullLogger(),
		ForwardRPC: func(rpcInfo structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			return true, fn(dc1Conn)
		},
	})
	_, err = dc2.Logout(context.Background(), &pbacl.LogoutRequest{
		Token: generateID(t),
	})
	require.NoError(t, err)
}

func TestServer_Logout_GlobalWritesForwardedToPrimaryDC(t *testing.T) {
	tokenWriter := NewMockTokenWriter(t)
	tokenWriter.On("Delete", mock.Anything, true).Return(nil)

	// This test checks that requests to delete global tokens are forwared to the
	// primary datacenter by:
	//
	//	1. Setting up 2 servers (1 in the primary DC, 1 in the secondary).
	//	2. Making a logout request to the secondary DC.
	//	3. Mocking TokenWriter.Delete to return ErrCannotWriteGlobalToken in the
	//	   secondary DC.
	//	4. Checking that the primary DC server's TokenWriter receives a call to
	//	   Delete.
	//	5. Capturing the forwarded request's Datacenter in the primary DC server's
	//	   ForwardRPC (to check that we overwrote the user-supplied Datacenter
	//	   field to prevent infinite forwarding loops!)
	var forwardedRequestDatacenter string
	primary := NewServer(Config{
		ACLsEnabled:         true,
		InPrimaryDatacenter: true,
		LocalTokensEnabled:  noopLocalTokensEnabled,
		Logger:              hclog.NewNullLogger(),
		NewTokenWriter:      func() TokenWriter { return tokenWriter },
		ForwardRPC: func(info structs.RPCInfo, _ func(*grpc.ClientConn) error) (bool, error) {
			forwardedRequestDatacenter = info.RequestDatacenter()
			return false, nil
		},
	})

	primaryConn, err := grpc.Dial(
		testutils.RunTestServer(t, primary).String(),
		//nolint:staticcheck
		grpc.WithInsecure(),
	)
	require.NoError(t, err)

	secondary := NewServer(Config{
		ACLsEnabled:         true,
		InPrimaryDatacenter: false,
		LocalTokensEnabled:  noopLocalTokensEnabled,
		Logger:              hclog.NewNullLogger(),
		PrimaryDatacenter:   "primary",
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			dc := info.RequestDatacenter()
			switch dc {
			case "secondary":
				return false, nil
			case "primary":
				return true, fn(primaryConn)
			default:
				return false, fmt.Errorf("unexpected target datacenter: %s", dc)
			}
		},
		NewTokenWriter: func() TokenWriter {
			tokenWriter := NewMockTokenWriter(t)
			tokenWriter.On("Delete", mock.Anything, true).Return(auth.ErrCannotWriteGlobalToken)
			return tokenWriter
		},
	})

	_, err = secondary.Logout(context.Background(), &pbacl.LogoutRequest{
		Token:      generateID(t),
		Datacenter: "secondary",
	})
	require.NoError(t, err)
	require.Equal(t, "primary", forwardedRequestDatacenter)
}
