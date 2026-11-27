// Copyright IBM Corp. 2024, 2026
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
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestResourceRead(t *testing.T) {
	availablePort := freeport.GetOne(t)
	a := agent.NewTestAgent(t, fmt.Sprintf("ports { grpc = %d }", availablePort))
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	grpcConfig, err := LoadGRPCConfig(&GRPCConfig{Address: fmt.Sprintf("127.0.0.1:%d", availablePort)})
	require.NoError(t, err)
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

		writeRsp, err := gRPCClient.Client.Write(testutil.TestContext(t), &pbresource.WriteRequest{Resource: v2Artist})
		require.NoError(t, err)

		readRsp, err := gRPCClient.Client.Read(context.Background(), &pbresource.ReadRequest{Id: v2Artist.Id})
		require.NoError(t, err)
		require.Equal(t, proto.Equal(readRsp.Resource.Id.Type, demo.TypeV2Artist), true)
		prototest.AssertDeepEqual(t, writeRsp.Resource, readRsp.Resource)
	})
}

func TestResourceReadInTLS(t *testing.T) {
	tests := []struct {
		name              string
		requireClientCert bool
		grpcConfig        func() (*GRPCConfig, int)
		// expectErr is set when the TLS handshake must be rejected, in which
		// case no RPC (and therefore no ACL token) reaches the server.
		expectErr string
	}{
		{
			name:              "Test with CertFile, KeyFile and CAFile",
			requireClientCert: true,
			grpcConfig: func() (*GRPCConfig, int) {
				availablePort := freeport.GetOne(t)
				return &GRPCConfig{
					Address:  fmt.Sprintf("127.0.0.1:%d", availablePort),
					GRPCTLS:  true,
					CertFile: "../../../test/client_certs/client.crt",
					KeyFile:  "../../../test/client_certs/client.key",
					CAFile:   "../../../test/client_certs/rootca.crt",
				}, availablePort
			},
		},
		{
			name:              "Test without CAFile and verification explicitly disabled",
			requireClientCert: true,
			grpcConfig: func() (*GRPCConfig, int) {
				availablePort := freeport.GetOne(t)
				return &GRPCConfig{
					Address:            fmt.Sprintf("127.0.0.1:%d", availablePort),
					GRPCTLS:            true,
					InsecureSkipVerify: true,
					CertFile:           "../../../test/client_certs/client.crt",
					KeyFile:            "../../../test/client_certs/client.key",
				}, availablePort
			},
		},
		{
			name:              "Test without client certificates",
			requireClientCert: false,
			grpcConfig: func() (*GRPCConfig, int) {
				availablePort := freeport.GetOne(t)
				return &GRPCConfig{
					Address: fmt.Sprintf("127.0.0.1:%d", availablePort),
					GRPCTLS: true,
					CAFile:  "../../../test/client_certs/rootca.crt",
				}, availablePort
			},
		},
		{
			name:              "Test with TLS server name",
			requireClientCert: false,
			grpcConfig: func() (*GRPCConfig, int) {
				availablePort := freeport.GetOne(t)
				return &GRPCConfig{
					Address:       fmt.Sprintf("127.0.0.1:%d", availablePort),
					GRPCTLS:       true,
					TLSServerName: "server.dc1.consul",
					CAFile:        "../../../test/client_certs/rootca.crt",
				}, availablePort
			},
		},
		{
			name:              "Test with IP address TLS server name",
			requireClientCert: false,
			grpcConfig: func() (*GRPCConfig, int) {
				availablePort := freeport.GetOne(t)
				return &GRPCConfig{
					Address:       fmt.Sprintf("localhost:%d", availablePort),
					GRPCTLS:       true,
					TLSServerName: "127.0.0.1",
					CAFile:        "../../../test/client_certs/rootca.crt",
				}, availablePort
			},
		},
		{
			// The server presents a certificate the client does not trust.
			name:              "Test untrusted server certificate is rejected by default",
			requireClientCert: false,
			grpcConfig: func() (*GRPCConfig, int) {
				availablePort := freeport.GetOne(t)
				return &GRPCConfig{
					Address: fmt.Sprintf("127.0.0.1:%d", availablePort),
					GRPCTLS: true,
				}, availablePort
			},
			expectErr: "authentication handshake failed",
		},
		{
			name:              "Test server certificate for a different name is rejected",
			requireClientCert: false,
			grpcConfig: func() (*GRPCConfig, int) {
				availablePort := freeport.GetOne(t)
				return &GRPCConfig{
					Address:       fmt.Sprintf("127.0.0.1:%d", availablePort),
					GRPCTLS:       true,
					TLSServerName: "other.example.com",
					CAFile:        "../../../test/client_certs/rootca.crt",
				}, availablePort
			},
			expectErr: "authentication handshake failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcClientConfig, availablePort := tt.grpcConfig()
			a := agent.StartTestAgent(t, agent.TestAgent{
				HCL: fmt.Sprintf(`
					ports { grpc_tls = %d }
					enable_agent_tls_for_checks = true
					tls {
						defaults {
							verify_incoming = %t
							key_file = "../../../test/client_certs/server.key"
							cert_file = "../../../test/client_certs/server.crt"
							ca_file = "../../../test/client_certs/rootca.crt"
						}
					}`, availablePort, tt.requireClientCert),
				UseGRPCTLS: true,
			})
			testrpc.WaitForTestAgent(t, a.RPC, "dc1")
			grpcConfig, err := LoadGRPCConfig(grpcClientConfig)
			require.NoError(t, err)
			gRPCClient, err := NewGRPCClient(grpcConfig)
			require.NoError(t, err)

			t.Cleanup(func() {
				a.Shutdown()
				gRPCClient.Conn.Close()
			})

			v2Artist, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			if tt.expectErr != "" {
				_, err = gRPCClient.Client.Read(context.Background(), &pbresource.ReadRequest{Id: v2Artist.Id})
				require.Equal(t, codes.Unavailable.String(), status.Code(err).String())
				require.ErrorContains(t, err, tt.expectErr)
				return
			}

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
}
