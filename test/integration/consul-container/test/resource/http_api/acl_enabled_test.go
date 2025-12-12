// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ServerAgent_Endpoints(t *testing.T) {
	t.Parallel()

	numOfServers := 1
	numOfClients := 0
	cluster, resourceClient := SetupClusterAndClient(t, makeClusterConfig(numOfServers, numOfClients, true), true)

	resource := Resource{
		HttpClient: resourceClient,
	}

	defer Terminate(t, cluster)

	testCases := []testCase{
		{
			description: "should write resource successfully when token is provided",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
					includeToken:     true,
				},
				{
					action:           readResource,
					expectedErrorMsg: "",
					includeToken:     true,
				},
				{
					action:           listResource,
					expectedErrorMsg: "",
					includeToken:     true,
				},
				{
					action:           deleteResource,
					expectedErrorMsg: "",
					includeToken:     true,
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
			description: "should return permission denied",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "Unexpected response code: 403 (rpc error: code = PermissionDenied desc = Permission denied",
					includeToken:     false,
				},
				{
					action:           applyResource,
					expectedErrorMsg: "",
					includeToken:     true,
				},
				{
					action:           readResource,
					expectedErrorMsg: "Unexpected response code: 403 (rpc error: code = PermissionDenied desc = Permission denied",
					includeToken:     false,
				},
				{
					action:           listResource,
					expectedErrorMsg: "Unexpected response code: 403 (rpc error: code = PermissionDenied desc = Permission denied",
					includeToken:     false,
				},
				{
					action:           deleteResource,
					expectedErrorMsg: "Unexpected response code: 403 (rpc error: code = PermissionDenied desc = Permission denied",
					includeToken:     false,
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "prince",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					resourceName: "deleteme",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
				{
					gvk:          demoGVK,
					resourceName: "keith",
					queryOptions: defaultTenancyQueryOptions,
				},
				{
					gvk:          demoGVK,
					queryOptions: defaultTenancyQueryOptions,
				},
				{
					gvk:          demoGVK,
					resourceName: "deleteme",
					queryOptions: defaultTenancyQueryOptions,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			for i, op := range tc.operations {
				if op.includeToken {
					tc.config[i].queryOptions.Token = cluster.TokenBootstrap
				}

				err := op.action(&resource, tc.config[i])
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Contains(t, err.Error(), op.expectedErrorMsg)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func Test_ClientAgent(t *testing.T) {
	t.Parallel()

	numOfServers := 1
	numOfClients := 1
	cluster, resourceClient := SetupClusterAndClient(t, makeClusterConfig(numOfServers, numOfClients, true), false)

	resource := Resource{
		HttpClient: resourceClient,
	}

	defer Terminate(t, cluster)

	testCases := []testCase{
		{
			description: "should write resource successfully when token is provided",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "",
					includeToken:     true,
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "test",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
			},
		},
		{
			description: "should return unauthorized when token is bad",
			operations: []operation{
				{
					action:           applyResource,
					expectedErrorMsg: "Unexpected response code: 403 (rpc error: code = PermissionDenied desc = Permission denied",
					includeToken:     false,
				},
			},
			config: []config{
				{
					gvk:          demoGVK,
					resourceName: "test2",
					queryOptions: defaultTenancyQueryOptions,
					payload:      demoPayload,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			for i, op := range tc.operations {
				if op.includeToken {
					tc.config[i].queryOptions.Token = cluster.TokenBootstrap
				}

				err := op.action(&resource, tc.config[i])
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Contains(t, err.Error(), op.expectedErrorMsg)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}
