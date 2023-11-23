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

func TestResourceReadInTLS(t *testing.T) {
	t.Parallel()

	a := agent.StartTestAgent(t, agent.TestAgent{
		HCL: `
			ports { grpc_tls = 8503 }
			enable_agent_tls_for_checks = true
			tls {
				defaults {
					verify_incoming = true
					key_file = "../../../test/client_certs/server.key"
					cert_file = "../../../test/client_certs/server.crt"
					ca_file = "../../../test/client_certs/rootca.crt"
				}
			}`,
		UseGRPCTLS: true,
	})
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	grpcConfig := &GRPCConfig{
		Address:  "127.0.0.1:8503",
		GRPCTLS:  true,
		CertFile: "../../../test/client_certs/client.crt",
		KeyFile:  "../../../test/client_certs/client.key",
		CAFile:   "../../../test/client_certs/rootca.crt",
	}
	gRPCClient, err := NewGRPCClient(grpcConfig)
	require.NoError(t, err)

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
