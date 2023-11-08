// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

func TestHealthSnapshot(t *testing.T) {
	type testcase struct {
		name   string
		in     []structs.CheckServiceNode
		expect *healthSnapshot
	}

	entMeta := acl.DefaultEnterpriseMeta()

	run := func(t *testing.T, tc testcase) {
		snap := newHealthSnapshot(tc.in, entMeta.PartitionOrEmpty(), "my-peer")
		require.Equal(t, tc.expect, snap)
	}

	newNode := func(id, name, peerName string) *structs.Node {
		return &structs.Node{
			ID:        types.NodeID(id),
			Node:      name,
			Partition: entMeta.PartitionOrEmpty(),
			PeerName:  peerName,
		}
	}

	newService := func(id string, port int, peerName string) *structs.NodeService {
		return &structs.NodeService{
			ID:             id,
			Service:        "xyz",
			EnterpriseMeta: *entMeta,
			PeerName:       peerName,
			Port:           port,
		}
	}

	newCheck := func(node, svcID, peerName string) *structs.HealthCheck {
		return &structs.HealthCheck{
			Node:           node,
			ServiceID:      svcID,
			ServiceName:    "xyz",
			CheckID:        types.CheckID(svcID + ":check"),
			Name:           "check",
			EnterpriseMeta: *entMeta,
			PeerName:       peerName,
			Status:         "passing",
		}
	}

	cases := []testcase{
		{
			name: "single",
			in: []structs.CheckServiceNode{
				{
					Node:    newNode("abc-123", "abc", ""),
					Service: newService("xyz-123", 8080, ""),
					Checks: structs.HealthChecks{
						newCheck("abc", "xyz-123", ""),
					},
				},
			},
			expect: &healthSnapshot{
				Nodes: map[string]*nodeSnapshot{
					"abc": {
						Node: newNode("abc-123", "abc", "my-peer"),
						Services: map[structs.ServiceID]*serviceSnapshot{
							structs.NewServiceID("xyz-123", nil): {
								Service: newService("xyz-123", 8080, "my-peer"),
								Checks: map[types.CheckID]*structs.HealthCheck{
									"xyz-123:check": newCheck("abc", "xyz-123", "my-peer"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple",
			in: []structs.CheckServiceNode{
				{
					Node:    newNode("", "abc", ""),
					Service: newService("xyz-123", 8080, ""),
					Checks: structs.HealthChecks{
						newCheck("abc", "xyz-123", ""),
					},
				},
				{
					Node:    newNode("", "abc", ""),
					Service: newService("xyz-789", 8181, ""),
					Checks: structs.HealthChecks{
						newCheck("abc", "xyz-789", ""),
					},
				},
				{
					Node:    newNode("def-456", "def", ""),
					Service: newService("xyz-456", 9090, ""),
					Checks: structs.HealthChecks{
						newCheck("def", "xyz-456", ""),
					},
				},
			},
			expect: &healthSnapshot{
				Nodes: map[string]*nodeSnapshot{
					"abc": {
						Node: newNode("", "abc", "my-peer"),
						Services: map[structs.ServiceID]*serviceSnapshot{
							structs.NewServiceID("xyz-123", nil): {
								Service: newService("xyz-123", 8080, "my-peer"),
								Checks: map[types.CheckID]*structs.HealthCheck{
									"xyz-123:check": newCheck("abc", "xyz-123", "my-peer"),
								},
							},
							structs.NewServiceID("xyz-789", nil): {
								Service: newService("xyz-789", 8181, "my-peer"),
								Checks: map[types.CheckID]*structs.HealthCheck{
									"xyz-789:check": newCheck("abc", "xyz-789", "my-peer"),
								},
							},
						},
					},
					"def": {
						Node: newNode("def-456", "def", "my-peer"),
						Services: map[structs.ServiceID]*serviceSnapshot{
							structs.NewServiceID("xyz-456", nil): {
								Service: newService("xyz-456", 9090, "my-peer"),
								Checks: map[types.CheckID]*structs.HealthCheck{
									"xyz-456:check": newCheck("def", "xyz-456", "my-peer"),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
