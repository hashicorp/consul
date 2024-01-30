// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

const (
	RESOURCE_TO_APPLY = `{
		"data": {
			"genre": "GENRE_METAL",
			"name": "Korn"
		},
		"id": {
			"name": "korn",
			"tenancy": {
				"partition": "default",
				"namespace": "default",
				"peerName": "local"
			},
			"type": {
				"group": "demo",
				"groupVersion": "v2",
				"kind": "Artist"
			}
		},
		"metadata": {
			"foo": "bar"
		}
	}`
)

func TestClientForwardToServer(t *testing.T) {
	type operation struct {
		action         func(agent libcluster.Agent) string
		expectedCode   int
		expectedMsg    string
		expectedErrMsg string
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
					action: func(a libcluster.Agent) string {
						//code, reader, err := a.GetPod().Exec(context.Background(), []string{fmt.Sprintf("echo %s | /bin/consul resource apply -f -", RESOURCE_TO_APPLY)})
						res, err := a.Exec(context.Background(), []string{fmt.Sprintf("which consul")})
						require.NoError(t, err)
						return res
					},
					expectedCode:   0,
					expectedMsg:    "demo.v2.Artist 'korn' created.",
					expectedErrMsg: "",
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			var clientAgent libcluster.Agent
			cluster := setupClusterAndClient(t)
			defer terminate(t, cluster)
			for _, a := range cluster.Agents {
				if !a.IsServer() {
					clientAgent = a
					break
				}
			}

			// perform actions and validate returned messages
			for _, op := range tc.operations {
				//code, reader := op.action(clientAgent)
				res := op.action(clientAgent)
				fmt.Printf("res: %s\n", res)
				//var buffer bytes.Buffer
				//_, err := buffer.ReadFrom(reader)
				//require.NoError(t, err)
				//require.Equal(t, op.expectedCode, code)
				//if code == 0 {
				//	require.Contains(t, buffer.String(), op.expectedMsg)
				//} else {
				//	require.Equal(t, buffer.String(), op.expectedErrMsg)
				//}
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
