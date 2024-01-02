// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testClusterID is the Consul cluster ID for testing.
//
// NOTE: this is explicitly duplicated from agent/connect:TestClusterID
const testClusterID = "11111111-2222-3333-4444-555555555555"

func TestAPI_DiscoveryChain_Get(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	config_entries := c.ConfigEntries()
	discoverychain := c.DiscoveryChain()

	s.WaitForActiveCARoot(t)

	require.True(t, t.Run("read default chain", func(t *testing.T) {
		resp, _, err := discoverychain.Get("web", nil, nil)
		require.NoError(t, err)

		expect := &DiscoveryChainResponse{
			Chain: &CompiledDiscoveryChain{
				ServiceName: "web",
				Namespace:   "default",
				Datacenter:  "dc1",
				Protocol:    "tcp",
				Default:     true,
				StartNode:   "resolver:web.default.default.dc1",
				Nodes: map[string]*DiscoveryGraphNode{
					"resolver:web.default.default.dc1": {
						Type: DiscoveryGraphNodeTypeResolver,
						Name: "web.default.default.dc1",
						Resolver: &DiscoveryResolver{
							Default:        true,
							ConnectTimeout: 5 * time.Second,
							Target:         "web.default.default.dc1",
						},
					},
				},
				Targets: map[string]*DiscoveryTarget{
					"web.default.default.dc1": {
						ID:             "web.default.default.dc1",
						Service:        "web",
						Namespace:      "default",
						Datacenter:     "dc1",
						ConnectTimeout: 5 * time.Second,
						SNI:            "web.default.dc1.internal." + testClusterID + ".consul",
						Name:           "web.default.dc1.internal." + testClusterID + ".consul",
					},
				},
			},
		}
		require.Equal(t, expect, resp)
	}))

	require.True(t, t.Run("read default chain; evaluate in dc2", func(t *testing.T) {
		opts := &DiscoveryChainOptions{
			EvaluateInDatacenter: "dc2",
		}
		resp, _, err := discoverychain.Get("web", opts, nil)
		require.NoError(t, err)

		expect := &DiscoveryChainResponse{
			Chain: &CompiledDiscoveryChain{
				ServiceName: "web",
				Namespace:   "default",
				Datacenter:  "dc2",
				Protocol:    "tcp",
				Default:     true,
				StartNode:   "resolver:web.default.default.dc2",
				Nodes: map[string]*DiscoveryGraphNode{
					"resolver:web.default.default.dc2": {
						Type: DiscoveryGraphNodeTypeResolver,
						Name: "web.default.default.dc2",
						Resolver: &DiscoveryResolver{
							Default:        true,
							ConnectTimeout: 5 * time.Second,
							Target:         "web.default.default.dc2",
						},
					},
				},
				Targets: map[string]*DiscoveryTarget{
					"web.default.default.dc2": {
						ID:             "web.default.default.dc2",
						Service:        "web",
						Namespace:      "default",
						Datacenter:     "dc2",
						ConnectTimeout: 5 * time.Second,
						SNI:            "web.default.dc2.internal." + testClusterID + ".consul",
						Name:           "web.default.dc2.internal." + testClusterID + ".consul",
					},
				},
			},
		}
		require.Equal(t, expect, resp)
	}))

	{ // Now create one config entry.
		ok, _, err := config_entries.Set(&ServiceResolverConfigEntry{
			Kind:           ServiceResolver,
			Name:           "web",
			ConnectTimeout: 33 * time.Second,
		}, nil)
		require.NoError(t, err)
		require.True(t, ok)
	}

	require.True(t, t.Run("read modified chain", func(t *testing.T) {
		resp, _, err := discoverychain.Get("web", nil, nil)
		require.NoError(t, err)

		expect := &DiscoveryChainResponse{
			Chain: &CompiledDiscoveryChain{
				ServiceName: "web",
				Namespace:   "default",
				Datacenter:  "dc1",
				Protocol:    "tcp",
				StartNode:   "resolver:web.default.default.dc1",
				Nodes: map[string]*DiscoveryGraphNode{
					"resolver:web.default.default.dc1": {
						Type: DiscoveryGraphNodeTypeResolver,
						Name: "web.default.default.dc1",
						Resolver: &DiscoveryResolver{
							ConnectTimeout: 33 * time.Second,
							Target:         "web.default.default.dc1",
						},
					},
				},
				Targets: map[string]*DiscoveryTarget{
					"web.default.default.dc1": {
						ID:             "web.default.default.dc1",
						Service:        "web",
						Namespace:      "default",
						Datacenter:     "dc1",
						ConnectTimeout: 33 * time.Second,
						SNI:            "web.default.dc1.internal." + testClusterID + ".consul",
						Name:           "web.default.dc1.internal." + testClusterID + ".consul",
					},
				},
			},
		}
		require.Equal(t, expect, resp)
	}))

	require.True(t, t.Run("read modified chain in dc2 with overrides", func(t *testing.T) {
		opts := &DiscoveryChainOptions{
			EvaluateInDatacenter: "dc2",
			OverrideMeshGateway: MeshGatewayConfig{
				Mode: MeshGatewayModeLocal,
			},
			OverrideProtocol:       "grpc",
			OverrideConnectTimeout: 22 * time.Second,
		}
		resp, _, err := discoverychain.Get("web", opts, nil)
		require.NoError(t, err)

		expect := &DiscoveryChainResponse{
			Chain: &CompiledDiscoveryChain{
				ServiceName:       "web",
				Namespace:         "default",
				Datacenter:        "dc2",
				Protocol:          "grpc",
				CustomizationHash: "98809527",
				StartNode:         "resolver:web.default.default.dc2",
				Nodes: map[string]*DiscoveryGraphNode{
					"resolver:web.default.default.dc2": {
						Type: DiscoveryGraphNodeTypeResolver,
						Name: "web.default.default.dc2",
						Resolver: &DiscoveryResolver{
							ConnectTimeout: 22 * time.Second,
							Target:         "web.default.default.dc2",
						},
					},
				},
				Targets: map[string]*DiscoveryTarget{
					"web.default.default.dc2": {
						ID:         "web.default.default.dc2",
						Service:    "web",
						Namespace:  "default",
						Datacenter: "dc2",
						MeshGateway: MeshGatewayConfig{
							Mode: MeshGatewayModeLocal,
						},
						ConnectTimeout: 22 * time.Second,
						SNI:            "web.default.dc2.internal." + testClusterID + ".consul",
						Name:           "web.default.dc2.internal." + testClusterID + ".consul",
					},
				},
			},
		}
		require.Equal(t, expect, resp)
	}))
}
