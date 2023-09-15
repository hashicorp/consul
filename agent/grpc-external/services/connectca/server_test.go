// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connectca

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
)

func noopForwardRPC(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error) {
	return false, nil
}

func setupFSMAndPublisher(t *testing.T) (*testutils.FakeFSM, state.EventPublisher) {
	t.Helper()

	config := testutils.FakeFSMConfig{
		Register: func(fsm *testutils.FakeFSM, publisher *stream.EventPublisher) {
			// register handlers
			publisher.RegisterHandler(state.EventTopicCARoots, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
				return fsm.GetStore().CARootsSnapshot(req, buf)
			}, false)
		},
		Refresh: []stream.Topic{state.EventTopicCARoots},
	}

	return testutils.SetupFSMAndPublisher(t, config)
}

func testClient(t *testing.T, server *Server) pbconnectca.ConnectCAServiceClient {
	t.Helper()

	addr := testutils.RunTestServer(t, server)

	//nolint:staticcheck
	conn, err := grpc.DialContext(context.Background(), addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return pbconnectca.NewConnectCAServiceClient(conn)
}
