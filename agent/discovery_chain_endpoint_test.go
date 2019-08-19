package agent

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryChainRead(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	newTarget := func(service, serviceSubset, namespace, datacenter string) *structs.DiscoveryTarget {
		t := structs.NewDiscoveryTarget(service, serviceSubset, namespace, datacenter)
		t.SNI = connect.TargetSNI(t, connect.TestClusterID+".consul")
		t.Name = t.SNI
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
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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
				Datacenter:  "dc1",
				Protocol:    "tcp",
				StartNode:   "resolver:web.default.dc1",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:web.default.dc1": &structs.DiscoveryGraphNode{
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "web.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Default:        true,
							ConnectTimeout: 5 * time.Second,
							Target:         "web.default.dc1",
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.dc1": newTarget("web", "", "default", "dc1"),
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
				Datacenter:  "dc2",
				Protocol:    "tcp",
				StartNode:   "resolver:web.default.dc2",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:web.default.dc2": &structs.DiscoveryGraphNode{
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "web.default.dc2",
						Resolver: &structs.DiscoveryResolver{
							Default:        true,
							ConnectTimeout: 5 * time.Second,
							Target:         "web.default.dc2",
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.dc2": newTarget("web", "", "default", "dc2"),
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
				Datacenter:  "dc1",
				Protocol:    "tcp",
				StartNode:   "resolver:web.default.dc1",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:web.default.dc1": &structs.DiscoveryGraphNode{
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "web.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Default:        true,
							ConnectTimeout: 5 * time.Second,
							Target:         "web.default.dc1",
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.dc1": newTarget("web", "", "default", "dc1"),
				},
			}
			require.Equal(t, expect, value.Chain)
		}))
	}

	{ // Now create one config entry.
		out := false
		require.NoError(t, a.RPC("ConfigEntry.Apply", &structs.ConfigEntryRequest{
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
				Datacenter:  "dc1",
				Protocol:    "tcp",
				StartNode:   "resolver:web.default.dc1",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:web.default.dc1": &structs.DiscoveryGraphNode{
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "web.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							ConnectTimeout: 33 * time.Second,
							Target:         "web.default.dc1",
							Failover: &structs.DiscoveryFailover{
								Targets: []string{"web.default.dc2"},
							},
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.dc1": newTarget("web", "", "default", "dc1"),
					"web.default.dc2": newTarget("web", "", "default", "dc2"),
				},
			}
			if !reflect.DeepEqual(expect, value.Chain) {
				r.Fatalf("should be equal: expected=%+v, got=%+v", expect, value.Chain)
			}
		})
	}))

	// TODO(namespaces): add a test

	expectTarget_DC2 := newTarget("web", "", "default", "dc2")
	expectTarget_DC2.MeshGateway = structs.MeshGatewayConfig{
		Mode: structs.MeshGatewayModeLocal,
	}

	expectModifiedWithOverrides := &structs.CompiledDiscoveryChain{
		ServiceName:       "web",
		Namespace:         "default",
		Datacenter:        "dc1",
		Protocol:          "grpc",
		CustomizationHash: "98809527",
		StartNode:         "resolver:web.default.dc1",
		Nodes: map[string]*structs.DiscoveryGraphNode{
			"resolver:web.default.dc1": &structs.DiscoveryGraphNode{
				Type: structs.DiscoveryGraphNodeTypeResolver,
				Name: "web.default.dc1",
				Resolver: &structs.DiscoveryResolver{
					ConnectTimeout: 22 * time.Second,
					Target:         "web.default.dc1",
					Failover: &structs.DiscoveryFailover{
						Targets: []string{"web.default.dc2"},
					},
				},
			},
		},
		Targets: map[string]*structs.DiscoveryTarget{
			"web.default.dc1":   newTarget("web", "", "default", "dc1"),
			expectTarget_DC2.ID: expectTarget_DC2,
		},
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
