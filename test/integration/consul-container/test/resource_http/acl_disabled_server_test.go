// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package resource_http

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

type config struct {
	gvk          api.GVK
	resourceName string
	queryOptions api.QueryOptions
	payload      api.WriteRequest
}
type operation struct {
	action           func(client *api.Client, config config) error
	expectedErrorMsg string
}
type testCase struct {
	description string
	operations  []operation
	config      []config
}

var clusterConfig = &libtopology.ClusterConfig{
	NumServers:  1,
	NumClients:  0,
	LogConsumer: &libtopology.TestLogConsumer{},
	BuildOpts: &libcluster.BuildOptions{
		Datacenter:             "dc1",
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
	},
	ApplyDefaultProxySettings: false,
}
var applyResource = func(client *api.Client, config config) error {
	_, _, err := client.Resource().Apply(&config.gvk, config.resourceName, &config.queryOptions, &config.payload)
	return err
}
var readResource = func(client *api.Client, config config) error {
	_, err := client.Resource().Read(&config.gvk, config.resourceName, &config.queryOptions)
	return err
}
var deleteResource = func(client *api.Client, config config) error {
	err := client.Resource().Delete(&config.gvk, config.resourceName, &config.queryOptions)
	return err
}
var listResource = func(client *api.Client, config config) error {
	_, err := client.Resource().List(&config.gvk, &config.queryOptions)
	return err
}

func TestWriteEndpoint(t *testing.T) {
	testCases := []testCase{
		{
			description: "should apply resource successfully",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
					payload:      commonPayload,
				},
			},
		},
		{
			description: "should not apply resource successfully: 400 bad request",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "Unexpected response code: 400 (Request body didn't follow the resource schema)",
				},
			},
			config: []config{
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
					payload:      api.WriteRequest{},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			cluster, client := SetupClusterAndClient(t, clusterConfig, true)
			defer Terminate(t, cluster)

			for i, op := range tc.operations {
				err := op.action(client, tc.config[i])
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Equal(t, op.expectedErrorMsg, err.Error())
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestReadEndpoint(t *testing.T) {
	testCases := []testCase{
		{
			description: "should read resource successfully",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
				},
				{
					action:           readResource,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
					payload:      commonPayload,
				},
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
				},
			},
		},
		{
			description: "should not read resource successfully: 404",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
				},
				{
					action:           readResource,
					expectedErrorMsg: "Unexpected response code: 404 (rpc error: code = NotFound desc = resource not found)",
				},
			},
			config: []config{
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
					payload:      commonPayload,
				},
				{
					gvk:          commonGVK,
					resourceName: "fake-korn",
					queryOptions: commonQueryOptions,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			cluster, client := SetupClusterAndClient(t, clusterConfig, true)
			defer Terminate(t, cluster)

			for i, op := range tc.operations {
				err := op.action(client, tc.config[i])
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Equal(t, op.expectedErrorMsg, err.Error())
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestDeleteEndpoint(t *testing.T) {
	testCases := []testCase{
		{
			description: "should delete resource successfully",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
				},
				{
					action:           deleteResource,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
					payload:      commonPayload,
				},
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
				},
			},
		},
		{
			description: "should delete resource successfully even if resource does not exist",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
				},
				{
					action:           deleteResource,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
					payload:      commonPayload,
				},
				{
					gvk:          commonGVK,
					resourceName: "fake-korn",
					queryOptions: commonQueryOptions,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			cluster, client := SetupClusterAndClient(t, clusterConfig, true)
			defer Terminate(t, cluster)

			for i, op := range tc.operations {
				err := op.action(client, tc.config[i])
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Equal(t, op.expectedErrorMsg, err.Error())
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestListEndpoint(t *testing.T) {
	testCases := []testCase{
		{
			description: "should list resource successfully",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
				},
				{
					action:           listResource,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
					payload:      commonPayload,
				},
				{
					gvk:          commonGVK,
					queryOptions: commonQueryOptions,
				},
			},
		},
		{
			description: "should read empty resource list",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
				},
				{
					action:           listResource,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          commonGVK,
					resourceName: "korn",
					queryOptions: commonQueryOptions,
					payload:      commonPayload,
				},
				{
					gvk:          commonGVK,
					queryOptions: fakeQueryOptions,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			cluster, client := SetupClusterAndClient(t, clusterConfig, true)
			defer Terminate(t, cluster)

			for i, op := range tc.operations {
				err := op.action(client, tc.config[i])
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Equal(t, op.expectedErrorMsg, err.Error())
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}
