// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/grpc-internal/balancer"
	"github.com/hashicorp/consul/agent/grpc-internal/resolver"
	"github.com/hashicorp/consul/agent/grpc-middleware/testutil/testservice"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func TestHandler_PanicRecoveryInterceptor(t *testing.T) {
	// Prepare a logger with output to a buffer
	// so we can check what it writes.
	var buf bytes.Buffer

	logger := hclog.New(&hclog.LoggerOptions{
		Output: &buf,
	})

	res := resolver.NewServerResolverBuilder(newConfig(t, "dc1", "server"))
	bb := balancer.NewBuilder(res.Authority(), testutil.Logger(t))
	registerWithGRPC(t, res, bb)

	srv := newPanicTestServer(t, logger, "server-1", "dc1", nil)
	res.AddServer(types.AreaLAN, srv.Metadata())
	t.Cleanup(srv.shutdown)

	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		UseTLSForDC:           useTLSForDcAlwaysTrue,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)
	client := testservice.NewSimpleClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	resp, err := client.Something(ctx, &testservice.Req{})
	expectedErr := status.Errorf(codes.Internal, "grpc: panic serving request")
	require.Equal(t, expectedErr.Error(), err.Error())
	require.Nil(t, resp)

	// Read the log
	strLog := buf.String()
	// Checking the entire stack trace is not possible, let's
	// make sure that it contains a couple of expected strings.
	require.Contains(t, strLog, `[ERROR] panic serving grpc request: panic="panic from Something`)
	require.Contains(t, strLog, `github.com/hashicorp/consul/agent/grpc-middleware/testutil/testservice.(*SimplePanic).Something`)

}
