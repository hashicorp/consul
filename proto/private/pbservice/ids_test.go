// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pbservice

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/proto/private/pbcommon"
)

func TestCheckServiceNode_UniqueID(t *testing.T) {
	type testCase struct {
		name     string
		csn      *CheckServiceNode
		expected string
	}
	fn := func(t *testing.T, tc *testCase) {
		require.Equal(t, tc.expected, tc.csn.UniqueID())
	}

	var testCases = []testCase{
		{
			name: "full",
			csn: &CheckServiceNode{
				Node: &Node{
					Node:      "the-node-name",
					PeerName:  "my-peer",
					Partition: "the-partition",
				},
				Service: &NodeService{
					ID: "the-service-id",
					EnterpriseMeta: &pbcommon.EnterpriseMeta{
						Partition: "the-partition",
						Namespace: "the-namespace",
					},
					PeerName: "my-peer",
				},
			},
			expected: "the-partition/my-peer/the-node-name/the-namespace/the-service-id",
		},
		{
			name: "without node",
			csn: &CheckServiceNode{
				Service: &NodeService{
					ID: "the-service-id",
					EnterpriseMeta: &pbcommon.EnterpriseMeta{
						Partition: "the-partition",
						Namespace: "the-namespace",
					},
					PeerName: "my-peer",
				},
			},
			expected: "the-partition/my-peer/the-namespace/the-service-id",
		},
		{
			name: "without service",
			csn: &CheckServiceNode{
				Node: &Node{
					Node:      "the-node-name",
					PeerName:  "my-peer",
					Partition: "the-partition",
				},
			},
			expected: "the-partition/my-peer/the-node-name/",
		},
		{
			name: "without namespace",
			csn: &CheckServiceNode{
				Node: &Node{
					Node:      "the-node-name",
					PeerName:  "my-peer",
					Partition: "the-partition",
				},
				Service: &NodeService{
					ID:       "the-service-id",
					PeerName: "my-peer",
					EnterpriseMeta: &pbcommon.EnterpriseMeta{
						Partition: "the-partition",
					},
				},
			},
			expected: "the-partition/my-peer/the-node-name//the-service-id",
		},
		{
			name: "without peer name",
			csn: &CheckServiceNode{
				Node: &Node{
					Node:      "the-node-name",
					Partition: "the-partition",
				},
				Service: &NodeService{
					ID: "the-service-id",
					EnterpriseMeta: &pbcommon.EnterpriseMeta{
						Partition: "the-partition",
						Namespace: "the-namespace",
					},
				},
			},
			expected: "the-partition//the-node-name/the-namespace/the-service-id",
		},
		{
			name: "without partition",
			csn: &CheckServiceNode{
				Node: &Node{
					Node:     "the-node-name",
					PeerName: "my-peer",
				},
				Service: &NodeService{
					ID:       "the-service-id",
					PeerName: "my-peer",
					EnterpriseMeta: &pbcommon.EnterpriseMeta{
						Namespace: "the-namespace",
					},
				},
			},
			expected: "/my-peer/the-node-name/the-namespace/the-service-id",
		},
		{
			name: "without partition or namespace",
			csn: &CheckServiceNode{
				Node: &Node{
					Node:     "the-node-name",
					PeerName: "my-peer",
				},
				Service: &NodeService{
					ID:       "the-service-id",
					PeerName: "my-peer",
				},
			},
			expected: "/my-peer/the-node-name//the-service-id",
		},
		{
			name: "without partition or namespace or peer name",
			csn: &CheckServiceNode{
				Node: &Node{
					Node: "the-node-name",
				},
				Service: &NodeService{
					ID: "the-service-id",
				},
			},
			expected: "//the-node-name//the-service-id",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, &tc)
		})
	}
}
