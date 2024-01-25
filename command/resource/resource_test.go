// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/command/resource/apply"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

const (
	GRPC_PORT     = "8502"
	GRPC_TLS_PORT = "8503"
)

func TestClientForwardToServer(t *testing.T) {
	type operation struct {
		action         func(clientPort int) (*cli.MockUi, int)
		expectedErrMsg string
		expectedMsg    string
		expectedLog    string
	}
	type testCase struct {
		description string
		operations  []operation
	}

	testCases := []testCase{
		{
			description: "The read request should be routed to consul server agent",
			operations: []operation{
				{
					action: func(clientPort int) (*cli.MockUi, int) {
						ui := cli.NewMockUi()
						c := apply.New(ui)
						args := []string{
							"-f=../../../../../command/resource/testdata/demo.hcl",
							fmt.Sprintf("-grpc-addr=127.0.0.1:%d", clientPort),
							"-token=root",
						}
						code := c.Run(args)
						return ui, code
					},
					expectedErrMsg: "",
					expectedMsg:    "demo.v2.Artist 'korn' created.",
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			var clientPort nat.Port
			cluster := setupClusterAndClient(t)
			defer terminate(t, cluster)
			for _, a := range cluster.Agents {
				var err error
				if !a.IsServer() {
					clientPort, err = a.GetPod().MappedPort(context.Background(), GRPC_PORT)
					require.NoError(t, err)
				}
			}

			// perform actions and validate returned messages
			for _, op := range tc.operations {
				mockUI, code := op.action(clientPort.Int())
				if code == 0 {
					require.Empty(t, mockUI.ErrorWriter.String())
					require.Contains(t, mockUI.OutputWriter.String(), op.expectedMsg)
				} else {
					require.Equal(t, mockUI.ErrorWriter.String(), op.expectedErrMsg)
				}
			}
		})
	}
}

// passing two cmd args to set up the cluster
func setupClusterAndClient(t *testing.T) *libcluster.Cluster {
	clusterConfig := &libtopology.ClusterConfig{
		NumServers:  1,
		NumClients:  1,
		LogConsumer: &libtopology.TestLogConsumer{},
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
		},
		ApplyDefaultProxySettings: false,
	}
	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)

	return cluster
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
