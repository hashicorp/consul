// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestResourceRead(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, "ports { grpc = 8502 }")
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	grpcConfig := GetDefaultGRPCConfig()
	gRPCClient, err := NewGRPCClient(grpcConfig)

	t.Cleanup(func() {
		a.Shutdown()
		gRPCClient.Conn.Close()
	})

	t.Run("test", func(t *testing.T) {
		if err != nil {
			fmt.Println("error when create new grpc client")
		}

		v2Artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		writeRsp, err := gRPCClient.Client.Write(testutil.TestContext(t), &pbresource.WriteRequest{Resource: v2Artist})
		require.NoError(t, err)

		readRsp, err := gRPCClient.Client.Read(context.Background(), &pbresource.ReadRequest{Id: v2Artist.Id})
		require.NoError(t, err)
		require.Equal(t, proto.Equal(readRsp.Resource.Id.Type, demo.TypeV2Artist), true)
		prototest.AssertDeepEqual(t, writeRsp.Resource, readRsp.Resource)
	})
}
