// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

const (
	RESOURCE_FILE_PATH_ON_HOST      = "../../../../../command/resource/testdata/demo.hcl"
	CLIENT_CERT_ON_HOST             = "../../../../client_certs/client.crt"
	CLIENT_KEY_ON_HOST              = "../../../../client_certs/client.key"
	ROOT_CA_ON_HOST                 = "../../../../client_certs/rootca.crt"
	RESOURCE_FILE_PATH_ON_CONTAINER = "/consul/data/demo.hcl"
	CLIENT_CERT_ON_CONTAINER        = "/consul/data/client.crt"
	CLIENT_KEY_ON_CONTAINER         = "/consul/data/client.key"
	ROOT_CA_ON_CONTAINER            = "/consul/data/rootca.crt"
)

func TestClientForwardToServer(t *testing.T) {
	type operation struct {
		action       func(*testing.T, libcluster.Agent, string, bool) (int, string)
		includeToken bool
		expectedCode int
		expectedMsg  string
	}
	type testCase struct {
		description    string
		operations     []operation
		aclEnabled     bool
		tlsEnabled     bool
		verifyIncoming bool
	}

	testCases := []testCase{
		{
			description: "The apply request should be forwarded to consul server agent",
			operations: []operation{
				{
					action:       applyResource,
					includeToken: false,
					expectedCode: 0,
					expectedMsg:  "demo.v2.Artist 'korn' created.",
				},
			},
			aclEnabled:     false,
			tlsEnabled:     false,
			verifyIncoming: false,
		},
		{
			description: "The apply request should be denied if missing token when ACL is enabled",
			operations: []operation{
				{
					action:       applyResource,
					includeToken: false,
					expectedCode: 1,
					expectedMsg:  "failed getting authorizer: ACL not found",
				},
			},
			aclEnabled: true,
		},
		{
			description: "The apply request should be allowed if providing token when ACL is enabled",
			operations: []operation{
				{
					action:       applyResource,
					includeToken: true,
					expectedCode: 0,
					expectedMsg:  "demo.v2.Artist 'korn' created.",
				},
			},
			aclEnabled:     true,
			tlsEnabled:     false,
			verifyIncoming: false,
		},
		{
			description: "The apply request should be forwarded to consul server agent when server is in TLS mode",
			operations: []operation{
				{
					action:       applyResource,
					includeToken: false,
					expectedCode: 0,
					expectedMsg:  "demo.v2.Artist 'korn' created.",
				},
			},
			aclEnabled:     false,
			tlsEnabled:     true,
			verifyIncoming: false,
		},
		{
			description: "The apply request should be forwarded to consul server agent when server and client are in TLS mode",
			operations: []operation{
				{
					action:       applyResource,
					includeToken: false,
					expectedCode: 0,
					expectedMsg:  "demo.v2.Artist 'korn' created.",
				},
			},
			aclEnabled:     false,
			tlsEnabled:     true,
			verifyIncoming: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			var clientAgent libcluster.Agent
			cluster, clientAgent := setupClusterAndClient(t, tc.aclEnabled, tc.tlsEnabled, tc.verifyIncoming)
			defer terminate(t, cluster)

			// perform actions and validate returned messages
			for _, op := range tc.operations {
				token := ""
				if op.includeToken {
					token = cluster.TokenBootstrap
				}
				code, res := op.action(t, clientAgent, token, tc.verifyIncoming)
				require.Equal(t, op.expectedCode, code)
				require.Contains(t, res, op.expectedMsg)
			}
		})
	}
}

func applyResource(t *testing.T, clientAgent libcluster.Agent, token string, verifyIncoming bool) (int, string) {
	c := clientAgent.GetConsulContainer()
	copyFilesToContainer(t, c, verifyIncoming)
	args := []string{"/bin/consul", "resource", "apply", fmt.Sprintf("-f=%s", RESOURCE_FILE_PATH_ON_CONTAINER)}
	if token != "" {
		args = append(args, fmt.Sprintf("-token=%s", token))
	}
	if verifyIncoming {
		args = append(
			args,
			"-grpc-tls=true",
			"-grpc-addr=127.0.0.1:8503",
			fmt.Sprintf("-client-cert=%s", CLIENT_CERT_ON_CONTAINER),
			fmt.Sprintf("-client-key=%s", CLIENT_KEY_ON_CONTAINER),
			fmt.Sprintf("-ca-file=%s", ROOT_CA_ON_CONTAINER),
		)
	}
	code, reader, err := c.Exec(context.Background(), args)
	require.NoError(t, err)
	buf, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(buf)
}

func copyFilesToContainer(t *testing.T, c testcontainers.Container, verifyIncoming bool) {
	err := c.CopyFileToContainer(context.Background(), RESOURCE_FILE_PATH_ON_HOST, RESOURCE_FILE_PATH_ON_CONTAINER, 700)
	require.NoError(t, err)
	if verifyIncoming {
		err = c.CopyFileToContainer(context.Background(), CLIENT_CERT_ON_HOST, CLIENT_CERT_ON_CONTAINER, 700)
		require.NoError(t, err)
		err = c.CopyFileToContainer(context.Background(), CLIENT_KEY_ON_HOST, CLIENT_KEY_ON_CONTAINER, 700)
		require.NoError(t, err)
		err = c.CopyFileToContainer(context.Background(), ROOT_CA_ON_HOST, ROOT_CA_ON_CONTAINER, 700)
		require.NoError(t, err)
	}
}

func setupClusterAndClient(t *testing.T, aclEnabled bool, tlsEnabled bool, verifyIncoming bool) (*libcluster.Cluster, libcluster.Agent) {
	clusterConfig := &libtopology.ClusterConfig{
		NumServers:  1,
		NumClients:  1,
		LogConsumer: &libtopology.TestLogConsumer{},
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: tlsEnabled,
			UseGRPCWithTLS:       tlsEnabled,
			ACLEnabled:           aclEnabled,
		},
		ApplyDefaultProxySettings: false,
	}
	if verifyIncoming {
		clusterConfig.Cmd = "-hcl=tls { defaults { verify_incoming = true } }"
	}
	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)

	return cluster, cluster.Clients()[0]
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
