package proxycfg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
)

const testSource ProxySource = "test"

func mustCopyProxyConfig(t *testing.T, ns *structs.NodeService) structs.ConnectProxyConfig {
	cfg, err := copyProxyConfig(ns)
	require.NoError(t, err)
	return cfg
}

// assertLastReqArgs verifies that each request type had the correct source
// parameters (e.g. Datacenter name) and token.
func assertLastReqArgs(t *testing.T, dataSources *TestDataSources, token string, source *structs.QuerySource) {
	t.Helper()
	// Roots needs correct DC and token
	rootReq := dataSources.CARoots.LastReq()
	require.Equal(t, token, rootReq.Token)
	require.Equal(t, source.Datacenter, rootReq.Datacenter)

	// Leaf needs correct DC and token
	leafReq := dataSources.LeafCertificate.LastReq()
	require.Equal(t, token, leafReq.Token)
	require.Equal(t, source.Datacenter, leafReq.Datacenter)

	// Intentions needs correct DC and token
	intReq := dataSources.Intentions.LastReq()
	require.Equal(t, token, intReq.Token)
	require.Equal(t, source.Datacenter, intReq.Datacenter)
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

	rootsReq := &structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token"},
	}
	leafReq := &cachetype.ConnectCALeafRequest{
		Datacenter: "dc1",
		Token:      "my-token",
		Service:    "web",
	}

	intentionReq := &structs.ServiceSpecificRequest{
		Datacenter:     "dc1",
		QueryOptions:   structs.QueryOptions{Token: "my-token"},
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		ServiceName:    "web",
	}

	meshConfigReq := &structs.ConfigEntryQuery{
		Datacenter:     "dc1",
		QueryOptions:   structs.QueryOptions{Token: "my-token"},
		Kind:           structs.MeshConfig,
		Name:           structs.MeshConfigMesh,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	dbChainReq := &structs.DiscoveryChainRequest{
		Name:                 "db",
		EvaluateInDatacenter: "dc1",
		EvaluateInNamespace:  "default",
		EvaluateInPartition:  "default",
		// This is because structs.TestUpstreams uses an opaque config
		// to override connect timeouts.
		OverrideConnectTimeout: 1 * time.Second,
		Datacenter:             "dc1",
		QueryOptions:           structs.QueryOptions{Token: "my-token"},
	}

	dbHealthReq := &structs.ServiceSpecificRequest{
		Datacenter:     "dc1",
		QueryOptions:   structs.QueryOptions{Token: "my-token", Filter: ""},
		ServiceName:    "db",
		Connect:        true,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	db_v1_HealthReq := &structs.ServiceSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token",
			Filter: "Service.Meta.version == v1",
		},
		ServiceName:    "db",
		Connect:        true,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	db_v2_HealthReq := &structs.ServiceSpecificRequest{
		Datacenter: "dc1",
		QueryOptions: structs.QueryOptions{Token: "my-token",
			Filter: "Service.Meta.version == v2",
		},
		ServiceName:    "db",
		Connect:        true,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	db := structs.NewServiceName("db", nil)
	dbUID := NewUpstreamIDFromServiceName(db)

	// Create test cases using some of the common data above.
	tests := []*testcase_BasicLifecycle{
		{
			name: "simple-default-resolver",
			setup: func(t *testing.T, dataSources *TestDataSources) {
				// Note that we deliberately leave the 'geo-cache' prepared query to time out
				dataSources.Health.Set(dbHealthReq, &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, db.Name),
				})
				dataSources.CompiledDiscoveryChain.Set(dbChainReq, &structs.DiscoveryChainResponse{
					Chain: dbDefaultChain(),
				})
			},
			expectSnap: &ConfigSnapshot{
				Kind:            structs.ServiceKindConnectProxy,
				Service:         webProxy.Service,
				ProxyID:         ProxyID{ServiceID: webProxy.CompoundServiceID()},
				Address:         webProxy.Address,
				Port:            webProxy.Port,
				Proxy:           mustCopyProxyConfig(t, webProxy),
				ServiceMeta:     webProxy.Meta,
				TaggedAddresses: make(map[string]structs.ServiceAddress),
				Roots:           roots,
				ConnectProxy: configSnapshotConnectProxy{
					ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
						Leaf:          leaf,
						MeshConfigSet: true,
						DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
							dbUID: dbDefaultChain(),
						},
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
						PassthroughUpstreams:              map[UpstreamID]map[string]map[string]struct{}{},
						PassthroughIndices:                map[string]indexedTarget{},
						UpstreamPeerTrustBundles:          watch.NewMap[PeerName, *pbpeering.PeeringTrustBundle](),
						PeerUpstreamEndpoints:             watch.NewMap[UpstreamID, structs.CheckServiceNodes](),
						PeerUpstreamEndpointsUseHostnames: map[UpstreamID]struct{}{},
					},
					PreparedQueryEndpoints: map[UpstreamID]structs.CheckServiceNodes{},
					DestinationsUpstream:   watch.NewMap[UpstreamID, *structs.ServiceConfigEntry](),
					DestinationGateways:    watch.NewMap[UpstreamID, structs.CheckServiceNodes](),
					WatchedServiceChecks:   map[structs.ServiceID][]structs.CheckType{},
					Intentions:             TestIntentions(),
					IntentionsSet:          true,
				},
				Datacenter: "dc1",
				Locality:   GatewayKey{Datacenter: "dc1", Partition: acl.PartitionOrDefault("")},
			},
		},
		{
			name: "chain-resolver-with-version-split",
			setup: func(t *testing.T, dataSources *TestDataSources) {
				// Note that we deliberately leave the 'geo-cache' prepared query to time out
				dataSources.Health.Set(db_v1_HealthReq, &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, db.Name),
				})
				dataSources.Health.Set(db_v2_HealthReq, &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodesAlternate(t),
				})
				dataSources.CompiledDiscoveryChain.Set(dbChainReq, &structs.DiscoveryChainResponse{
					Chain: dbSplitChain(),
				})
			},
			expectSnap: &ConfigSnapshot{
				Kind:            structs.ServiceKindConnectProxy,
				Service:         webProxy.Service,
				ProxyID:         ProxyID{ServiceID: webProxy.CompoundServiceID()},
				Address:         webProxy.Address,
				Port:            webProxy.Port,
				Proxy:           mustCopyProxyConfig(t, webProxy),
				ServiceMeta:     webProxy.Meta,
				TaggedAddresses: make(map[string]structs.ServiceAddress),
				Roots:           roots,
				ConnectProxy: configSnapshotConnectProxy{
					ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
						Leaf:          leaf,
						MeshConfigSet: true,
						DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
							dbUID: dbSplitChain(),
						},
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
						PassthroughUpstreams:              map[UpstreamID]map[string]map[string]struct{}{},
						PassthroughIndices:                map[string]indexedTarget{},
						UpstreamPeerTrustBundles:          watch.NewMap[PeerName, *pbpeering.PeeringTrustBundle](),
						PeerUpstreamEndpoints:             watch.NewMap[UpstreamID, structs.CheckServiceNodes](),
						PeerUpstreamEndpointsUseHostnames: map[UpstreamID]struct{}{},
					},
					PreparedQueryEndpoints: map[UpstreamID]structs.CheckServiceNodes{},
					DestinationsUpstream:   watch.NewMap[UpstreamID, *structs.ServiceConfigEntry](),
					DestinationGateways:    watch.NewMap[UpstreamID, structs.CheckServiceNodes](),
					WatchedServiceChecks:   map[structs.ServiceID][]structs.CheckType{},
					Intentions:             TestIntentions(),
					IntentionsSet:          true,
				},
				Datacenter: "dc1",
				Locality:   GatewayKey{Datacenter: "dc1", Partition: acl.PartitionOrDefault("")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.setup)
			require.NotNil(t, tt.expectSnap)

			// Setup initial values
			dataSources := NewTestDataSources()
			dataSources.LeafCertificate.Set(leafReq, leaf)
			dataSources.CARoots.Set(rootsReq, roots)
			dataSources.Intentions.Set(intentionReq, TestIntentions())
			dataSources.ConfigEntry.Set(meshConfigReq, &structs.ConfigEntryResponse{Entry: nil})
			tt.setup(t, dataSources)

			expectSnapCopy := tt.expectSnap.Clone()
			webProxyCopy := webProxy.DeepCopy()

			testManager_BasicLifecycle(t,
				dataSources,
				rootsReq, leafReq,
				roots,
				webProxyCopy,
				expectSnapCopy,
			)
		})
	}
}

type testcase_BasicLifecycle struct {
	name       string
	setup      func(t *testing.T, dataSources *TestDataSources)
	webProxy   *structs.NodeService
	expectSnap *ConfigSnapshot
}

func testManager_BasicLifecycle(
	t *testing.T,
	dataSources *TestDataSources,
	rootsReq *structs.DCSpecificRequest,
	leafReq *cachetype.ConnectCALeafRequest,
	roots *structs.IndexedCARoots,
	webProxy *structs.NodeService,
	expectSnap *ConfigSnapshot,
) {
	logger := testutil.Logger(t)
	source := &structs.QuerySource{Datacenter: "dc1"}

	// Create manager
	m, err := NewManager(ManagerConfig{
		Source:      source,
		Logger:      logger,
		DataSources: dataSources.ToDataSources(),
	})
	require.NoError(t, err)

	webProxyID := ProxyID{
		ServiceID: webProxy.CompoundServiceID(),
	}

	// BEFORE we register, we should be able to get a watch channel
	wCh, cancel := m.Watch(webProxyID)
	defer cancel()

	// And it should block with nothing sent on it yet
	assertWatchChanBlocks(t, wCh)

	require.NoError(t, m.Register(webProxyID, webProxy, testSource, "my-token", false))

	// We should see the initial config delivered but not until after the
	// coalesce timeout
	start := time.Now()
	assertWatchChanRecvs(t, wCh, expectSnap)
	require.True(t, time.Since(start) >= coalesceTimeout)

	assertLastReqArgs(t, dataSources, "my-token", source)

	// Update NodeConfig
	webProxy.Port = 7777
	require.NoError(t, m.Register(webProxyID, webProxy, testSource, "my-token", false))

	expectSnap.Port = 7777
	assertWatchChanRecvs(t, wCh, expectSnap)

	// Register a second watcher
	wCh2, cancel2 := m.Watch(webProxyID)
	defer cancel2()

	// New watcher should immediately receive the current state
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Change token
	require.NoError(t, m.Register(webProxyID, webProxy, testSource, "other-token", false))
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// This is actually sort of timing dependent - the cache background fetcher
	// will still be fetching with the old token, but we rely on the fact that our
	// mock type will have been blocked on those for a while.
	assertLastReqArgs(t, dataSources, "other-token", source)
	// Update roots
	newRoots, newLeaf := TestCerts(t)
	newRoots.Roots = append(newRoots.Roots, roots.Roots...)
	dataSources.CARoots.Set(rootsReq, newRoots)

	// Expect new roots in snapshot
	expectSnap.Roots = newRoots
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Update leaf
	dataSources.LeafCertificate.Set(leafReq, newLeaf)

	// Expect new roots in snapshot
	expectSnap.ConnectProxy.Leaf = newLeaf
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Remove the proxy
	m.Deregister(webProxyID, testSource)

	// Chan should NOT close
	assertWatchChanBlocks(t, wCh)
	assertWatchChanBlocks(t, wCh2)

	// Re-add the proxy with another new port
	webProxy.Port = 3333
	require.NoError(t, m.Register(webProxyID, webProxy, testSource, "other-token", false))

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
		Source: &structs.QuerySource{
			Node:       "node1",
			Datacenter: "dc1",
		},
		Logger: logger,
	}

	m, err := NewManager(cfg)
	require.NoError(t, err)

	snap1 := &ConfigSnapshot{
		ProxyID: ProxyID{ServiceID: structs.NewServiceID("test-proxy", nil)},
		Port:    1111,
	}
	snap2 := &ConfigSnapshot{
		ProxyID: ProxyID{ServiceID: structs.NewServiceID("test-proxy", nil)},
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

func TestManager_SyncState_No_Notify(t *testing.T) {
	dataSources := NewTestDataSources()
	logger := testutil.Logger(t)

	m, err := NewManager(ManagerConfig{
		Source:      &structs.QuerySource{Datacenter: "dc1"},
		Logger:      logger,
		DataSources: dataSources.ToDataSources(),
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

	proxyID := ProxyID{
		ServiceID: srv.CompoundServiceID(),
	}

	require.NoError(t, m.Register(proxyID, srv, testSource, "", false))

	watchCh, cancelWatch := m.Watch(proxyID)
	t.Cleanup(cancelWatch)

	// Get the relevant notification Channel, should only have 1
	notifyCH := m.proxies[proxyID].ch

	// update the leaf certs
	roots, issuedCert := TestCerts(t)
	notifyCH <- UpdateEvent{
		CorrelationID: leafWatchID,
		Result:        issuedCert,
		Err:           nil,
	}
	// at this point the snapshot should not be valid and not be sent
	after := time.After(200 * time.Millisecond)
	select {
	case <-watchCh:
		t.Fatal("snap should not be valid")
	case <-after:

	}

	// update the root certs
	notifyCH <- UpdateEvent{
		CorrelationID: rootsWatchID,
		Result:        roots,
		Err:           nil,
	}

	// at this point the snapshot should not be valid and not be sent
	after = time.After(200 * time.Millisecond)
	select {
	case <-watchCh:
		t.Fatal("snap should not be valid")
	case <-after:

	}

	// update the mesh config entry
	notifyCH <- UpdateEvent{
		CorrelationID: meshConfigEntryID,
		Result:        &structs.ConfigEntryResponse{},
		Err:           nil,
	}

	// at this point the snapshot should not be valid and not be sent
	after = time.After(200 * time.Millisecond)
	select {
	case <-watchCh:
		t.Fatal("snap should not be valid")
	case <-after:

	}

	// update the intentions
	notifyCH <- UpdateEvent{
		CorrelationID: intentionsWatchID,
		Result:        structs.Intentions{},
		Err:           nil,
	}

	// at this point we have a valid snapshot
	after = time.After(500 * time.Millisecond)
	select {
	case <-watchCh:
	case <-after:
		t.Fatal("snap should be valid")

	}
}
