// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package resource

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

var clusterConfig = makeClusterConfig(1, 1, false)

func TestWriteEndpoint(t *testing.T) {
	testCases := []testCase{
		{
			description: "should apply resource successfully",
			operations: []operation{
				{
					action:           applyResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           applyResource,
					isServerOp:       false,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					resourceName: "korn-client",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
			},
		},
		{
			description: "should not apply resource successfully: 400 bad request",
			operations: []operation{
				{
					action:           applyResource,
					isServerOp:       true,
					expectedErrorMsg: "Unexpected response code: 400 (Request body didn't follow the resource schema)",
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      api.WriteRequest{},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			cluster, server, client := SetupClusterAndClient(t, clusterConfig)
			defer Terminate(t, cluster)

			for i, op := range tc.operations {
				if op.isServerOp {
					client = server
				}
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
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           readResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           readResource,
					isServerOp:       false,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
				},
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
				},
			},
		},
		{
			description: "should not read resource successfully: 404",
			operations: []operation{
				{
					action:           applyResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           readResource,
					isServerOp:       true,
					expectedErrorMsg: "Unexpected response code: 404 (rpc error: code = NotFound desc = resource not found)",
				},
				{
					action:           readResource,
					isServerOp:       false,
					expectedErrorMsg: "Unexpected response code: 404 (rpc error: code = NotFound desc = resource not found)",
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					resourceName: "fake-korn",
					queryOptions: defaultTenancyQueryOptions,
				},
				{
					gvk:          demoGVK,
					resourceName: "fake-korn",
					queryOptions: defaultTenancyQueryOptions,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			cluster, server, client := SetupClusterAndClient(t, clusterConfig)
			defer Terminate(t, cluster)

			for i, op := range tc.operations {
				if op.isServerOp {
					client = server
				}
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
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           deleteResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           applyResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           deleteResource,
					isServerOp:       false,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
				},
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
				},
			},
		},
		{
			description: "should delete resource successfully even if resource does not exist",
			operations: []operation{
				{
					action:           applyResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           deleteResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           deleteResource,
					isServerOp:       false,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					resourceName: "fake-korn",
					queryOptions: defaultTenancyQueryOptions,
				},
				{
					gvk:          demoGVK,
					resourceName: "fake-korn",
					queryOptions: defaultTenancyQueryOptions,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			cluster, server, client := SetupClusterAndClient(t, clusterConfig)
			defer Terminate(t, cluster)

			for i, op := range tc.operations {
				if op.isServerOp {
					client = server
				}
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
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           listResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           listResource,
					isServerOp:       false,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					queryOptions: defaultTenancyQueryOptions,
				},
				{
					gvk:          demoGVK,
					queryOptions: defaultTenancyQueryOptions,
				},
			},
		},
		{
			description: "should read empty resource list",
			operations: []operation{
				{
					action:           applyResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           listResource,
					isServerOp:       true,
					expectedErrorMsg: "",
				},
				{
					action:           listResource,
					isServerOp:       false,
					expectedErrorMsg: "",
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "korn",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					queryOptions: fakeTenancyQueryOptions,
				},
				{
					gvk:          demoGVK,
					queryOptions: fakeTenancyQueryOptions,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			cluster, server, client := SetupClusterAndClient(t, clusterConfig)
			defer Terminate(t, cluster)

			for i, op := range tc.operations {
				if op.isServerOp {
					client = server
				}
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
