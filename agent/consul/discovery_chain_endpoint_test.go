// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
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
			Default:     true,
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
					"web.default.default.dc1": targetWithConnectTimeout(
						newTarget(structs.DiscoveryTargetOpts{Service: "web"}),
						33*time.Second,
					),
				},
				AutoVirtualIPs:   []string{"240.0.0.1"},
				ManualVirtualIPs: []string{},
			},
		}
		require.Equal(t, expect, resp)
	}
}

func TestDiscoveryChainEndpoint_Get_BlockOnNoChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.DevMode = true // keep it in ram to make it 10x faster on macos
		c.PrimaryDatacenter = "dc1"
	})

	codec := rpcClient(t, s1)

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
		rpcBlockingQueryTestHarness(t,
			func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error) {
				args := &structs.DiscoveryChainRequest{
					Name:                 "web",
					EvaluateInDatacenter: "dc1",
					EvaluateInNamespace:  "default",
					EvaluateInPartition:  "default",
					Datacenter:           "dc1",
				}
				args.MinQueryIndex = minQueryIndex

				var out structs.DiscoveryChainResponse
				errCh := channelCallRPC(s1, "DiscoveryChain.Get", &args, &out, nil)
				return &out.QueryMeta, errCh
			},
			func(i int) <-chan error {
				var out bool
				return channelCallRPC(s1, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
					Datacenter: "dc1",
					Entry: &structs.ServiceConfigEntry{
						Kind: structs.ServiceDefaults,
						Name: fmt.Sprintf(dataPrefix+"%d", i),
					},
				}, &out, nil)
			},
		)
	}

	testutil.RunStep(t, "test the errNotFound path", func(t *testing.T) {
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

	testutil.RunStep(t, "test the errNotChanged path", func(t *testing.T) {
		run(t, "completely-different-other")
	})
}

// TestComputeDiscoveryChainHash_CoversAllCompileRequestFields ensures that
// computeDiscoveryChainHash covers all fields in CompileRequest.
// If this test fails, a new field was added to CompileRequest that needs
// to be included in the hash computation.
func TestComputeDiscoveryChainHash_CoversAllCompileRequestFields(t *testing.T) {
	// Fields that are covered in computeDiscoveryChainHash
	// UPDATE THIS MAP when adding new fields to CompileRequest
	coveredFields := map[string]string{
		"ServiceName":            "hashed directly",
		"EvaluateInNamespace":    "hashed directly",
		"EvaluateInPartition":    "hashed directly",
		"EvaluateInDatacenter":   "hashed directly",
		"EvaluateInTrustDomain":  "hashed directly",
		"OverrideMeshGateway":    "hashed via Mode field",
		"OverrideProtocol":       "hashed directly",
		"OverrideConnectTimeout": "hashed directly",
		"Entries":                "covered via entries.Hash() parameter",
		"AutoVirtualIPs":         "hashed from chain",
		"ManualVirtualIPs":       "hashed from chain",
	}

	typ := reflect.TypeOf(discoverychain.CompileRequest{})

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if _, ok := coveredFields[field.Name]; !ok {
			t.Errorf("CompileRequest field %q is not covered in computeDiscoveryChainHash. "+
				"Add it to the hash computation and update coveredFields in this test.", field.Name)
		}
	}
}

// TestComputeDiscoveryChainHash_CoversAllCompiledChainFields ensures that
// fields in CompiledDiscoveryChain that are NOT derived from config entries
// are properly hashed.
// If this test fails, review if the new field affects cache correctness.
func TestComputeDiscoveryChainHash_CoversAllCompiledChainFields(t *testing.T) {
	// Fields and their coverage status
	// UPDATE THIS MAP when adding new fields to CompiledDiscoveryChain
	fieldCoverage := map[string]string{
		// Derived from Entries - covered by entries.Hash()
		"ServiceName":       "derived from compile request, hashed via req",
		"Namespace":         "derived from compile request, hashed via req.EvaluateInNamespace",
		"Partition":         "derived from compile request, hashed via req.EvaluateInPartition",
		"Datacenter":        "derived from compile request, hashed via req.EvaluateInDatacenter",
		"CustomizationHash": "derived from overrides, hashed via req override fields",
		"Default":           "derived from entries, covered by entries.Hash()",
		"Protocol":          "derived from entries + override, covered",
		"ServiceMeta":       "derived from entries, covered by entries.Hash()",
		"EnvoyExtensions":   "derived from entries, covered by entries.Hash()",
		"StartNode":         "derived from entries, covered by entries.Hash()",
		"Nodes":             "derived from entries, covered by entries.Hash()",
		"Targets":           "derived from entries, covered by entries.Hash()",

		// NOT derived from entries - must be explicitly hashed
		"AutoVirtualIPs":   "from VIP table, hashed from chain",
		"ManualVirtualIPs": "from VIP table, hashed from chain",
	}

	typ := reflect.TypeOf(structs.CompiledDiscoveryChain{})

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if _, ok := fieldCoverage[field.Name]; !ok {
			t.Errorf("CompiledDiscoveryChain field %q is not documented in fieldCoverage. "+
				"Determine if it's derived from entries (covered by entries.Hash()) or "+
				"needs explicit hashing, then update this test.", field.Name)
		}
	}
}

// TestComputeDiscoveryChainHash_Deterministic verifies that the hash is
// deterministic for the same inputs.
func TestComputeDiscoveryChainHash_Deterministic(t *testing.T) {
	entries := configentry.NewDiscoveryChainSet()
	entries.AddRouters(&structs.ServiceRouterConfigEntry{
		Kind: structs.ServiceRouter,
		Name: "web",
	})

	chain := &structs.CompiledDiscoveryChain{
		ServiceName:    "web",
		Namespace:      "default",
		Partition:      "default",
		Datacenter:     "dc1",
		AutoVirtualIPs: []string{"10.0.0.1"},
	}

	hash1 := computeDiscoveryChainHash(entries, chain)
	hash2 := computeDiscoveryChainHash(entries, chain)

	require.Equal(t, hash1, hash2, "hash should be deterministic")
}

// TestComputeDiscoveryChainHash_DifferentInputs verifies that different
// inputs produce different hashes.
func TestComputeDiscoveryChainHash_DifferentInputs(t *testing.T) {
	entries := configentry.NewDiscoveryChainSet()

	chain1 := &structs.CompiledDiscoveryChain{
		ServiceName:    "web",
		Namespace:      "default",
		Partition:      "default",
		Datacenter:     "dc1",
		AutoVirtualIPs: []string{"10.0.0.1"},
	}

	chain2 := &structs.CompiledDiscoveryChain{
		ServiceName:    "web",
		Namespace:      "default",
		Partition:      "default",
		Datacenter:     "dc1",
		AutoVirtualIPs: []string{"10.0.0.2"}, // Different VIP
	}

	hash1 := computeDiscoveryChainHash(entries, chain1)
	hash2 := computeDiscoveryChainHash(entries, chain2)

	require.NotEqual(t, hash1, hash2, "different VIPs should produce different hashes")
}

// TestComputeDiscoveryChainHash_FieldChangesAffectHash verifies that changing
// each hashed field produces a different hash value.
func TestComputeDiscoveryChainHash_FieldChangesAffectHash(t *testing.T) {
	// Base entries and chain for comparison
	baseEntries := func() *configentry.DiscoveryChainSet {
		entries := configentry.NewDiscoveryChainSet()
		router := &structs.ServiceRouterConfigEntry{
			Kind: structs.ServiceRouter,
			Name: "web",
		}
		router.Normalize()
		entries.AddRouters(router)
		return entries
	}

	baseChain := func() *structs.CompiledDiscoveryChain {
		return &structs.CompiledDiscoveryChain{
			ServiceName:       "web",
			Namespace:         "default",
			Partition:         "default",
			Datacenter:        "dc1",
			CustomizationHash: "abc123",
			AutoVirtualIPs:    []string{"10.0.0.1"},
			ManualVirtualIPs:  []string{"10.0.0.100"},
		}
	}

	baseHash := computeDiscoveryChainHash(baseEntries(), baseChain())

	tests := []struct {
		name       string
		modifyFunc func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain)
	}{
		{
			name: "different ServiceName",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.ServiceName = "api"
				return baseEntries(), chain
			},
		},
		{
			name: "different Namespace",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.Namespace = "prod"
				return baseEntries(), chain
			},
		},
		{
			name: "different Partition",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.Partition = "aws"
				return baseEntries(), chain
			},
		},
		{
			name: "different Datacenter",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.Datacenter = "dc2"
				return baseEntries(), chain
			},
		},
		{
			name: "different CustomizationHash",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.CustomizationHash = "xyz789"
				return baseEntries(), chain
			},
		},
		{
			name: "different AutoVirtualIPs value",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.AutoVirtualIPs = []string{"10.0.0.2"}
				return baseEntries(), chain
			},
		},
		{
			name: "additional AutoVirtualIP",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.AutoVirtualIPs = []string{"10.0.0.1", "10.0.0.2"}
				return baseEntries(), chain
			},
		},
		{
			name: "empty AutoVirtualIPs",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.AutoVirtualIPs = []string{}
				return baseEntries(), chain
			},
		},
		{
			name: "different ManualVirtualIPs value",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.ManualVirtualIPs = []string{"10.0.0.200"}
				return baseEntries(), chain
			},
		},
		{
			name: "additional ManualVirtualIP",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.ManualVirtualIPs = []string{"10.0.0.100", "10.0.0.101"}
				return baseEntries(), chain
			},
		},
		{
			name: "empty ManualVirtualIPs",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				chain := baseChain()
				chain.ManualVirtualIPs = []string{}
				return baseEntries(), chain
			},
		},
		{
			name: "different router entry",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				entries := configentry.NewDiscoveryChainSet()
				router := &structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "api", // different name
				}
				router.Normalize()
				entries.AddRouters(router)
				return entries, baseChain()
			},
		},
		{
			name: "additional splitter entry",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				entries := baseEntries()
				splitter := &structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "web",
					Splits: []structs.ServiceSplit{
						{Weight: 100, Service: "web"},
					},
				}
				splitter.Normalize()
				entries.AddSplitters(splitter)
				return entries, baseChain()
			},
		},
		{
			name: "additional resolver entry",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				entries := baseEntries()
				resolver := &structs.ServiceResolverConfigEntry{
					Kind:           structs.ServiceResolver,
					Name:           "web",
					ConnectTimeout: 10 * time.Second,
				}
				resolver.Normalize()
				entries.AddResolvers(resolver)
				return entries, baseChain()
			},
		},
		{
			name: "nil entries vs empty entries",
			modifyFunc: func() (*configentry.DiscoveryChainSet, *structs.CompiledDiscoveryChain) {
				return nil, baseChain()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entries, chain := tc.modifyFunc()
			modifiedHash := computeDiscoveryChainHash(entries, chain)
			require.NotEqual(t, baseHash, modifiedHash,
				"changing %s should produce a different hash", tc.name)
		})
	}
}

// TestComputeDiscoveryChainHash_NilInputs verifies behavior with nil inputs.
func TestComputeDiscoveryChainHash_NilInputs(t *testing.T) {
	entries := configentry.NewDiscoveryChainSet()
	chain := &structs.CompiledDiscoveryChain{
		ServiceName: "web",
	}

	// All combinations should not panic
	t.Run("nil entries", func(t *testing.T) {
		hash := computeDiscoveryChainHash(nil, chain)
		require.NotZero(t, hash)
	})

	t.Run("nil chain", func(t *testing.T) {
		hash := computeDiscoveryChainHash(entries, nil)
		require.NotZero(t, hash)
	})

	t.Run("both nil", func(t *testing.T) {
		hash := computeDiscoveryChainHash(nil, nil)
		// Should return a consistent value (could be zero)
		hash2 := computeDiscoveryChainHash(nil, nil)
		require.Equal(t, hash, hash2)
	})

	t.Run("nil entries vs non-nil entries produce different hash", func(t *testing.T) {
		hash1 := computeDiscoveryChainHash(nil, chain)
		// Empty entries and nil entries might be the same
		hash2 := computeDiscoveryChainHash(entries, chain)
		// The important test: if entries has content they should differ
		router := &structs.ServiceRouterConfigEntry{Kind: structs.ServiceRouter, Name: "web"}
		router.Normalize()
		entries.AddRouters(router)
		hash3 := computeDiscoveryChainHash(entries, chain)
		require.NotEqual(t, hash1, hash3, "nil entries vs entries with content should differ")
		// Also verify empty vs content differs
		require.NotEqual(t, hash2, hash3, "empty entries vs entries with content should differ")
	})
}

// TestComputeDiscoveryChainHash_OrderIndependence verifies that map iteration
// order doesn't affect the hash (entries should sort keys).
func TestComputeDiscoveryChainHash_OrderIndependence(t *testing.T) {
	// Create entries with multiple items to test sorting
	createEntries := func() *configentry.DiscoveryChainSet {
		entries := configentry.NewDiscoveryChainSet()

		// Add multiple routers in different order each time
		router1 := &structs.ServiceRouterConfigEntry{Kind: structs.ServiceRouter, Name: "aaa"}
		router2 := &structs.ServiceRouterConfigEntry{Kind: structs.ServiceRouter, Name: "zzz"}
		router3 := &structs.ServiceRouterConfigEntry{Kind: structs.ServiceRouter, Name: "mmm"}

		router1.Normalize()
		router2.Normalize()
		router3.Normalize()

		entries.AddRouters(router1, router2, router3)
		return entries
	}

	chain := &structs.CompiledDiscoveryChain{
		ServiceName: "web",
	}

	// Hash multiple times - should be consistent due to sorted iteration
	hashes := make([]uint64, 10)
	for i := 0; i < 10; i++ {
		hashes[i] = computeDiscoveryChainHash(createEntries(), chain)
	}

	for i := 1; i < len(hashes); i++ {
		require.Equal(t, hashes[0], hashes[i],
			"hash should be consistent regardless of map iteration order")
	}
}
