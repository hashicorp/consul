// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/auth"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/proto-public/pbacl"
)

// Logout destroys the given ACL token once the caller is done with it.
func (s *Server) Logout(ctx context.Context, req *pbacl.LogoutRequest) (*pbacl.LogoutResponse, error) {
	logger := s.Logger.Named("logout").With("request_id", external.TraceID())
	logger.Trace("request received")
	fmt.Println(time.Now().String()+" ===================>  grpc logout called by Logout with token = ", req.Token)

	if err := s.requireACLsEnabled(logger); err != nil {
		return nil, err
	}
	fmt.Println(time.Now().String()+" ===================>  grpc logout called by Logout with 2 token = ", req.Token)

	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}
	fmt.Println(time.Now().String()+" ===================>  grpc logout called by Logout with 3 token = ", req.Token)

	// Forward request to leader in the requested datacenter.
	var rsp *pbacl.LogoutResponse
	handled, err := s.forwardWriteDC(req.Datacenter, func(conn *grpc.ClientConn) error {
		var err error
		rsp, err = pbacl.NewACLServiceClient(conn).Logout(ctx, req)
		return err
	}, logger)
	if handled || err != nil {
		return rsp, err
	}
	fmt.Println(time.Now().String()+" ===================>  grpc logout called by Logout with 4 token = ", req.Token)

	if err := s.requireLocalTokens(logger); err != nil {
		return nil, err
	}
	fmt.Println(time.Now().String()+" ===================>  grpc logout called by Logout with 5 token = ", req.Token)

	err = s.NewTokenWriter().Delete(req.Token, true)

	switch {
	case errors.Is(err, auth.ErrCannotWriteGlobalToken):
		// Writes to global tokens must be forwarded to the primary DC.
		req.Datacenter = s.PrimaryDatacenter
		fmt.Println(time.Now().String()+" ===================>  grpc logout called by Logout with 6 token = ", req.Token)

		_, err = s.forwardWriteDC(s.PrimaryDatacenter, func(conn *grpc.ClientConn) error {
			var err error
			rsp, err = pbacl.NewACLServiceClient(conn).Logout(ctx, req)
			return err
		}, logger)
		fmt.Println(time.Now().String()+" ===================>  grpc logout called by Logout with 7 token = ", req.Token)

		return rsp, err
	case errors.Is(err, acl.ErrNotFound):
		// No token? Pretend the delete was successful (for idempotency).
		return &pbacl.LogoutResponse{}, nil
	case errors.Is(err, acl.ErrPermissionDenied):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		logger.Error("failed to delete token", "error", err.Error())
		return nil, status.Error(codes.Internal, "failed to delete token")
	}
	fmt.Println(time.Now().String()+" ===================>  grpc logout called by Logout with 8 deleted deleted token = ", req.Token)

	return &pbacl.LogoutResponse{}, nil
}
