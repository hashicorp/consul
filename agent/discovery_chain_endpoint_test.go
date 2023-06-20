// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestDiscoveryChainRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	newTarget := func(opts structs.DiscoveryTargetOpts) *structs.DiscoveryTarget {
		if opts.Namespace == "" {
			opts.Namespace = "default"
		}
		if opts.Partition == "" {
			opts.Partition = "default"
		}
		if opts.Datacenter == "" {
			opts.Datacenter = "dc1"
		}
		t := structs.NewDiscoveryTarget(opts)
		t.SNI = connect.TargetSNI(t, connect.TestClusterID+".consul")
		t.Name = t.SNI
		t.ConnectTimeout = 5 * time.Second // default
		return t
	}

	targetWithConnectTimeout := func(t *structs.DiscoveryTarget, connectTimeout time.Duration) *structs.DiscoveryTarget {
		t.ConnectTimeout = connectTimeout
		return t
	}

	for _, method := range []string{"GET", "POST"} {
		require.True(t, t.Run(method+": error on no service name", func(t *testing.T) {
			var (
				req *http.Request
				err error
			)
			if method == "GET" {
				req, err = http.NewRequest("GET", "/v1/discovery-chain/", nil)
			} else {
				apiReq := &discoveryChainReadRequest{}
				req, err = http.NewRequest("POST", "/v1/discovery-chain/", jsonReader(apiReq))
			}
			require.NoError(t, err)

			resp := httptest.NewRecorder()
			_, err = a.srv.DiscoveryChainRead(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		}))

		require.True(t, t.Run(method+": read default chain", func(t *testing.T) {
			var (
				req *http.Request
				err error
			)
			if method == "GET" {
				req, err = http.NewRequest("GET", "/v1/discovery-chain/web", nil)
			} else {
				apiReq := &discoveryChainReadRequest{}
				req, err = http.NewRequest("POST", "/v1/discovery-chain/web", jsonReader(apiReq))
			}
			require.NoError(t, err)

			resp := httptest.NewRecorder()
			obj, err := a.srv.DiscoveryChainRead(resp, req)
			require.NoError(t, err)

			value := obj.(discoveryChainReadResponse)

			expect := &structs.CompiledDiscoveryChain{
				ServiceName: "web",
				Namespace:   "default",
				Partition:   "default",
				Datacenter:  "dc1",
				Protocol:    "tcp",
				Default:     true,
				StartNode:   "resolver:web.default.default.dc1",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:web.default.default.dc1": {
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "web.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Default:        true,
							ConnectTimeout: 5 * time.Second,
							Target:         "web.default.default.dc1",
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.default.dc1": newTarget(structs.DiscoveryTargetOpts{Service: "web"}),
				},
			}
			require.Equal(t, expect, value.Chain)
		}))

		require.True(t, t.Run(method+": read default chain; evaluate in dc2", func(t *testing.T) {
			var (
				req *http.Request
				err error
			)
			if method == "GET" {
				req, err = http.NewRequest("GET", "/v1/discovery-chain/web?compile-dc=dc2", nil)
			} else {
				apiReq := &discoveryChainReadRequest{}
				req, err = http.NewRequest("POST", "/v1/discovery-chain/web?compile-dc=dc2", jsonReader(apiReq))
			}
			require.NoError(t, err)

			resp := httptest.NewRecorder()
			obj, err := a.srv.DiscoveryChainRead(resp, req)
			require.NoError(t, err)

			value := obj.(discoveryChainReadResponse)

			expect := &structs.CompiledDiscoveryChain{
				ServiceName: "web",
				Namespace:   "default",
				Partition:   "default",
				Datacenter:  "dc2",
				Default:     true,
				Protocol:    "tcp",
				StartNode:   "resolver:web.default.default.dc2",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:web.default.default.dc2": {
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "web.default.default.dc2",
						Resolver: &structs.DiscoveryResolver{
							Default:        true,
							ConnectTimeout: 5 * time.Second,
							Target:         "web.default.default.dc2",
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.default.dc2": newTarget(structs.DiscoveryTargetOpts{Service: "web", Datacenter: "dc2"}),
				},
			}
			require.Equal(t, expect, value.Chain)
		}))

		require.True(t, t.Run(method+": read default chain with cache", func(t *testing.T) {
			var (
				req *http.Request
				err error
			)
			if method == "GET" {
				req, err = http.NewRequest("GET", "/v1/discovery-chain/web?cached", nil)
			} else {
				apiReq := &discoveryChainReadRequest{}
				req, err = http.NewRequest("POST", "/v1/discovery-chain/web?cached", jsonReader(apiReq))
			}
			require.NoError(t, err)

			resp := httptest.NewRecorder()
			obj, err := a.srv.DiscoveryChainRead(resp, req)
			require.NoError(t, err)

			// The GET request primes the cache so the POST is a hit.
			if method == "GET" {
				// Should be a cache miss
				require.Equal(t, "MISS", resp.Header().Get("X-Cache"))
			} else {
				// Should be a cache HIT now!
				require.Equal(t, "HIT", resp.Header().Get("X-Cache"))
			}

			value := obj.(discoveryChainReadResponse)

			expect := &structs.CompiledDiscoveryChain{
				ServiceName: "web",
				Namespace:   "default",
				Partition:   "default",
				Datacenter:  "dc1",
				Default:     true,
				Protocol:    "tcp",
				StartNode:   "resolver:web.default.default.dc1",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:web.default.default.dc1": {
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "web.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Default:        true,
							ConnectTimeout: 5 * time.Second,
							Target:         "web.default.default.dc1",
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.default.dc1": newTarget(structs.DiscoveryTargetOpts{Service: "web"}),
				},
			}
			require.Equal(t, expect, value.Chain)
		}))
	}

	{ // Now create one config entry.
		out := false
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "web",
				ConnectTimeout: 33 * time.Second,
				Failover: map[string]structs.ServiceResolverFailover{
					"*": {
						Datacenters: []string{"dc2"},
					},
				},
			},
		}, &out))
		require.True(t, out)
	}

	// Ensure background refresh works
	require.True(t, t.Run("GET: read modified chain", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, err := http.NewRequest("GET", "/v1/discovery-chain/web?cached", nil)
			r.Check(err)

			resp := httptest.NewRecorder()
			obj, err := a.srv.DiscoveryChainRead(resp, req)
			r.Check(err)

			// Should be a cache hit! The data should've updated in the cache
			// in the background so this should've been fetched directly from
			// the cache.
			if resp.Header().Get("X-Cache") != "HIT" {
				r.Fatalf("should be a cache hit")
			}

			value := obj.(discoveryChainReadResponse)

			expect := &structs.CompiledDiscoveryChain{
				ServiceName: "web",
				Namespace:   "default",
				Partition:   "default",
				Datacenter:  "dc1",
				Protocol:    "tcp",
				StartNode:   "resolver:web.default.default.dc1",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:web.default.default.dc1": {
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "web.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							ConnectTimeout: 33 * time.Second,
							Target:         "web.default.default.dc1",
							Failover: &structs.DiscoveryFailover{
								Targets: []string{"web.default.default.dc2"},
							},
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.default.dc1": targetWithConnectTimeout(
						newTarget(structs.DiscoveryTargetOpts{Service: "web"}),
						33*time.Second,
					),
					"web.default.default.dc2": targetWithConnectTimeout(
						newTarget(structs.DiscoveryTargetOpts{Service: "web", Datacenter: "dc2"}),
						33*time.Second,
					),
				},
				AutoVirtualIPs:   []string{"240.0.0.1"},
				ManualVirtualIPs: []string{},
			}
			if !reflect.DeepEqual(expect, value.Chain) {
				r.Fatalf("should be equal: expected=%+v, got=%+v", expect, value.Chain)
			}
		})
	}))

	expectTarget_DC1 := targetWithConnectTimeout(
		newTarget(structs.DiscoveryTargetOpts{Service: "web"}),
		22*time.Second,
	)
	expectTarget_DC1.MeshGateway = structs.MeshGatewayConfig{
		Mode: structs.MeshGatewayModeLocal,
	}

	expectTarget_DC2 := targetWithConnectTimeout(
		newTarget(structs.DiscoveryTargetOpts{Service: "web", Datacenter: "dc2"}),
		22*time.Second,
	)
	expectTarget_DC2.MeshGateway = structs.MeshGatewayConfig{
		Mode: structs.MeshGatewayModeLocal,
	}

	expectModifiedWithOverrides := &structs.CompiledDiscoveryChain{
		ServiceName:       "web",
		Namespace:         "default",
		Partition:         "default",
		Datacenter:        "dc1",
		Protocol:          "grpc",
		CustomizationHash: "98809527",
		StartNode:         "resolver:web.default.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:web.default.default.dc1": {
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "web.default.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 22 * time.Second,
					Target:         "web.default.default.dc1",
					Failover: &structs.DiscoveryFailover{
						Targets: []string{"web.default.default.dc2"},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			expectTarget_DC1.ID: expectTarget_DC1,
			expectTarget_DC2.ID: expectTarget_DC2,
		},
		AutoVirtualIPs:   []string{"240.0.0.1"},
		ManualVirtualIPs: []string{},
	}

	require.True(t, t.Run("POST: read modified chain with overrides (camel case)", func(t *testing.T) {
		body := ` {
			"OverrideMeshGateway": {
				"Mode": "local"
			},
			"OverrideProtocol":       "grpc",
			"OverrideConnectTimeout": "22s"
		} `
		req, err := http.NewRequest("POST", "/v1/discovery-chain/web", strings.NewReader(body))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.DiscoveryChainRead(resp, req)
		require.NoError(t, err)

		value := obj.(discoveryChainReadResponse)

		require.Equal(t, expectModifiedWithOverrides, value.Chain)
	}))

	require.True(t, t.Run("POST: read modified chain with overrides (snake case)", func(t *testing.T) {
		body := ` {
			"override_mesh_gateway": {
				"mode": "local"
			},
			"override_protocol":       "grpc",
			"override_connect_timeout": "22s"
		} `
		req, err := http.NewRequest("POST", "/v1/discovery-chain/web", strings.NewReader(body))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.DiscoveryChainRead(resp, req)
		require.NoError(t, err)

		value := obj.(discoveryChainReadResponse)

		require.Equal(t, expectModifiedWithOverrides, value.Chain)
	}))
}
