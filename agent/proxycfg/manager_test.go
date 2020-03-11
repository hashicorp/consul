package proxycfg

import (
	"path"
	"testing"
	"time"

	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil"
)

func mustCopyProxyConfig(t *testing.T, ns *structs.NodeService) structs.ConnectProxyConfig {
	cfg, err := copyProxyConfig(ns)
	require.NoError(t, err)
	return cfg
}

// assertLastReqArgs verifies that each request type had the correct source
// parameters (e.g. Datacenter name) and token.
func assertLastReqArgs(t *testing.T, types *TestCacheTypes, token string, source *structs.QuerySource) {
	t.Helper()
	// Roots needs correct DC and token
	rootReq := types.roots.lastReq.Load()
	require.IsType(t, rootReq, &structs.DCSpecificRequest{})
	require.Equal(t, token, rootReq.(*structs.DCSpecificRequest).Token)
	require.Equal(t, source.Datacenter, rootReq.(*structs.DCSpecificRequest).Datacenter)

	// Leaf needs correct DC and token
	leafReq := types.leaf.lastReq.Load()
	require.IsType(t, leafReq, &cachetype.ConnectCALeafRequest{})
	require.Equal(t, token, leafReq.(*cachetype.ConnectCALeafRequest).Token)
	require.Equal(t, source.Datacenter, leafReq.(*cachetype.ConnectCALeafRequest).Datacenter)

	// Intentions needs correct DC and token
	intReq := types.intentions.lastReq.Load()
	require.IsType(t, intReq, &structs.IntentionQueryRequest{})
	require.Equal(t, token, intReq.(*structs.IntentionQueryRequest).Token)
	require.Equal(t, source.Datacenter, intReq.(*structs.IntentionQueryRequest).Datacenter)
}

func TestManager_BasicLifecycle(t *testing.T) {
	// Create a bunch of common data for the various test cases.
	roots, leaf := TestCerts(t)

	dbDefaultChain := func() *structs.CompiledDiscoveryChain {
		return discoverychain.TestCompileConfigEntries(t, "db", "default", "dc1", connect.TestClusterID+".consul", "dc1",
			func(req *discoverychain.CompileRequest) {
				// This is because structs.TestUpstreams uses an opaque config
				// to override connect timeouts.
				req.OverrideConnectTimeout = 1 * time.Second
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "db",
			},
		)
	}
	dbSplitChain := func() *structs.CompiledDiscoveryChain {
		return discoverychain.TestCompileConfigEntries(t, "db", "default", "dc1", "trustdomain.consul", "dc1",
			func(req *discoverychain.CompileRequest) {
				// This is because structs.TestUpstreams uses an opaque config
				// to override connect timeouts.
				req.OverrideConnectTimeout = 1 * time.Second
			},
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "db",
				Subsets: map[string]structs.ServiceResolverSubset{
					"v1": {
						Filter: "Service.Meta.version == v1",
					},
					"v2": {
						Filter: "Service.Meta.version == v2",
					},
				},
			},
			&structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "db",
				Splits: []structs.ServiceSplit{
					{Weight: 60, ServiceSubset: "v1"},
					{Weight: 40, ServiceSubset: "v2"},
				},
			},
		)
	}
	webProxy := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-sidecar-proxy",
		Service: "web-sidecar-proxy",
		Port:    9999,
		Meta:    map[string]string{},
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceID:   "web",
			DestinationServiceName: "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8080,
			Config: map[string]interface{}{
				"foo": "bar",
			},
			Upstreams: structs.TestUpstreams(t),
		},
	}

	rootsCacheKey := testGenCacheKey(&structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token"},
	})
	leafCacheKey := testGenCacheKey(&cachetype.ConnectCALeafRequest{
		Datacenter: "dc1",
		Token:      "my-token",
		Service:    "web",
	})
	intentionCacheKey := testGenCacheKey(&structs.IntentionQueryRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token"},
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: structs.IntentionDefaultNamespace,
					Name:      "web",
				},
			},
		},
	})

	dbChainCacheKey := testGenCacheKey(&structs.DiscoveryChainRequest{
		Name:                 "db",
		EvaluateInDatacenter: "dc1",
		EvaluateInNamespace:  "default",
		// This is because structs.TestUpstreams uses an opaque config
		// to override connect timeouts.
		OverrideConnectTimeout: 1 * time.Second,
		Datacenter:             "dc1",
		QueryOptions:           structs.QueryOptions{Token: "my-token"},
	})

	dbHealthCacheKey := testGenCacheKey(&structs.ServiceSpecificRequest{
		Datacenter:     "dc1",
		QueryOptions:   structs.QueryOptions{Token: "my-token", Filter: ""},
		ServiceName:    "db",
		Connect:        true,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	})
	db_v1_HealthCacheKey := testGenCacheKey(&structs.ServiceSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token",
			Filter: "Service.Meta.version == v1",
		},
		ServiceName:    "db",
		Connect:        true,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	})
	db_v2_HealthCacheKey := testGenCacheKey(&structs.ServiceSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token",
			Filter: "Service.Meta.version == v2",
		},
		ServiceName:    "db",
		Connect:        true,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	})

	// Create test cases using some of the common data above.
	tests := []*testcase_BasicLifecycle{
		{
			name: "simple-default-resolver",
			setup: func(t *testing.T, types *TestCacheTypes) {
				// Note that we deliberately leave the 'geo-cache' prepared query to time out
				types.health.Set(dbHealthCacheKey, &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t),
				})
				types.compiledChain.Set(dbChainCacheKey, &structs.DiscoveryChainResponse{
					Chain: dbDefaultChain(),
				})
			},
			expectSnap: &ConfigSnapshot{
				Kind:            structs.ServiceKindConnectProxy,
				Service:         webProxy.Service,
				ProxyID:         webProxy.CompoundServiceID(),
				Address:         webProxy.Address,
				Port:            webProxy.Port,
				Proxy:           mustCopyProxyConfig(t, webProxy),
				ServiceMeta:     webProxy.Meta,
				TaggedAddresses: make(map[string]structs.ServiceAddress),
				Roots:           roots,
				ConnectProxy: configSnapshotConnectProxy{
					Leaf: leaf,
					DiscoveryChain: map[string]*structs.CompiledDiscoveryChain{
						"db": dbDefaultChain(),
					},
					WatchedUpstreams: nil, // Clone() clears this out
					WatchedUpstreamEndpoints: map[string]map[string]structs.CheckServiceNodes{
						"db": {
							"db.default.dc1": TestUpstreamNodes(t),
						},
					},
					WatchedGateways: nil, // Clone() clears this out
					WatchedGatewayEndpoints: map[string]map[string]structs.CheckServiceNodes{
						"db": {},
					},
					PreparedQueryEndpoints: map[string]structs.CheckServiceNodes{},
					WatchedServiceChecks:   map[structs.ServiceID][]structs.CheckType{},
				},
				Datacenter: "dc1",
			},
		},
		{
			name: "chain-resolver-with-version-split",
			setup: func(t *testing.T, types *TestCacheTypes) {
				// Note that we deliberately leave the 'geo-cache' prepared query to time out
				types.health.Set(db_v1_HealthCacheKey, &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t),
				})
				types.health.Set(db_v2_HealthCacheKey, &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodesAlternate(t),
				})
				types.compiledChain.Set(dbChainCacheKey, &structs.DiscoveryChainResponse{
					Chain: dbSplitChain(),
				})
			},
			expectSnap: &ConfigSnapshot{
				Kind:            structs.ServiceKindConnectProxy,
				Service:         webProxy.Service,
				ProxyID:         webProxy.CompoundServiceID(),
				Address:         webProxy.Address,
				Port:            webProxy.Port,
				Proxy:           mustCopyProxyConfig(t, webProxy),
				ServiceMeta:     webProxy.Meta,
				TaggedAddresses: make(map[string]structs.ServiceAddress),
				Roots:           roots,
				ConnectProxy: configSnapshotConnectProxy{
					Leaf: leaf,
					DiscoveryChain: map[string]*structs.CompiledDiscoveryChain{
						"db": dbSplitChain(),
					},
					WatchedUpstreams: nil, // Clone() clears this out
					WatchedUpstreamEndpoints: map[string]map[string]structs.CheckServiceNodes{
						"db": {
							"v1.db.default.dc1": TestUpstreamNodes(t),
							"v2.db.default.dc1": TestUpstreamNodesAlternate(t),
						},
					},
					WatchedGateways: nil, // Clone() clears this out
					WatchedGatewayEndpoints: map[string]map[string]structs.CheckServiceNodes{
						"db": {},
					},
					PreparedQueryEndpoints: map[string]structs.CheckServiceNodes{},
					WatchedServiceChecks:   map[structs.ServiceID][]structs.CheckType{},
				},
				Datacenter: "dc1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.setup)
			require.NotNil(t, tt.expectSnap)

			// Use a mocked cache to make life simpler
			types := NewTestCacheTypes(t)

			// Setup initial values
			types.roots.Set(rootsCacheKey, roots)
			types.leaf.Set(leafCacheKey, leaf)
			types.intentions.Set(intentionCacheKey, TestIntentions(t))
			tt.setup(t, types)

			expectSnapCopy, err := copystructure.Copy(tt.expectSnap)
			require.NoError(t, err)

			webProxyCopy, err := copystructure.Copy(webProxy)
			require.NoError(t, err)

			testManager_BasicLifecycle(t, tt, types,
				rootsCacheKey, leafCacheKey,
				roots, leaf,
				webProxyCopy.(*structs.NodeService),
				expectSnapCopy.(*ConfigSnapshot),
			)
		})
	}
}

type testcase_BasicLifecycle struct {
	name       string
	setup      func(t *testing.T, types *TestCacheTypes)
	webProxy   *structs.NodeService
	expectSnap *ConfigSnapshot
}

func testManager_BasicLifecycle(
	t *testing.T,
	tt *testcase_BasicLifecycle,
	types *TestCacheTypes,
	rootsCacheKey, leafCacheKey string,
	roots *structs.IndexedCARoots,
	leaf *structs.IssuedCert,
	webProxy *structs.NodeService,
	expectSnap *ConfigSnapshot,
) {
	c := TestCacheWithTypes(t, types)

	require := require.New(t)
	logger := testutil.Logger(t)
	state := local.NewState(local.Config{}, logger, &token.Store{})
	source := &structs.QuerySource{
		Node:       "node1",
		Datacenter: "dc1",
	}

	// Stub state syncing
	state.TriggerSyncChanges = func() {}

	// Create manager
	m, err := NewManager(ManagerConfig{c, state, source, logger, nil})
	require.NoError(err)

	// And run it
	go func() {
		err := m.Run()
		require.NoError(err)
	}()

	// BEFORE we register, we should be able to get a watch channel
	wCh, cancel := m.Watch(webProxy.CompoundServiceID())
	defer cancel()

	// And it should block with nothing sent on it yet
	assertWatchChanBlocks(t, wCh)

	require.NoError(state.AddService(webProxy, "my-token"))

	// We should see the initial config delivered but not until after the
	// coalesce timeout
	start := time.Now()
	assertWatchChanRecvs(t, wCh, expectSnap)
	require.True(time.Since(start) >= coalesceTimeout)

	assertLastReqArgs(t, types, "my-token", source)

	// Update NodeConfig
	webProxy.Port = 7777
	require.NoError(state.AddService(webProxy, "my-token"))

	expectSnap.Port = 7777
	assertWatchChanRecvs(t, wCh, expectSnap)

	// Register a second watcher
	wCh2, cancel2 := m.Watch(webProxy.CompoundServiceID())
	defer cancel2()

	// New watcher should immediately receive the current state
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Change token
	require.NoError(state.AddService(webProxy, "other-token"))
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// This is actually sort of timing dependent - the cache background fetcher
	// will still be fetching with the old token, but we rely on the fact that our
	// mock type will have been blocked on those for a while.
	assertLastReqArgs(t, types, "other-token", source)
	// Update roots
	newRoots, newLeaf := TestCerts(t)
	newRoots.Roots = append(newRoots.Roots, roots.Roots...)
	types.roots.Set(rootsCacheKey, newRoots)

	// Expect new roots in snapshot
	expectSnap.Roots = newRoots
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Update leaf
	types.leaf.Set(leafCacheKey, newLeaf)

	// Expect new roots in snapshot
	expectSnap.ConnectProxy.Leaf = newLeaf
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Remove the proxy
	state.RemoveService(webProxy.CompoundServiceID())

	// Chan should NOT close
	assertWatchChanBlocks(t, wCh)
	assertWatchChanBlocks(t, wCh2)

	// Re-add the proxy with another new port
	webProxy.Port = 3333
	require.NoError(state.AddService(webProxy, "other-token"))

	// Same watch chan should be notified again
	expectSnap.Port = 3333
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Cancel watch
	cancel()

	// Watch chan should be closed
	assertWatchChanRecvs(t, wCh, nil)

	// We specifically don't remove the proxy or cancel the second watcher to
	// ensure both are cleaned up by close.
	require.NoError(m.Close())

	// Sanity check the state is clean
	m.mu.Lock()
	defer m.mu.Unlock()
	require.Len(m.proxies, 0)
	require.Len(m.watchers, 0)
}

func assertWatchChanBlocks(t *testing.T, ch <-chan *ConfigSnapshot) {
	t.Helper()

	select {
	case <-ch:
		t.Fatal("Should be nothing sent on watch chan yet")
	default:
	}
}

func assertWatchChanRecvs(t *testing.T, ch <-chan *ConfigSnapshot, expect *ConfigSnapshot) {
	t.Helper()

	select {
	case got, ok := <-ch:
		require.Equal(t, expect, got)
		if expect == nil {
			require.False(t, ok, "watch chan should be closed")
		}
	case <-time.After(100*time.Millisecond + coalesceTimeout):
		t.Fatal("recv timeout")
	}
}

func TestManager_deliverLatest(t *testing.T) {
	// None of these need to do anything to test this method just be valid
	logger := testutil.Logger(t)
	cfg := ManagerConfig{
		Cache: cache.New(nil),
		State: local.NewState(local.Config{}, logger, &token.Store{}),
		Source: &structs.QuerySource{
			Node:       "node1",
			Datacenter: "dc1",
		},
		Logger: logger,
	}
	require := require.New(t)

	m, err := NewManager(cfg)
	require.NoError(err)

	snap1 := &ConfigSnapshot{
		ProxyID: structs.NewServiceID("test-proxy", nil),
		Port:    1111,
	}
	snap2 := &ConfigSnapshot{
		ProxyID: structs.NewServiceID("test-proxy", nil),
		Port:    2222,
	}

	// Put an overall time limit on this test case so we don't have to guard every
	// call to ensure the whole test doesn't deadlock.
	time.AfterFunc(100*time.Millisecond, func() {
		t.Fatal("test timed out")
	})

	// test 1 buffered chan
	ch1 := make(chan *ConfigSnapshot, 1)

	// Sending to an unblocked chan should work
	m.deliverLatest(snap1, ch1)

	// Check it was delivered
	require.Equal(snap1, <-ch1)

	// Now send both without reading simulating a slow client
	m.deliverLatest(snap1, ch1)
	m.deliverLatest(snap2, ch1)

	// Check we got the _second_ one
	require.Equal(snap2, <-ch1)

	// Same again for 5-buffered chan
	ch5 := make(chan *ConfigSnapshot, 5)

	// Sending to an unblocked chan should work
	m.deliverLatest(snap1, ch5)

	// Check it was delivered
	require.Equal(snap1, <-ch5)

	// Now send enough to fill the chan simulating a slow client
	for i := 0; i < 5; i++ {
		m.deliverLatest(snap1, ch5)
	}
	m.deliverLatest(snap2, ch5)

	// Check we got the _second_ one
	require.Equal(snap2, <-ch5)
}

func testGenCacheKey(req cache.Request) string {
	info := req.CacheInfo()
	return path.Join(info.Key, info.Datacenter)
}
