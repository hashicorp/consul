// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

const (
	RESOURCE_FILE_PATH_ON_HOST      = "../../../../../command/resource/testdata/demo.hcl"
	RESOURCE_FILE_PATH_ON_CONTAINER = "/consul/data/demo.hcl"
)

func TestClientForwardToServer(t *testing.T) {
	type operation struct {
		action       func(*testing.T, libcluster.Agent, string) (int, string)
		includeToken bool
		expectedCode int
		expectedMsg  string
	}
	type testCase struct {
		description string
		operations  []operation
		aclEnabled  bool
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
			aclEnabled: false,
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
				{
					action:       applyResource,
					includeToken: true,
					expectedCode: 0,
					expectedMsg:  "demo.v2.Artist 'korn' created.",
				},
			},
			aclEnabled: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			var clientAgent libcluster.Agent
			cluster, clientAgent := setupClusterAndClient(t, tc.aclEnabled)
			defer terminate(t, cluster)

			// perform actions and validate returned messages
			for _, op := range tc.operations {
				token := ""
				if op.includeToken {
					token = cluster.TokenBootstrap
				}
				code, res := op.action(t, clientAgent, token)
				require.Equal(t, op.expectedCode, code)
				require.Contains(t, res, op.expectedMsg)
			}
		})
	}
}

func applyResource(t *testing.T, clientAgent libcluster.Agent, token string) (int, string) {
	ctx := context.Background()
	c := clientAgent.GetConsulContainer()
	err := c.CopyFileToContainer(ctx, RESOURCE_FILE_PATH_ON_HOST, RESOURCE_FILE_PATH_ON_CONTAINER, 700)
	require.NoError(t, err)
	args := []string{"/bin/consul", "resource", "apply", fmt.Sprintf("-f=%s", RESOURCE_FILE_PATH_ON_CONTAINER)}
	if token != "" {
		args = append(args, fmt.Sprintf("-token=%s", token))
	}
	code, reader, err := c.Exec(ctx, args)
	require.NoError(t, err)
	buf, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(buf)
}

// passing two cmd args to set up the cluster
func setupClusterAndClient(t *testing.T, aclEnabled bool) (*libcluster.Cluster, libcluster.Agent) {
	clusterConfig := &libtopology.ClusterConfig{
		NumServers:  1,
		NumClients:  1,
		LogConsumer: &libtopology.TestLogConsumer{},
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			ACLEnabled:             aclEnabled,
		},
		ApplyDefaultProxySettings: false,
	}
	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)
	var clientAgent libcluster.Agent
	for _, a := range cluster.Agents {
		if !a.IsServer() {
			clientAgent = a
			break
		}
	}

	return cluster, clientAgent
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
