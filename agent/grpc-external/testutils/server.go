// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutils

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type GRPCService interface {
	Register(*grpc.Server)
}

func RunTestServer(t *testing.T, services ...GRPCService) net.Addr {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	for _, svc := range services {
		svc.Register(grpcServer)
	}

	go grpcServer.Serve(lis)
	t.Cleanup(grpcServer.Stop)

	return lis.Addr()
}
