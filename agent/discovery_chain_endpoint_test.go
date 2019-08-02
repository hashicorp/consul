package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryChainRead(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
					"web.default.dc1": structs.NewDiscoveryTarget("web", "", "default", "dc1"),
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
					"web.default.dc2": structs.NewDiscoveryTarget("web", "", "default", "dc2"),
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
			},
		}, &out))
		require.True(t, out)
	}

	require.True(t, t.Run("GET: read modified chain", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/discovery-chain/web", nil)
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
						ConnectTimeout: 33 * time.Second,
						Target:         "web.default.dc1",
					},
				},
			},
			Targets: map[string]*structs.DiscoveryTarget{
				"web.default.dc1": structs.NewDiscoveryTarget("web", "", "default", "dc1"),
			},
		}
		require.Equal(t, expect, value.Chain)
	}))

	// TODO(namespaces): add a test

	require.True(t, t.Run("POST: read modified chain with overrides", func(t *testing.T) {
		apiReq := &discoveryChainReadRequest{
			OverrideMeshGateway: structs.MeshGatewayConfig{
				Mode: structs.MeshGatewayModeLocal,
			},
			OverrideProtocol:       "grpc",
			OverrideConnectTimeout: 22 * time.Second,
		}
		req, err := http.NewRequest("POST", "/v1/discovery-chain/web", jsonReader(apiReq))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.DiscoveryChainRead(resp, req)
		require.NoError(t, err)

		value := obj.(discoveryChainReadResponse)

		expectTarget := structs.NewDiscoveryTarget("web", "", "default", "dc1")
		expectTarget.MeshGateway = structs.MeshGatewayConfig{
			Mode: structs.MeshGatewayModeLocal,
		}

		expect := &structs.CompiledDiscoveryChain{
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
					},
				},
			},
			Targets: map[string]*structs.DiscoveryTarget{
				expectTarget.ID: expectTarget,
			},
		}
		require.Equal(t, expect, value.Chain)
	}))
}
