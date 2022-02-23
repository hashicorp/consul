package consul

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestDiscoveryChainEndpoint_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

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

	newTarget := func(service, serviceSubset, namespace, partition, datacenter string) *structs.DiscoveryTarget {
		t := structs.NewDiscoveryTarget(service, serviceSubset, namespace, partition, datacenter)
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
			EvaluateInPartition:  "default",
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
			EvaluateInPartition:  "default",
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
			Partition:   "default",
			Datacenter:  "dc1",
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
				"web.default.default.dc1": newTarget("web", "", "default", "default", "dc1"),
			},
		},
	}

	// various ways with good token
	for _, tc := range []struct {
		evalDC   string
		evalNS   string
		evalPart string
		expect   *structs.DiscoveryChainResponse
	}{
		{
			evalDC:   "dc1",
			evalNS:   "default",
			evalPart: "default",
			expect:   expectDefaultResponse_DC1_Default,
		},
		{
			evalDC:   "",
			evalNS:   "default",
			evalPart: "default",
			expect:   expectDefaultResponse_DC1_Default,
		},
		{
			evalDC:   "dc1",
			evalNS:   "",
			evalPart: "default",
			expect:   expectDefaultResponse_DC1_Default,
		},
		{
			evalDC:   "",
			evalNS:   "",
			evalPart: "default",
			expect:   expectDefaultResponse_DC1_Default,
		},
		{
			evalDC:   "dc1",
			evalNS:   "default",
			evalPart: "",
			expect:   expectDefaultResponse_DC1_Default,
		},
		{
			evalDC:   "",
			evalNS:   "default",
			evalPart: "",
			expect:   expectDefaultResponse_DC1_Default,
		},
		{
			evalDC:   "dc1",
			evalNS:   "",
			evalPart: "",
			expect:   expectDefaultResponse_DC1_Default,
		},
		{
			evalDC:   "",
			evalNS:   "",
			evalPart: "",
			expect:   expectDefaultResponse_DC1_Default,
		},
	} {
		tc := tc
		name := fmt.Sprintf("dc=%q ns=%q", tc.evalDC, tc.evalNS)
		require.True(t, t.Run(name, func(t *testing.T) {
			resp, err := getChain(&structs.DiscoveryChainRequest{
				Name:                 "web",
				EvaluateInDatacenter: tc.evalDC,
				EvaluateInNamespace:  tc.evalNS,
				EvaluateInPartition:  tc.evalPart,
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
			EvaluateInPartition:  "default",
			Datacenter:           "dc1",
			QueryOptions:         structs.QueryOptions{Token: allowToken.SecretID},
		})
		require.NoError(t, err)

		expect := &structs.DiscoveryChainResponse{
			Chain: &structs.CompiledDiscoveryChain{
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
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"web.default.default.dc1": newTarget("web", "", "default", "default", "dc1"),
				},
			},
		}
		require.Equal(t, expect, resp)
	}
}

func TestDiscoveryChainEndpoint_Get_BlockOnNoChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
	})

	codec := rpcClient(t, s1)
	readerCodec := rpcClient(t, s1)
	writerCodec := rpcClient(t, s1)

	waitForLeaderEstablishment(t, s1)
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	{ // create one unrelated entry
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "unrelated",
				ConnectTimeout: 33 * time.Second,
			},
		}, &out))
		require.True(t, out)
	}

	run := func(t *testing.T, dataPrefix string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var count int

		start := time.Now()
		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			args := &structs.DiscoveryChainRequest{
				Name:                 "web",
				EvaluateInDatacenter: "dc1",
				EvaluateInNamespace:  "default",
				EvaluateInPartition:  "default",
				Datacenter:           "dc1",
			}
			args.QueryOptions.MaxQueryTime = time.Second

			for ctx.Err() == nil {
				var out structs.DiscoveryChainResponse
				err := msgpackrpc.CallWithCodec(readerCodec, "DiscoveryChain.Get", &args, &out)
				if err != nil {
					return fmt.Errorf("error getting discovery chain: %w", err)
				}
				if !out.Chain.IsDefault() {
					return fmt.Errorf("expected default chain")
				}

				t.Log("blocking query index", out.QueryMeta.Index, out.Chain)
				count++
				args.QueryOptions.MinQueryIndex = out.QueryMeta.Index
			}
			return nil
		})

		g.Go(func() error {
			for i := 0; i < 200; i++ {
				time.Sleep(5 * time.Millisecond)

				var out bool
				err := msgpackrpc.CallWithCodec(writerCodec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
					Datacenter: "dc1",
					Entry: &structs.ServiceConfigEntry{
						Kind: structs.ServiceDefaults,
						Name: fmt.Sprintf(dataPrefix+"%d", i),
					},
				}, &out)
				if err != nil {
					return fmt.Errorf("[%d] unexpected error: %w", i, err)
				}
				if !out {
					return fmt.Errorf("[%d] unexpectedly returned false", i)
				}
			}
			cancel()
			return nil
		})

		require.NoError(t, g.Wait())

		assertBlockingQueryWakeupCount(t, time.Second, start, count)
	}

	runStep(t, "test the errNotFound path", func(t *testing.T) {
		run(t, "other")
	})

	{ // create one relevant entry
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "web",
				Protocol: "grpc",
			},
		}, &out))
		require.True(t, out)
	}

	runStep(t, "test the errNotChanged path", func(t *testing.T) {
		run(t, "completely-different-other")
	})
}
