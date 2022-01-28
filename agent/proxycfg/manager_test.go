package proxycfg

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/rpcclient/health"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Create a bunch of common data for the various test cases.
	roots, leaf := TestCerts(t)

	dbDefaultChain := func() *structs.CompiledDiscoveryChain {
		return discoverychain.TestCompileConfigEntries(t, "db", "default", "default", "dc1", connect.TestClusterID+".consul", func(req *discoverychain.CompileRequest) {
			// This is because structs.TestUpstreams uses an opaque config
			// to override connect timeouts.
			req.OverrideConnectTimeout = 1 * time.Second
		}, &structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "db",
		})
	}
	dbSplitChain := func() *structs.CompiledDiscoveryChain {
		return discoverychain.TestCompileConfigEntries(t, "db", "default", "default", "dc1", "trustdomain.consul", func(req *discoverychain.CompileRequest) {
			// This is because structs.TestUpstreams uses an opaque config
			// to override connect timeouts.
			req.OverrideConnectTimeout = 1 * time.Second
		}, &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": "http",
			},
		}, &structs.ServiceResolverConfigEntry{
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
		}, &structs.ServiceSplitterConfigEntry{
			Kind: structs.ServiceSplitter,
			Name: "db",
			Splits: []structs.ServiceSplit{
				{Weight: 60, ServiceSubset: "v1"},
				{Weight: 40, ServiceSubset: "v2"},
			},
		})
	}

	upstreams := structs.TestUpstreams(t)
	for i := range upstreams {
		upstreams[i].DestinationNamespace = structs.IntentionDefaultNamespace
		upstreams[i].DestinationPartition = api.PartitionDefaultName
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
			Upstreams: upstreams,
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
					Partition: structs.IntentionDefaultNamespace,
					Name:      "web",
				},
			},
		},
	})

	dbChainCacheKey := testGenCacheKey(&structs.DiscoveryChainRequest{
		Name:                 "db",
		EvaluateInDatacenter: "dc1",
		EvaluateInNamespace:  "default",
		EvaluateInPartition:  "default",
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	})
	db_v1_HealthCacheKey := testGenCacheKey(&structs.ServiceSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token",
			Filter: "Service.Meta.version == v1",
		},
		ServiceName:    "db",
		Connect:        true,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	})
	db_v2_HealthCacheKey := testGenCacheKey(&structs.ServiceSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token",
			Filter: "Service.Meta.version == v2",
		},
		ServiceName:    "db",
		Connect:        true,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	})

	db := structs.NewServiceName("db", nil)
	dbUID := NewUpstreamIDFromServiceName(db)

	// Create test cases using some of the common data above.
	tests := []*testcase_BasicLifecycle{
		{
			name: "simple-default-resolver",
			setup: func(t *testing.T, types *TestCacheTypes) {
				// Note that we deliberately leave the 'geo-cache' prepared query to time out
				types.health.Set(dbHealthCacheKey, &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, db.Name),
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
					ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
						Leaf: leaf,
						DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
							dbUID: dbDefaultChain(),
						},
						WatchedDiscoveryChains: map[UpstreamID]context.CancelFunc{},
						WatchedUpstreams:       nil, // Clone() clears this out
						WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
							dbUID: {
								"db.default.default.dc1": TestUpstreamNodes(t, db.Name),
							},
						},
						WatchedGateways: nil, // Clone() clears this out
						WatchedGatewayEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
							dbUID: {},
						},
						UpstreamConfig: map[UpstreamID]*structs.Upstream{
							NewUpstreamID(&upstreams[0]): &upstreams[0],
							NewUpstreamID(&upstreams[1]): &upstreams[1],
							NewUpstreamID(&upstreams[2]): &upstreams[2],
						},
						PassthroughUpstreams: map[UpstreamID]map[string]map[string]struct{}{},
					},
					PreparedQueryEndpoints: map[UpstreamID]structs.CheckServiceNodes{},
					WatchedServiceChecks:   map[structs.ServiceID][]structs.CheckType{},
					Intentions:             TestIntentions().Matches[0],
					IntentionsSet:          true,
				},
				Datacenter: "dc1",
				Locality:   GatewayKey{Datacenter: "dc1", Partition: structs.PartitionOrDefault("")},
			},
		},
		{
			name: "chain-resolver-with-version-split",
			setup: func(t *testing.T, types *TestCacheTypes) {
				// Note that we deliberately leave the 'geo-cache' prepared query to time out
				types.health.Set(db_v1_HealthCacheKey, &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, db.Name),
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
					ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
						Leaf: leaf,
						DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
							dbUID: dbSplitChain(),
						},
						WatchedDiscoveryChains: map[UpstreamID]context.CancelFunc{},
						WatchedUpstreams:       nil, // Clone() clears this out
						WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
							dbUID: {
								"v1.db.default.default.dc1": TestUpstreamNodes(t, db.Name),
								"v2.db.default.default.dc1": TestUpstreamNodesAlternate(t),
							},
						},
						WatchedGateways: nil, // Clone() clears this out
						WatchedGatewayEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
							dbUID: {},
						},
						UpstreamConfig: map[UpstreamID]*structs.Upstream{
							NewUpstreamID(&upstreams[0]): &upstreams[0],
							NewUpstreamID(&upstreams[1]): &upstreams[1],
							NewUpstreamID(&upstreams[2]): &upstreams[2],
						},
						PassthroughUpstreams: map[UpstreamID]map[string]map[string]struct{}{},
					},
					PreparedQueryEndpoints: map[UpstreamID]structs.CheckServiceNodes{},
					WatchedServiceChecks:   map[structs.ServiceID][]structs.CheckType{},
					Intentions:             TestIntentions().Matches[0],
					IntentionsSet:          true,
				},
				Datacenter: "dc1",
				Locality:   GatewayKey{Datacenter: "dc1", Partition: structs.PartitionOrDefault("")},
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
			types.intentions.Set(intentionCacheKey, TestIntentions())
			tt.setup(t, types)

			expectSnapCopy, err := copystructure.Copy(tt.expectSnap)
			require.NoError(t, err)

			webProxyCopy, err := copystructure.Copy(webProxy)
			require.NoError(t, err)

			testManager_BasicLifecycle(t, types,
				rootsCacheKey, leafCacheKey,
				roots,
				webProxyCopy.(*structs.NodeService),
				local.Config{},
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
	types *TestCacheTypes,
	rootsCacheKey, leafCacheKey string,
	roots *structs.IndexedCARoots,
	webProxy *structs.NodeService,
	agentConfig local.Config,
	expectSnap *ConfigSnapshot,
) {
	c := TestCacheWithTypes(t, types)

	logger := testutil.Logger(t)
	state := local.NewState(agentConfig, logger, &token.Store{})
	source := &structs.QuerySource{Datacenter: "dc1"}

	// Stub state syncing
	state.TriggerSyncChanges = func() {}

	// Create manager
	m, err := NewManager(ManagerConfig{
		Cache:  c,
		Health: &health.Client{Cache: c, CacheName: cachetype.HealthServicesName},
		State:  state,
		Source: source,
		Logger: logger,
	})
	require.NoError(t, err)

	// And run it
	go func() {
		err := m.Run()
		require.NoError(t, err)
	}()

	// BEFORE we register, we should be able to get a watch channel
	wCh, cancel := m.Watch(webProxy.CompoundServiceID())
	defer cancel()

	// And it should block with nothing sent on it yet
	assertWatchChanBlocks(t, wCh)

	require.NoError(t, state.AddService(webProxy, "my-token"))

	// We should see the initial config delivered but not until after the
	// coalesce timeout
	start := time.Now()
	assertWatchChanRecvs(t, wCh, expectSnap)
	require.True(t, time.Since(start) >= coalesceTimeout)

	assertLastReqArgs(t, types, "my-token", source)

	// Update NodeConfig
	webProxy.Port = 7777
	require.NoError(t, state.AddService(webProxy, "my-token"))

	expectSnap.Port = 7777
	assertWatchChanRecvs(t, wCh, expectSnap)

	// Register a second watcher
	wCh2, cancel2 := m.Watch(webProxy.CompoundServiceID())
	defer cancel2()

	// New watcher should immediately receive the current state
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Change token
	require.NoError(t, state.AddService(webProxy, "other-token"))
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
	require.NoError(t, state.AddService(webProxy, "other-token"))

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
	require.NoError(t, m.Close())

	// Sanity check the state is clean
	m.mu.Lock()
	defer m.mu.Unlock()
	require.Len(t, m.proxies, 0)
	require.Len(t, m.watchers, 0)
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
		Cache: cache.New(cache.Options{EntryFetchRate: rate.Inf, EntryFetchMaxBurst: 2}),
		State: local.NewState(local.Config{}, logger, &token.Store{}),
		Source: &structs.QuerySource{
			Node:       "node1",
			Datacenter: "dc1",
		},
		Logger: logger,
	}

	m, err := NewManager(cfg)
	require.NoError(t, err)

	snap1 := &ConfigSnapshot{
		ProxyID: structs.NewServiceID("test-proxy", nil),
		Port:    1111,
	}
	snap2 := &ConfigSnapshot{
		ProxyID: structs.NewServiceID("test-proxy", nil),
		Port:    2222,
	}

	// test 1 buffered chan
	ch1 := make(chan *ConfigSnapshot, 1)

	// Sending to an unblocked chan should work
	m.deliverLatest(snap1, ch1)

	// Check it was delivered
	require.Equal(t, snap1, <-ch1)

	// Now send both without reading simulating a slow client
	m.deliverLatest(snap1, ch1)
	m.deliverLatest(snap2, ch1)

	// Check we got the _second_ one
	require.Equal(t, snap2, <-ch1)

	// Same again for 5-buffered chan
	ch5 := make(chan *ConfigSnapshot, 5)

	// Sending to an unblocked chan should work
	m.deliverLatest(snap1, ch5)

	// Check it was delivered
	require.Equal(t, snap1, <-ch5)

	// Now send enough to fill the chan simulating a slow client
	for i := 0; i < 5; i++ {
		m.deliverLatest(snap1, ch5)
	}
	m.deliverLatest(snap2, ch5)

	// Check we got the _second_ one
	require.Equal(t, snap2, <-ch5)
}

func testGenCacheKey(req cache.Request) string {
	info := req.CacheInfo()
	return path.Join(info.Key, info.Datacenter)
}

func TestManager_SyncState_DefaultToken(t *testing.T) {
	types := NewTestCacheTypes(t)
	c := TestCacheWithTypes(t, types)
	logger := testutil.Logger(t)
	tokens := new(token.Store)
	tokens.UpdateUserToken("default-token", token.TokenSourceConfig)

	state := local.NewState(local.Config{}, logger, tokens)
	state.TriggerSyncChanges = func() {}

	m, err := NewManager(ManagerConfig{
		Cache:  c,
		Health: &health.Client{Cache: c, CacheName: cachetype.HealthServicesName},
		State:  state,
		Tokens: tokens,
		Source: &structs.QuerySource{Datacenter: "dc1"},
		Logger: logger,
	})
	require.NoError(t, err)
	defer m.Close()

	srv := &structs.NodeService{
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
		},
	}

	err = state.AddServiceWithChecks(srv, nil, "")
	require.NoError(t, err)
	m.syncState(m.notifyBroadcast)

	require.Equal(t, "default-token", m.proxies[srv.CompoundServiceID()].serviceInstance.token)
}

func TestManager_SyncState_No_Notify(t *testing.T) {
	types := NewTestCacheTypes(t)
	c := TestCacheWithTypes(t, types)
	logger := testutil.Logger(t)
	tokens := new(token.Store)
	tokens.UpdateUserToken("default-token", token.TokenSourceConfig)

	state := local.NewState(local.Config{}, logger, tokens)
	state.TriggerSyncChanges = func() {}

	m, err := NewManager(ManagerConfig{
		Cache:  c,
		Health: &health.Client{Cache: c, CacheName: cachetype.HealthServicesName},
		State:  state,
		Tokens: tokens,
		Source: &structs.QuerySource{Datacenter: "dc1"},
		Logger: logger,
	})
	require.NoError(t, err)
	defer m.Close()

	srv := &structs.NodeService{
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
		},
	}

	err = state.AddServiceWithChecks(srv, nil, "")
	require.NoError(t, err)

	readEvent := make(chan bool, 1)
	snapSent := make(chan bool, 1)

	m.syncState(func(ch <-chan ConfigSnapshot) {
		for {
			<-readEvent
			snap := <-ch
			m.notify(&snap)
			snapSent <- true
		}
	})

	// Get the relevant notification Channel, should only have 1
	notifyCH := m.proxies[srv.CompoundServiceID()].ch

	// update the leaf certs
	roots, issuedCert := TestCerts(t)
	notifyCH <- cache.UpdateEvent{
		CorrelationID: leafWatchID,
		Result:        issuedCert,
		Err:           nil,
	}
	// at this point the snapshot should not be valid and not be sent
	after := time.After(200 * time.Millisecond)
	select {
	case <-snapSent:
		t.Fatal("snap should not be valid")
	case <-after:

	}

	// update the root certs
	notifyCH <- cache.UpdateEvent{
		CorrelationID: rootsWatchID,
		Result:        roots,
		Err:           nil,
	}

	// at this point the snapshot should not be valid and not be sent
	after = time.After(200 * time.Millisecond)
	select {
	case <-snapSent:
		t.Fatal("snap should not be valid")
	case <-after:

	}

	// prepare to read a snapshot update as the next update should make the snapshot valid
	readEvent <- true

	// update the intentions
	notifyCH <- cache.UpdateEvent{
		CorrelationID: intentionsWatchID,
		Result:        &structs.IndexedIntentionMatches{},
		Err:           nil,
	}

	// at this point we have a valid snapshot
	after = time.After(500 * time.Millisecond)
	select {
	case <-snapSent:
	case <-after:
		t.Fatal("snap should be valid")

	}

	// send two snapshots back to back without reading them to overflow the snapshot channel and get to the default use case
	for i := 0; i < 2; i++ {
		time.Sleep(250 * time.Millisecond)
		notifyCH <- cache.UpdateEvent{
			CorrelationID: leafWatchID,
			Result:        issuedCert,
			Err:           nil,
		}
	}

	// make sure that we are not receiving any snapshot and wait for the snapshots to be processed
	after = time.After(500 * time.Millisecond)
	select {
	case <-snapSent:
		t.Fatal("snap should not be sent")
	case <-after:
	}

	// now make sure that both snapshots got propagated
	for i := 0; i < 2; i++ {

		readEvent <- true
		after = time.After(500 * time.Millisecond)
		select {
		case <-snapSent:
		case <-after:
			t.Fatal("snap should be valid")

		}
	}
}
