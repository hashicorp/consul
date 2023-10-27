// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/internal/resource"
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

		_, err = gRPCClient.Client.Read(context.Background(), &pbresource.ReadRequest{Id: v2Artist.Id})
		require.Equal(t, codes.NotFound.String(), status.Code(err).String())

		writeRsp, err := gRPCClient.Client.Write(testutil.TestContext(t), &pbresource.WriteRequest{Resource: v2Artist})
		require.NoError(t, err)

		readRsp, err := gRPCClient.Client.Read(context.Background(), &pbresource.ReadRequest{Id: v2Artist.Id})
		require.NoError(t, err)
		require.Equal(t, proto.Equal(readRsp.Resource.Id.Type, demo.TypeV2Artist), true)
		prototest.AssertDeepEqual(t, writeRsp.Resource, readRsp.Resource)
	})
}

func TestResourceList(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, "ports { grpc = 8602 }")
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	grpcConfig := GetDefaultGRPCConfig()
	grpcConfig.Address = "localhost:8602"
	gRPCClient, err := NewGRPCClient(grpcConfig)

	t.Cleanup(func() {
		a.Shutdown()
		gRPCClient.Conn.Close()
	})

	t.Run("test", func(t *testing.T) {
		defer gRPCClient.Conn.Close()
		if err != nil {
			fmt.Println("error when create new grpc client")
		}

		v2ArtistOne, err := demo.GenerateV2Artist()
		require.NoError(t, err)
		v2ArtistTwo, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		rsp, err := gRPCClient.Client.List(context.Background(), &pbresource.ListRequest{
			Type:       demo.TypeV2Artist,
			Tenancy:    resource.DefaultNamespacedTenancy(),
			NamePrefix: "",
		})
		require.NoError(t, err)
		require.Empty(t, rsp.Resources)

		resources := make([]*pbresource.Resource, 2)

		// Prevent test flakes if the generated names collide.
		v2ArtistOne.Id.Name = fmt.Sprintf("%s-%d", v2ArtistOne.Id.Name, 0)
		v2ArtistTwo.Id.Name = fmt.Sprintf("%s-%d", v2ArtistTwo.Id.Name, 1)

		writeRspOne, err := gRPCClient.Client.Write(testutil.TestContext(t), &pbresource.WriteRequest{Resource: v2ArtistOne})
		require.NoError(t, err)
		writeRspTwo, err := gRPCClient.Client.Write(testutil.TestContext(t), &pbresource.WriteRequest{Resource: v2ArtistTwo})
		require.NoError(t, err)

		resources[0] = writeRspOne.Resource
		resources[1] = writeRspTwo.Resource

		rsp, err = gRPCClient.Client.List(context.Background(), &pbresource.ListRequest{
			Type:       demo.TypeV2Artist,
			Tenancy:    resource.DefaultNamespacedTenancy(),
			NamePrefix: "",
		})
		require.NoError(t, err)
		prototest.AssertElementsMatch(t, resources, rsp.Resources)
	})
}

func TestResourceDelete(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, "ports { grpc = 8702 }")
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	grpcConfig := GetDefaultGRPCConfig()
	grpcConfig.Address = "localhost:8702"
	gRPCClient, err := NewGRPCClient(grpcConfig)

	t.Cleanup(func() {
		a.Shutdown()
		gRPCClient.Conn.Close()
	})

	t.Run("test", func(t *testing.T) {
		defer gRPCClient.Conn.Close()
		if err != nil {
			fmt.Println("error when create new grpc client")
		}

		v2Artist, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		_, err = gRPCClient.Client.Read(context.Background(), &pbresource.ReadRequest{Id: v2Artist.Id})
		require.Equal(t, codes.NotFound.String(), status.Code(err).String())

		writeRsp, err := gRPCClient.Client.Write(testutil.TestContext(t), &pbresource.WriteRequest{Resource: v2Artist})
		require.NoError(t, err)

		readRsp, err := gRPCClient.Client.Read(context.Background(), &pbresource.ReadRequest{Id: v2Artist.Id})
		require.NoError(t, err)
		require.Equal(t, proto.Equal(readRsp.Resource.Id.Type, demo.TypeV2Artist), true)
		prototest.AssertDeepEqual(t, writeRsp.Resource, readRsp.Resource)

		_, err = gRPCClient.Client.Delete(context.Background(), &pbresource.DeleteRequest{Id: readRsp.Resource.Id})
		require.NoError(t, err)

		_, err = gRPCClient.Client.Read(context.Background(), &pbresource.ReadRequest{Id: v2Artist.Id})
		require.Equal(t, codes.NotFound.String(), status.Code(err).String())
	})
}
