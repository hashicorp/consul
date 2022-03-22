package pbservice

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/proto/pbcommongogo"
)

func TestCheckServiceNode_UniqueID(t *testing.T) {
	type testCase struct {
		name     string
		csn      CheckServiceNode
		expected string
	}
	fn := func(t *testing.T, tc testCase) {
		require.Equal(t, tc.expected, tc.csn.UniqueID())
	}

	var testCases = []testCase{
		{
			name: "full",
			csn: CheckServiceNode{
				Node: &Node{Node: "the-node-name"},
				Service: &NodeService{
					ID:             "the-service-id",
					EnterpriseMeta: pbcommongogo.EnterpriseMeta{Namespace: "the-namespace"},
				},
			},
			expected: "/the-node-name/the-namespace/the-service-id",
		},
		{
			name: "without node",
			csn: CheckServiceNode{
				Service: &NodeService{
					ID:             "the-service-id",
					EnterpriseMeta: pbcommongogo.EnterpriseMeta{Namespace: "the-namespace"},
				},
			},
			expected: "/the-namespace/the-service-id",
		},
		{
			name: "without service",
			csn: CheckServiceNode{
				Node: &Node{Node: "the-node-name"},
			},
			expected: "/the-node-name/",
		},
		{
			name: "without namespace",
			csn: CheckServiceNode{
				Node: &Node{Node: "the-node-name"},
				Service: &NodeService{
					ID: "the-service-id",
				},
			},
			expected: "/the-node-name//the-service-id",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}

}
