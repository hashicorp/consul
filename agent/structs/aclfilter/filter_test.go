// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package aclfilter

import (
	"reflect"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

func TestACL_filterImported_IndexedHealthChecks(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	type testCase struct {
		policyRules string
		list        *structs.IndexedHealthChecks
		expectEmpty bool
	}

	run := func(t *testing.T, tc testCase) {
		policy, err := acl.NewPolicyFromSource(tc.policyRules, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		New(authz, logger).Filter(tc.list)

		if tc.expectEmpty {
			require.Empty(t, tc.list.HealthChecks)
		} else {
			require.Len(t, tc.list.HealthChecks, 1)
		}
	}

	tt := map[string]testCase{
		"permissions for imports (Allowed)": {
			policyRules: `
service_prefix "" { policy = "read" } node_prefix "" { policy = "read" }`,
			list: &structs.IndexedHealthChecks{
				HealthChecks: structs.HealthChecks{
					{
						Node:        "node1",
						CheckID:     "check1",
						ServiceName: "foo",
						PeerName:    "some-peer",
					},
				},
			},
			// Can read imports with wildcard service/node reads in the importing partition.
			expectEmpty: false,
		},
		"permissions for local only (Deny)": {
			policyRules: `
service "foo" { policy = "read" } node "node1" { policy = "read" }`,
			list: &structs.IndexedHealthChecks{
				HealthChecks: structs.HealthChecks{
					{
						Node:        "node1",
						CheckID:     "check1",
						ServiceName: "foo",
						PeerName:    "some-peer",
					},
				},
			},
			// Cannot read imports with rules referencing local resources with the same name
			// as the imported ones.
			expectEmpty: true,
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestACL_filterImported_IndexedNodes(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	type testCase struct {
		policyRules string
		list        *structs.IndexedNodes
		configFunc  func(config *acl.Config)
		expectEmpty bool
	}

	run := func(t *testing.T, tc testCase) {
		policy, err := acl.NewPolicyFromSource(tc.policyRules, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		New(authz, logger).Filter(tc.list)

		if tc.expectEmpty {
			require.Empty(t, tc.list.Nodes)
		} else {
			require.Len(t, tc.list.Nodes, 1)
		}
	}

	tt := map[string]testCase{
		"permissions for imports (Allowed)": {
			policyRules: `
		node_prefix "" { policy = "read" }`,
			list: &structs.IndexedNodes{
				Nodes: structs.Nodes{
					{
						ID:         types.NodeID("1"),
						Node:       "foo",
						Address:    "127.0.0.1",
						Datacenter: "dc1",
						PeerName:   "some-peer",
					},
				},
			},
			// Can read imports with wildcard service/node reads in the importing partition.
			expectEmpty: false,
		},
		"permissions for local only (Deny)": {
			policyRules: `
node "node1" { policy = "read" }`,
			list: &structs.IndexedNodes{
				Nodes: structs.Nodes{
					{
						ID:         types.NodeID("1"),
						Node:       "node1",
						Address:    "127.0.0.1",
						Datacenter: "dc1",
						PeerName:   "some-peer",
					},
				},
			},
			// Cannot read imports with rules referencing local resources with the same name
			// as the imported ones.
			expectEmpty: true,
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestACL_filterImported_IndexedNodeServices(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	type testCase struct {
		policyRules string
		list        *structs.IndexedNodeServices
		configFunc  func(config *acl.Config)
		expectEmpty bool
	}

	run := func(t *testing.T, tc testCase) {
		policy, err := acl.NewPolicyFromSource(tc.policyRules, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		New(authz, logger).Filter(tc.list)

		if tc.expectEmpty {
			require.Nil(t, tc.list.NodeServices)
		} else {
			require.Len(t, tc.list.NodeServices.Services, 1)
		}
	}

	tt := map[string]testCase{
		"permissions for imports (Allowed)": {
			policyRules: `
service_prefix "" { policy = "read" } node_prefix "" { policy = "read" }`,
			list: &structs.IndexedNodeServices{
				NodeServices: &structs.NodeServices{
					Node: &structs.Node{
						Node:     "node1",
						PeerName: "some-peer",
					},
					Services: map[string]*structs.NodeService{
						"foo": {
							ID:       "foo",
							Service:  "foo",
							PeerName: "some-peer",
						},
					},
				},
			},
			// Can read imports with wildcard service/node reads in the importing partition.
			expectEmpty: false,
		},
		"permissions for local only (Deny)": {
			policyRules: `
service "foo" { policy = "read" } node "node1" { policy = "read" }`,
			list: &structs.IndexedNodeServices{
				NodeServices: &structs.NodeServices{
					Node: &structs.Node{
						Node:     "node1",
						PeerName: "some-peer",
					},
					Services: map[string]*structs.NodeService{
						"foo": {
							ID:       "foo",
							Service:  "foo",
							PeerName: "some-peer",
						},
					},
				},
			},
			// Cannot read imports with rules referencing local resources with the same name
			// as the imported ones.
			expectEmpty: true,
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestACL_filterImported_IndexedNodeServiceList(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	type testCase struct {
		policyRules string
		list        *structs.IndexedNodeServiceList
		configFunc  func(config *acl.Config)
		expectEmpty bool
	}

	run := func(t *testing.T, tc testCase) {
		policy, err := acl.NewPolicyFromSource(tc.policyRules, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		New(authz, logger).Filter(tc.list)

		if tc.expectEmpty {
			require.Nil(t, tc.list.NodeServices.Node)
			require.Nil(t, tc.list.NodeServices.Services)
		} else {
			require.Len(t, tc.list.NodeServices.Services, 1)
		}
	}

	tt := map[string]testCase{
		"permissions for imports (Allowed)": {
			policyRules: `
service_prefix "" { policy = "read" } node_prefix "" { policy = "read" }`,
			list: &structs.IndexedNodeServiceList{
				NodeServices: structs.NodeServiceList{
					Node: &structs.Node{
						Node:     "node1",
						PeerName: "some-peer",
					},
					Services: []*structs.NodeService{
						{
							Service:  "foo",
							PeerName: "some-peer",
						},
					},
				},
			},
			// Can read imports with wildcard service/node reads in the importing partition.
			expectEmpty: false,
		},
		"permissions for local only (Deny)": {
			policyRules: `
service "foo" { policy = "read" } node "node1" { policy = "read" }`,
			list: &structs.IndexedNodeServiceList{
				NodeServices: structs.NodeServiceList{
					Node: &structs.Node{
						Node:     "node1",
						PeerName: "some-peer",
					},
					Services: []*structs.NodeService{
						{
							Service:  "foo",
							PeerName: "some-peer",
						},
					},
				},
			},
			// Cannot read imports with rules referencing local resources with the same name
			// as the imported ones.
			expectEmpty: true,
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestACL_filterImported_IndexedServiceNodes(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	type testCase struct {
		policyRules string
		list        *structs.IndexedServiceNodes
		configFunc  func(config *acl.Config)
		expectEmpty bool
	}

	run := func(t *testing.T, tc testCase) {
		policy, err := acl.NewPolicyFromSource(tc.policyRules, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		New(authz, logger).Filter(tc.list)

		if tc.expectEmpty {
			require.Empty(t, tc.list.ServiceNodes)
		} else {
			require.Len(t, tc.list.ServiceNodes, 1)
		}
	}

	tt := map[string]testCase{
		"permissions for imports (Allowed)": {
			policyRules: `
service_prefix "" { policy = "read" } node_prefix "" { policy = "read" }`,
			list: &structs.IndexedServiceNodes{
				ServiceNodes: structs.ServiceNodes{
					{
						Node:        "node1",
						ServiceName: "foo",
						PeerName:    "some-peer",
					},
				},
			},
			// Can read imports with wildcard service/node reads in the importing partition.
			expectEmpty: false,
		},
		"permissions for local only (Deny)": {
			policyRules: `
service "foo" { policy = "read" } node "node1" { policy = "read" }`,
			list: &structs.IndexedServiceNodes{
				ServiceNodes: structs.ServiceNodes{
					{
						Node:        "node1",
						ServiceName: "foo",
						PeerName:    "some-peer",
					},
				},
			},
			// Cannot read imports with rules referencing local resources with the same name
			// as the imported ones.
			expectEmpty: true,
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestACL_filterImported_CheckServiceNode(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	type testCase struct {
		policyRules string
		list        *structs.CheckServiceNodes
		configFunc  func(config *acl.Config)
		expectEmpty bool
	}

	run := func(t *testing.T, tc testCase) {
		policy, err := acl.NewPolicyFromSource(tc.policyRules, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		New(authz, logger).Filter(tc.list)

		if tc.expectEmpty {
			require.Empty(t, tc.list)
		} else {
			require.Len(t, *tc.list, 1)
		}
	}

	tt := map[string]testCase{
		"permissions for imports (Allowed)": {
			policyRules: `
service_prefix "" { policy = "read" } node_prefix "" { policy = "read" }`,
			list: &structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:     "node1",
						PeerName: "some-peer",
					},
					Service: &structs.NodeService{
						ID:       "foo",
						Service:  "foo",
						PeerName: "some-peer",
					},
					Checks: structs.HealthChecks{
						{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "foo",
							PeerName:    "some-peer",
						},
					},
				},
			},
			// Can read imports with wildcard service/node reads in the importing partition.
			expectEmpty: false,
		},
		"permissions for local only (Deny)": {
			policyRules: `
service "foo" { policy = "read" } node "node1" { policy = "read" }`,
			list: &structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:     "node1",
						PeerName: "some-peer",
					},
					Service: &structs.NodeService{
						ID:       "foo",
						Service:  "foo",
						PeerName: "some-peer",
					},
					Checks: structs.HealthChecks{
						{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "foo",
							PeerName:    "some-peer",
						},
					},
				},
			},
			// Cannot read imports with rules referencing local resources with the same name
			// as the imported ones.
			expectEmpty: true,
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestACL_filterHealthChecks(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedHealthChecks {
		return &structs.IndexedHealthChecks{
			HealthChecks: structs.HealthChecks{
				{
					Node:        "node1",
					CheckID:     "check1",
					ServiceName: "foo",
				},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			service_prefix "foo" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
			node_prefix "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.HealthChecks, 1)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read the service, but not the node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			service_prefix "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.HealthChecks)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("allowed to read the node, but not the service", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
			node_prefix "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.HealthChecks)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.HealthChecks)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterIntentions(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedIntentions {
		return &structs.IndexedIntentions{
			Intentions: structs.Intentions{
				&structs.Intention{
					ID:              "f004177f-2c28-83b7-4229-eacc25fe55d1",
					DestinationName: "bar",
				},
				&structs.Intention{
					ID:              "f004177f-2c28-83b7-4229-eacc25fe55d2",
					DestinationName: "foo",
				},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		list := makeList()
		New(acl.AllowAll(), logger).Filter(list)

		require.Len(t, list.Intentions, 2)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read 1", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			service_prefix "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.Intentions, 1)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.Intentions)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterServices(t *testing.T) {
	t.Parallel()

	// Create some services
	services := structs.Services{
		"service1": []string{},
		"service2": []string{},
		"consul":   []string{},
	}

	// Try permissive filtering.
	filt := New(acl.AllowAll(), nil)
	removed := filt.filterServices(services, nil)
	require.False(t, removed)
	require.Len(t, services, 3)

	// Try restrictive filtering.
	filt = New(acl.DenyAll(), nil)
	removed = filt.filterServices(services, nil)
	require.True(t, removed)
	require.Empty(t, services)
}

func TestACL_filterServiceNodes(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedServiceNodes {
		return &structs.IndexedServiceNodes{
			ServiceNodes: structs.ServiceNodes{
				{
					Node:        "node1",
					ServiceName: "foo",
				},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.ServiceNodes, 1)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read the service, but not the node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.ServiceNodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.ServiceNodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterNodeServices(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedNodeServices {
		return &structs.IndexedNodeServices{
			NodeServices: &structs.NodeServices{
				Node: &structs.Node{
					Node: "node1",
				},
				Services: map[string]*structs.NodeService{
					"foo": {
						ID:      "foo",
						Service: "foo",
					},
				},
			},
		}
	}

	t.Run("nil input", func(t *testing.T) {

		list := &structs.IndexedNodeServices{
			NodeServices: nil,
		}
		New(acl.AllowAll(), logger).Filter(list)

		require.Nil(t, list.NodeServices)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.NodeServices.Services, 1)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read the service, but not the node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Nil(t, list.NodeServices)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("allowed to read the node, but not the service", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.NodeServices.Services)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Nil(t, list.NodeServices)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterNodeServiceList(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedNodeServiceList {
		return &structs.IndexedNodeServiceList{
			NodeServices: structs.NodeServiceList{
				Node: &structs.Node{
					Node: "node1",
				},
				Services: []*structs.NodeService{
					{Service: "foo"},
				},
			},
		}
	}

	t.Run("empty NodeServices", func(t *testing.T) {

		var list structs.IndexedNodeServiceList
		New(acl.AllowAll(), logger).Filter(&list)

		require.Empty(t, list)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.NodeServices.Services, 1)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read the service, but not the node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.NodeServices)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("allowed to read the node, but not the service", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.NotEmpty(t, list.NodeServices.Node)
		require.Empty(t, list.NodeServices.Services)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.NodeServices)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterGatewayServices(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedGatewayServices {
		return &structs.IndexedGatewayServices{
			Services: structs.GatewayServices{
				{Service: structs.ServiceName{Name: "foo"}},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.Services, 1)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.Services)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterCheckServiceNodes(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedCheckServiceNodes {
		return &structs.IndexedCheckServiceNodes{
			Nodes: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node: "node1",
					},
					Service: &structs.NodeService{
						ID:      "foo",
						Service: "foo",
					},
					Checks: structs.HealthChecks{
						{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "foo",
						},
					},
				},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.Nodes, 1)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read the service, but not the node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.Nodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("allowed to read the node, but not the service", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.Nodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.Nodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterPreparedQueryExecuteResponse(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.PreparedQueryExecuteResponse {
		return &structs.PreparedQueryExecuteResponse{
			Nodes: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node: "node1",
					},
					Service: &structs.NodeService{
						ID:      "foo",
						Service: "foo",
					},
					Checks: structs.HealthChecks{
						{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "foo",
						},
					},
				},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.Nodes, 1)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read the service, but not the node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.Nodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("allowed to read the node, but not the service", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.Nodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.Nodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterServiceTopology(t *testing.T) {
	t.Parallel()
	// Create some nodes.
	fill := func() structs.ServiceTopology {
		return structs.ServiceTopology{
			Upstreams: structs.CheckServiceNodes{
				structs.CheckServiceNode{
					Node: &structs.Node{
						Node: "node1",
					},
					Service: &structs.NodeService{
						ID:      "foo",
						Service: "foo",
					},
					Checks: structs.HealthChecks{
						&structs.HealthCheck{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "foo",
						},
					},
				},
			},
			Downstreams: structs.CheckServiceNodes{
				structs.CheckServiceNode{
					Node: &structs.Node{
						Node: "node2",
					},
					Service: &structs.NodeService{
						ID:      "bar",
						Service: "bar",
					},
					Checks: structs.HealthChecks{
						&structs.HealthCheck{
							Node:        "node2",
							CheckID:     "check1",
							ServiceName: "bar",
						},
					},
				},
			},
		}
	}
	original := fill()

	t.Run("allow all without permissions", func(t *testing.T) {
		topo := fill()
		f := New(acl.AllowAll(), nil)

		filtered := f.filterServiceTopology(&topo)
		if filtered {
			t.Fatalf("should not have been filtered")
		}
		assert.Equal(t, original, topo)
	})

	t.Run("deny all without permissions", func(t *testing.T) {
		topo := fill()
		f := New(acl.DenyAll(), nil)

		filtered := f.filterServiceTopology(&topo)
		if !filtered {
			t.Fatalf("should have been marked as filtered")
		}
		assert.Len(t, topo.Upstreams, 0)
		assert.Len(t, topo.Upstreams, 0)
	})

	t.Run("only upstream permissions", func(t *testing.T) {
		rules := `
node "node1" {
  policy = "read"
}
service "foo" {
  policy = "read"
}`
		policy, err := acl.NewPolicyFromSource(rules, nil, nil)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		topo := fill()
		f := New(perms, nil)

		filtered := f.filterServiceTopology(&topo)
		if !filtered {
			t.Fatalf("should have been marked as filtered")
		}
		assert.Equal(t, original.Upstreams, topo.Upstreams)
		assert.Len(t, topo.Downstreams, 0)
	})

	t.Run("only downstream permissions", func(t *testing.T) {
		rules := `
node "node2" {
  policy = "read"
}
service "bar" {
  policy = "read"
}`
		policy, err := acl.NewPolicyFromSource(rules, nil, nil)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		topo := fill()
		f := New(perms, nil)

		filtered := f.filterServiceTopology(&topo)
		if !filtered {
			t.Fatalf("should have been marked as filtered")
		}
		assert.Equal(t, original.Downstreams, topo.Downstreams)
		assert.Len(t, topo.Upstreams, 0)
	})

	t.Run("upstream and downstream permissions", func(t *testing.T) {
		rules := `
node "node1" {
  policy = "read"
}
service "foo" {
  policy = "read"
}
node "node2" {
  policy = "read"
}
service "bar" {
  policy = "read"
}`
		policy, err := acl.NewPolicyFromSource(rules, nil, nil)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		topo := fill()
		f := New(perms, nil)

		filtered := f.filterServiceTopology(&topo)
		if filtered {
			t.Fatalf("should not have been filtered")
		}

		original := fill()
		assert.Equal(t, original, topo)
	})
}

func TestACL_filterCoordinates(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedCoordinates {
		return &structs.IndexedCoordinates{
			Coordinates: structs.Coordinates{
				{Node: "node1", Coord: coordinate.NewCoordinate(coordinate.DefaultConfig())},
				{Node: "node2", Coord: coordinate.NewCoordinate(coordinate.DefaultConfig())},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		list := makeList()
		New(acl.AllowAll(), logger).Filter(list)

		require.Len(t, list.Coordinates, 2)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read one node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.Coordinates, 1)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.Coordinates)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterSessions(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedSessions {
		return &structs.IndexedSessions{
			Sessions: structs.Sessions{
				{Node: "foo"},
				{Node: "bar"},
			},
		}
	}

	t.Run("all allowed", func(t *testing.T) {

		list := makeList()
		New(acl.AllowAll(), logger).Filter(list)

		require.Len(t, list.Sessions, 2)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("just one node's sessions allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			session "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.Sessions, 1)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.Sessions)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterNodeDump(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedNodeDump {
		return &structs.IndexedNodeDump{
			Dump: structs.NodeDump{
				{
					Node: "node1",
					Services: []*structs.NodeService{
						{
							ID:      "foo",
							Service: "foo",
						},
					},
					Checks: []*structs.HealthCheck{
						{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "foo",
						},
					},
				},
			},
			ImportedDump: structs.NodeDump{
				{
					// The node and service names are intentionally the same to ensure that
					// local permissions for the same names do not allow reading imports.
					Node:     "node1",
					PeerName: "cluster-02",
					Services: []*structs.NodeService{
						{
							ID:       "foo",
							Service:  "foo",
							PeerName: "cluster-02",
						},
					},
					Checks: []*structs.HealthCheck{
						{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "foo",
							PeerName:    "cluster-02",
						},
					},
				},
			},
		}
	}
	type testCase struct {
		authzFn func() acl.Authorizer
		expect  *structs.IndexedNodeDump
	}

	run := func(t *testing.T, tc testCase) {
		authz := tc.authzFn()

		list := makeList()
		New(authz, logger).Filter(list)

		require.Equal(t, tc.expect, list)
	}

	tt := map[string]testCase{
		"denied": {
			authzFn: func() acl.Authorizer {
				return acl.DenyAll()
			},
			expect: &structs.IndexedNodeDump{
				Dump:         structs.NodeDump{},
				ImportedDump: structs.NodeDump{},
				QueryMeta:    structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"can read local service but not the node": {
			authzFn: func() acl.Authorizer {
				policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)

				return authz
			},
			expect: &structs.IndexedNodeDump{
				Dump:         structs.NodeDump{},
				ImportedDump: structs.NodeDump{},
				QueryMeta:    structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"can read the local node but not the service": {
			authzFn: func() acl.Authorizer {
				policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)

				return authz
			},
			expect: &structs.IndexedNodeDump{
				Dump: structs.NodeDump{
					{
						Node:     "node1",
						Services: []*structs.NodeService{},
						Checks:   structs.HealthChecks{},
					},
				},
				ImportedDump: structs.NodeDump{},
				QueryMeta:    structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"can read local data": {
			authzFn: func() acl.Authorizer {
				policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)

				return authz
			},
			expect: &structs.IndexedNodeDump{
				Dump: structs.NodeDump{
					{
						Node: "node1",
						Services: []*structs.NodeService{
							{
								ID:      "foo",
								Service: "foo",
							},
						},
						Checks: []*structs.HealthCheck{
							{
								Node:        "node1",
								CheckID:     "check1",
								ServiceName: "foo",
							},
						},
					},
				},
				ImportedDump: structs.NodeDump{},
				QueryMeta:    structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"can read imported service but not the node": {
			authzFn: func() acl.Authorizer {
				// Wildcard service read also grants read to imported services.
				policy, err := acl.NewPolicyFromSource(`
			service "" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)

				return authz
			},
			expect: &structs.IndexedNodeDump{
				Dump:         structs.NodeDump{},
				ImportedDump: structs.NodeDump{},
				QueryMeta:    structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"can read the imported node but not the service": {
			authzFn: func() acl.Authorizer {
				// Wildcard node read also grants read to imported nodes.
				policy, err := acl.NewPolicyFromSource(`
			node "" {
			  policy = "read"
			},
			node_prefix "" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)

				return authz
			},
			expect: &structs.IndexedNodeDump{
				Dump: structs.NodeDump{
					{
						Node:     "node1",
						Services: []*structs.NodeService{},
						Checks:   structs.HealthChecks{},
					},
				},
				ImportedDump: structs.NodeDump{
					{
						Node:     "node1",
						PeerName: "cluster-02",
						Services: []*structs.NodeService{},
						Checks:   structs.HealthChecks{},
					},
				},
				QueryMeta: structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"can read all data": {
			authzFn: func() acl.Authorizer {
				policy, err := acl.NewPolicyFromSource(`
			service "" {
			  policy = "read"
			},
            service_prefix "" {
			  policy = "read"
			}
			node "" {
			  policy = "read"
			},
            node_prefix "" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)

				return authz
			},
			expect: &structs.IndexedNodeDump{
				Dump: structs.NodeDump{
					{
						Node: "node1",
						Services: []*structs.NodeService{
							{
								ID:      "foo",
								Service: "foo",
							},
						},
						Checks: []*structs.HealthCheck{
							{
								Node:        "node1",
								CheckID:     "check1",
								ServiceName: "foo",
							},
						},
					},
				},
				ImportedDump: structs.NodeDump{
					{
						Node:     "node1",
						PeerName: "cluster-02",
						Services: []*structs.NodeService{
							{
								ID:       "foo",
								Service:  "foo",
								PeerName: "cluster-02",
							},
						},
						Checks: []*structs.HealthCheck{
							{
								Node:        "node1",
								CheckID:     "check1",
								ServiceName: "foo",
								PeerName:    "cluster-02",
							},
						},
					},
				},
				QueryMeta: structs.QueryMeta{ResultsFilteredByACLs: false},
			},
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestACL_filterNodes(t *testing.T) {
	t.Parallel()

	// Create a nodes list.
	nodes := structs.Nodes{
		&structs.Node{
			Node: "foo",
		},
		&structs.Node{
			Node: "bar",
		},
	}

	// Try permissive filtering.
	filt := New(acl.AllowAll(), nil)
	removed := filt.filterNodes(&nodes)
	require.False(t, removed)
	require.Len(t, nodes, 2)

	// Try restrictive filtering
	filt = New(acl.DenyAll(), nil)
	removed = filt.filterNodes(&nodes)
	require.True(t, removed)
	require.Len(t, nodes, 0)
}

func TestACL_filterIndexedNodesWithGateways(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedNodesWithGateways {
		return &structs.IndexedNodesWithGateways{
			Nodes: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node: "node1",
					},
					Service: &structs.NodeService{
						ID:      "foo",
						Service: "foo",
					},
					Checks: structs.HealthChecks{
						{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "foo",
						},
					},
				},
			},
			Gateways: structs.GatewayServices{
				{Service: structs.ServiceNameFromString("foo")},
				{Service: structs.ServiceNameFromString("bar")},
			},
			ImportedNodes: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:     "imported-node",
						PeerName: "cluster-02",
					},
					Service: &structs.NodeService{
						ID:       "zip",
						Service:  "zip",
						PeerName: "cluster-02",
					},
					Checks: structs.HealthChecks{
						{
							Node:        "node1",
							CheckID:     "check1",
							ServiceName: "zip",
							PeerName:    "cluster-02",
						},
					},
				},
			},
		}
	}

	type testCase struct {
		authzFn func() acl.Authorizer
		expect  *structs.IndexedNodesWithGateways
	}

	run := func(t *testing.T, tc testCase) {
		authz := tc.authzFn()

		list := makeList()
		New(authz, logger).Filter(list)

		require.Equal(t, tc.expect, list)
	}

	tt := map[string]testCase{
		"not filtered": {
			authzFn: func() acl.Authorizer {
				policy, err := acl.NewPolicyFromSource(`
			service "baz" {
				policy = "write"
			}
			service "foo" {
			  policy = "read"
			}
			service "bar" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)
				return authz
			},
			expect: &structs.IndexedNodesWithGateways{
				Nodes: structs.CheckServiceNodes{
					{
						Node: &structs.Node{
							Node: "node1",
						},
						Service: &structs.NodeService{
							ID:      "foo",
							Service: "foo",
						},
						Checks: structs.HealthChecks{
							{
								Node:        "node1",
								CheckID:     "check1",
								ServiceName: "foo",
							},
						},
					},
				},
				Gateways: structs.GatewayServices{
					{Service: structs.ServiceNameFromString("foo")},
					{Service: structs.ServiceNameFromString("bar")},
				},
				// Service write to "bar" allows reading all imported services
				ImportedNodes: structs.CheckServiceNodes{
					{
						Node: &structs.Node{
							Node:     "imported-node",
							PeerName: "cluster-02",
						},
						Service: &structs.NodeService{
							ID:       "zip",
							Service:  "zip",
							PeerName: "cluster-02",
						},
						Checks: structs.HealthChecks{
							{
								Node:        "node1",
								CheckID:     "check1",
								ServiceName: "zip",
								PeerName:    "cluster-02",
							},
						},
					},
				},
				QueryMeta: structs.QueryMeta{ResultsFilteredByACLs: false},
			},
		},
		"not allowed to read the node": {
			authzFn: func() acl.Authorizer {
				policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			service "bar" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)
				return authz
			},
			expect: &structs.IndexedNodesWithGateways{
				Nodes: structs.CheckServiceNodes{},
				Gateways: structs.GatewayServices{
					{Service: structs.ServiceNameFromString("foo")},
					{Service: structs.ServiceNameFromString("bar")},
				},
				ImportedNodes: structs.CheckServiceNodes{},
				QueryMeta:     structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"not allowed to read the service": {
			authzFn: func() acl.Authorizer {
				policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
			service "bar" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)
				return authz
			},
			expect: &structs.IndexedNodesWithGateways{
				Nodes: structs.CheckServiceNodes{},
				Gateways: structs.GatewayServices{
					{Service: structs.ServiceNameFromString("bar")},
				},
				ImportedNodes: structs.CheckServiceNodes{},
				QueryMeta:     structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"not allowed to read the other gateway service": {
			authzFn: func() acl.Authorizer {
				policy, err := acl.NewPolicyFromSource(`
			service "foo" {
			  policy = "read"
			}
			node "node1" {
			  policy = "read"
			}
		`, nil, nil)
				require.NoError(t, err)

				authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
				require.NoError(t, err)
				return authz
			},
			expect: &structs.IndexedNodesWithGateways{
				Nodes: structs.CheckServiceNodes{
					{
						Node: &structs.Node{
							Node: "node1",
						},
						Service: &structs.NodeService{
							ID:      "foo",
							Service: "foo",
						},
						Checks: structs.HealthChecks{
							{
								Node:        "node1",
								CheckID:     "check1",
								ServiceName: "foo",
							},
						},
					},
				},
				Gateways: structs.GatewayServices{
					{Service: structs.ServiceNameFromString("foo")},
				},
				ImportedNodes: structs.CheckServiceNodes{},
				QueryMeta:     structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
		"denied": {
			authzFn: acl.DenyAll,
			expect: &structs.IndexedNodesWithGateways{
				Nodes:         structs.CheckServiceNodes{},
				Gateways:      structs.GatewayServices{},
				ImportedNodes: structs.CheckServiceNodes{},
				QueryMeta:     structs.QueryMeta{ResultsFilteredByACLs: true},
			},
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestACL_filterIndexedServiceDump(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedServiceDump {
		return &structs.IndexedServiceDump{
			Dump: structs.ServiceDump{
				{
					Node: &structs.Node{
						Node: "node1",
					},
					Service: &structs.NodeService{
						Service: "foo",
					},
					GatewayService: &structs.GatewayService{
						Service: structs.ServiceNameFromString("foo"),
						Gateway: structs.ServiceNameFromString("foo-gateway"),
					},
				},
				// No node information.
				{
					GatewayService: &structs.GatewayService{
						Service: structs.ServiceNameFromString("bar"),
						Gateway: structs.ServiceNameFromString("bar-gateway"),
					},
				},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
			service_prefix "foo" {
			  policy = "read"
			}
			service_prefix "bar" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.Dump, 2)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("not allowed to access node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service_prefix "foo" {
			  policy = "read"
			}
			service_prefix "bar" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.Dump, 1)
		require.Equal(t, "bar", list.Dump[0].GatewayService.Service.Name)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("not allowed to access service", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
			service "foo-gateway" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.Dump)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("not allowed to access gateway", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node "node1" {
			  policy = "read"
			}
			service "foo" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.Dump)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterDatacenterCheckServiceNodes(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.DatacenterIndexedCheckServiceNodes {
		t.Helper()

		node := func(dc, node, ip string) structs.CheckServiceNode {
			t.Helper()

			id, err := uuid.GenerateUUID()
			require.NoError(t, err)

			return structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         types.NodeID(id),
					Node:       node,
					Datacenter: dc,
					Address:    ip,
				},
				Service: &structs.NodeService{
					ID:      "mesh-gateway",
					Service: "mesh-gateway",
					Kind:    structs.ServiceKindMeshGateway,
					Port:    9999,
					Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
				},
				Checks: []*structs.HealthCheck{
					{
						Name:      "web connectivity",
						Status:    api.HealthPassing,
						ServiceID: "mesh-gateway",
					},
				},
			}
		}

		return &structs.DatacenterIndexedCheckServiceNodes{
			DatacenterNodes: map[string]structs.CheckServiceNodes{
				"dc1": []structs.CheckServiceNode{
					node("dc1", "gateway1a", "1.2.3.4"),
					node("dc1", "gateway2a", "4.3.2.1"),
				},
				"dc2": []structs.CheckServiceNode{
					node("dc2", "gateway1b", "5.6.7.8"),
					node("dc2", "gateway2b", "8.7.6.5"),
				},
			},
		}
	}

	t.Run("allowed", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node_prefix "" {
			  policy = "read"
			}
			service_prefix "" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Len(t, list.DatacenterNodes["dc1"], 2)
		require.Len(t, list.DatacenterNodes["dc2"], 2)
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("allowed to read the service, but not the node", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			service_prefix "" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.DatacenterNodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("allowed to read the node, but not the service", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			node_prefix "" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		require.Empty(t, list.DatacenterNodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("denied", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.DatacenterNodes)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_redactPreparedQueryTokens(t *testing.T) {
	t.Parallel()
	query := &structs.PreparedQuery{
		ID:    "f004177f-2c28-83b7-4229-eacc25fe55d1",
		Token: "root",
	}

	expected := &structs.PreparedQuery{
		ID:    "f004177f-2c28-83b7-4229-eacc25fe55d1",
		Token: "root",
	}

	// Try permissive filtering with a management token. This will allow the
	// embedded token to be seen.
	filt := New(acl.ManageAll(), nil)
	filt.redactPreparedQueryTokens(&query)
	if !reflect.DeepEqual(query, expected) {
		t.Fatalf("bad: %#v", &query)
	}

	// Hang on to the entry with a token, which needs to survive the next
	// operation.
	original := query

	// Now try permissive filtering with a client token, which should cause
	// the embedded token to get redacted.
	filt = New(acl.AllowAll(), nil)
	filt.redactPreparedQueryTokens(&query)
	expected.Token = RedactedToken
	if !reflect.DeepEqual(query, expected) {
		t.Fatalf("bad: %#v", *query)
	}

	// Make sure that the original object didn't lose its token.
	if original.Token != "root" {
		t.Fatalf("bad token: %s", original.Token)
	}
}

func TestFilterACL_redactTokenSecret(t *testing.T) {
	t.Parallel()

	token := &structs.ACLToken{
		AccessorID: "6a5e25b3-28f2-4085-9012-c3fb754314d1",
		SecretID:   "6a5e25b3-28f2-4085-9012-c3fb754314d1",
	}

	New(policy(t, `acl = "write"`), nil).Filter(&token)
	require.Equal(t, "6a5e25b3-28f2-4085-9012-c3fb754314d1", token.SecretID)

	New(policy(t, `acl = "read"`), nil).Filter(&token)
	require.Equal(t, RedactedToken, token.SecretID)
}

func TestFilterACL_redactTokenSecrets(t *testing.T) {
	t.Parallel()

	tokens := structs.ACLTokens{
		&structs.ACLToken{
			AccessorID: "6a5e25b3-28f2-4085-9012-c3fb754314d1",
			SecretID:   "6a5e25b3-28f2-4085-9012-c3fb754314d1",
		},
	}

	New(policy(t, `acl = "write"`), nil).Filter(&tokens)
	require.Equal(t, "6a5e25b3-28f2-4085-9012-c3fb754314d1", tokens[0].SecretID)

	New(policy(t, `acl = "read"`), nil).Filter(&tokens)
	require.Equal(t, RedactedToken, tokens[0].SecretID)
}

func TestACL_filterPreparedQueries(t *testing.T) {
	t.Parallel()

	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedPreparedQueries {
		return &structs.IndexedPreparedQueries{
			Queries: structs.PreparedQueries{
				{ID: "f004177f-2c28-83b7-4229-eacc25fe55d1"},
				{
					ID:   "f004177f-2c28-83b7-4229-eacc25fe55d2",
					Name: "query-with-no-token",
				},
				{
					ID:    "f004177f-2c28-83b7-4229-eacc25fe55d3",
					Name:  "query-with-a-token",
					Token: "root",
				},
			},
		}
	}

	t.Run("management token", func(t *testing.T) {

		list := makeList()
		New(acl.ManageAll(), logger).Filter(list)

		// Check we get the un-named query.
		require.Len(t, list.Queries, 3)

		// Check we get the un-redacted token.
		require.Equal(t, "root", list.Queries[2].Token)

		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("permissive filtering", func(t *testing.T) {

		list := makeList()
		queryWithToken := list.Queries[2]

		New(acl.AllowAll(), logger).Filter(list)

		// Check the un-named query is filtered out.
		require.Len(t, list.Queries, 2)

		// Check the token is redacted.
		require.Equal(t, RedactedToken, list.Queries[1].Token)

		// Check the original object is unmodified.
		require.Equal(t, "root", queryWithToken.Token)

		// ResultsFilteredByACLs should not include un-named queries, which are only
		// readable by a management token.
		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("limited access", func(t *testing.T) {

		policy, err := acl.NewPolicyFromSource(`
			query "query-with-a-token" {
			  policy = "read"
			}
		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		list := makeList()
		New(authz, logger).Filter(list)

		// Check we only get the query we have access to.
		require.Len(t, list.Queries, 1)

		// Check the token is redacted.
		require.Equal(t, RedactedToken, list.Queries[0].Token)

		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("restrictive filtering", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.Empty(t, list.Queries)
		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestACL_filterServiceList(t *testing.T) {
	logger := hclog.NewNullLogger()

	makeList := func() *structs.IndexedServiceList {
		return &structs.IndexedServiceList{
			Services: structs.ServiceList{
				{Name: "foo"},
				{Name: "bar"},
			},
		}
	}

	t.Run("permissive filtering", func(t *testing.T) {

		list := makeList()
		New(acl.AllowAll(), logger).Filter(list)

		require.False(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
		require.Len(t, list.Services, 2)
	})

	t.Run("restrictive filtering", func(t *testing.T) {

		list := makeList()
		New(acl.DenyAll(), logger).Filter(list)

		require.True(t, list.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
		require.Empty(t, list.Services)
	})
}

func TestACL_unhandledFilterType(t *testing.T) {
	t.Parallel()

	filter := New(acl.AllowAll(), nil)

	require.Panics(t, func() {
		filter.Filter(&structs.HealthCheck{})
	})
}

func policy(t *testing.T, hcl string) acl.Authorizer {
	t.Helper()

	policy, err := acl.NewPolicyFromSource(hcl, nil, nil)
	require.NoError(t, err)

	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	return authz
}
