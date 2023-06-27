// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func TestCatalogOverview(t *testing.T) {
	cases := []struct {
		name     string
		nodes    []*structs.Node
		services []*structs.ServiceNode
		checks   []*structs.HealthCheck
		expected structs.CatalogSummary
	}{
		{
			name:     "empty",
			expected: structs.CatalogSummary{},
		},
		{
			name: "one node with no checks",
			nodes: []*structs.Node{
				{Node: "node1"},
			},
			expected: structs.CatalogSummary{
				Nodes: []structs.HealthSummary{
					{Total: 1, Passing: 1},
				},
			},
		},
		{
			name: "one service with no checks",
			services: []*structs.ServiceNode{
				{Node: "node1", ServiceName: "service1"},
			},
			expected: structs.CatalogSummary{
				Services: []structs.HealthSummary{
					{Name: "service1", Total: 1, Passing: 1},
				},
			},
		},
		{
			name: "three nodes with node checks",
			nodes: []*structs.Node{
				{Node: "node1"},
				{Node: "node2"},
				{Node: "node3"},
			},
			checks: []*structs.HealthCheck{
				{Node: "node1", Name: "check1", CheckID: "check1", Status: api.HealthPassing},
				{Node: "node2", Name: "check1", CheckID: "check1", Status: api.HealthWarning},
				{Node: "node3", Name: "check1", CheckID: "check1", Status: api.HealthCritical},
			},
			expected: structs.CatalogSummary{
				Nodes: []structs.HealthSummary{
					{Total: 3, Passing: 1, Warning: 1, Critical: 1},
				},
				Checks: []structs.HealthSummary{
					{Name: "check1", Total: 3, Passing: 1, Warning: 1, Critical: 1},
				},
			},
		},
		{
			name: "three instances of one service with checks",
			nodes: []*structs.Node{
				{Node: "node1"},
			},
			services: []*structs.ServiceNode{
				{Node: "node1", ServiceName: "service1", ServiceID: "id1"},
				{Node: "node1", ServiceName: "service1", ServiceID: "id2"},
				{Node: "node1", ServiceName: "service1", ServiceID: "id3"},
			},
			checks: []*structs.HealthCheck{
				{Node: "node1", Name: "check1", CheckID: "check1", ServiceID: "id1", Status: api.HealthPassing},
				{Node: "node1", Name: "check1", CheckID: "check2", ServiceID: "id2", Status: api.HealthWarning},
				{Node: "node1", Name: "check1", CheckID: "check3", ServiceID: "id3", Status: api.HealthCritical},
			},
			expected: structs.CatalogSummary{
				Nodes: []structs.HealthSummary{
					{Total: 1, Passing: 1},
				},
				Services: []structs.HealthSummary{
					{Name: "service1", Total: 3, Passing: 1, Warning: 1, Critical: 1},
				},
				Checks: []structs.HealthSummary{
					{Name: "check1", Total: 3, Passing: 1, Warning: 1, Critical: 1},
				},
			},
		},
		{
			name: "three instances of different services with checks",
			nodes: []*structs.Node{
				{Node: "node1"},
			},
			services: []*structs.ServiceNode{
				{Node: "node1", ServiceName: "service1", ServiceID: "id1"},
				{Node: "node1", ServiceName: "service2", ServiceID: "id2"},
				{Node: "node1", ServiceName: "service3", ServiceID: "id3"},
			},
			checks: []*structs.HealthCheck{
				{Node: "node1", Name: "check1", CheckID: "check1", ServiceID: "id1", Status: api.HealthPassing},
				{Node: "node1", Name: "check1", CheckID: "check2", ServiceID: "id2", Status: api.HealthWarning},
				{Node: "node1", Name: "check1", CheckID: "check3", ServiceID: "id3", Status: api.HealthCritical},
			},
			expected: structs.CatalogSummary{
				Nodes: []structs.HealthSummary{
					{Total: 1, Passing: 1},
				},
				Services: []structs.HealthSummary{
					{Name: "service1", Total: 1, Passing: 1},
					{Name: "service2", Total: 1, Warning: 1},
					{Name: "service3", Total: 1, Critical: 1},
				},
				Checks: []structs.HealthSummary{
					{Name: "check1", Total: 3, Passing: 1, Warning: 1, Critical: 1},
				},
			},
		},
		{
			name: "many instances of the same check",
			checks: []*structs.HealthCheck{
				{Name: "check1", CheckID: "check1", Status: api.HealthPassing},
				{Name: "check1", CheckID: "check2", Status: api.HealthWarning},
				{Name: "check1", CheckID: "check3", Status: api.HealthCritical},
				{Name: "check1", CheckID: "check4", Status: api.HealthPassing},
				{Name: "check1", CheckID: "check5", Status: api.HealthCritical},
			},
			expected: structs.CatalogSummary{
				Checks: []structs.HealthSummary{
					{Name: "check1", Total: 5, Passing: 2, Warning: 1, Critical: 2},
				},
			},
		},
		{
			name: "three different checks",
			checks: []*structs.HealthCheck{
				{Name: "check1", CheckID: "check1", Status: api.HealthPassing},
				{Name: "check2", CheckID: "check2", Status: api.HealthWarning},
				{Name: "check3", CheckID: "check3", Status: api.HealthCritical},
			},
			expected: structs.CatalogSummary{
				Checks: []structs.HealthSummary{
					{Name: "check1", Total: 1, Passing: 1},
					{Name: "check2", Total: 1, Warning: 1},
					{Name: "check3", Total: 1, Critical: 1},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := getCatalogOverview(&structs.CatalogContents{
				Nodes:    tc.nodes,
				Services: tc.services,
				Checks:   tc.checks,
			})
			require.ElementsMatch(t, tc.expected.Nodes, summary.Nodes)
			require.ElementsMatch(t, tc.expected.Services, summary.Services)
			require.ElementsMatch(t, tc.expected.Checks, summary.Checks)
		})
	}
}
