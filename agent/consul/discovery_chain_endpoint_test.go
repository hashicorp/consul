package consul

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryChainEndpoint_Get(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	denyToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", "")
	require.NoError(t, err)

	allowToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `service "web" { policy = "read" }`)
	require.NoError(t, err)

	getChain := func(args *structs.DiscoveryChainRequest) (*structs.DiscoveryChainResponse, error) {
		resp := structs.DiscoveryChainResponse{}
		err := msgpackrpc.CallWithCodec(codec, "DiscoveryChain.Get", &args, &resp)
		if err != nil {
			return nil, err
		}
		// clear fields that we don't care about
		resp.QueryMeta = structs.QueryMeta{}
		return &resp, nil
	}

	newTarget := func(service, serviceSubset, namespace, datacenter string) *structs.DiscoveryTarget {
		t := structs.NewDiscoveryTarget(service, serviceSubset, namespace, datacenter)
		t.SNI = connect.TargetSNI(t, connect.TestClusterID+".consul")
		t.Name = t.SNI
		return t
	}

	// ==== compiling the default chain (no config entries)

	{ // no token
		_, err := getChain(&structs.DiscoveryChainRequest{
			Name:                 "web",
			EvaluateInDatacenter: "dc1",
			EvaluateInNamespace:  "default",
			Datacenter:           "dc1",
		})
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	}

	{ // wrong token
		_, err := getChain(&structs.DiscoveryChainRequest{
			Name:                 "web",
			EvaluateInDatacenter: "dc1",
			EvaluateInNamespace:  "default",
			Datacenter:           "dc1",
			QueryOptions:         structs.QueryOptions{Token: denyToken.SecretID},
		})
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	}

	expectDefaultResponse_DC1_Default := &structs.DiscoveryChainResponse{
		Chain: &structs.CompiledDiscoveryChain{
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
		},
	}

	// various ways with good token
	for _, tc := range []struct {
		evalDC string
		evalNS string
		expect *structs.DiscoveryChainResponse
	}{
		{
			evalDC: "dc1",
			evalNS: "default",
			expect: expectDefaultResponse_DC1_Default,
		},
		{
			evalDC: "",
			evalNS: "default",
			expect: expectDefaultResponse_DC1_Default,
		},
		{
			evalDC: "dc1",
			evalNS: "",
			expect: expectDefaultResponse_DC1_Default,
		},
		{
			evalDC: "",
			evalNS: "",
			expect: expectDefaultResponse_DC1_Default,
		},
	} {
		tc := tc
		name := fmt.Sprintf("dc=%q ns=%q", tc.evalDC, tc.evalNS)
		require.True(t, t.Run(name, func(t *testing.T) {
			resp, err := getChain(&structs.DiscoveryChainRequest{
				Name:                 "web",
				EvaluateInDatacenter: tc.evalDC,
				EvaluateInNamespace:  tc.evalNS,
				Datacenter:           "dc1",
				QueryOptions:         structs.QueryOptions{Token: allowToken.SecretID},
			})
			require.NoError(t, err)

			require.Equal(t, tc.expect, resp)
		}))
	}

	{ // Now create one config entry.
		out := false
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply",
			&structs.ConfigEntryRequest{
				Datacenter: "dc1",
				Entry: &structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "web",
					ConnectTimeout: 33 * time.Second,
				},
				WriteRequest: structs.WriteRequest{Token: "root"},
			}, &out))
		require.True(t, out)
	}

	// ==== compiling a chain with config entries

	{ // good token
		resp, err := getChain(&structs.DiscoveryChainRequest{
			Name:                 "web",
			EvaluateInDatacenter: "dc1",
			EvaluateInNamespace:  "default",
			Datacenter:           "dc1",
			QueryOptions:         structs.QueryOptions{Token: allowToken.SecretID},
		})
		require.NoError(t, err)

		expect := &structs.DiscoveryChainResponse{
			Chain: &structs.CompiledDiscoveryChain{
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
					"web.default.dc1": newTarget("web", "", "default", "dc1"),
				},
			},
		}
		require.Equal(t, expect, resp)
	}
}
