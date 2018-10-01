package proxycfg

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
)

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
	// Use a mocked cache to make life simpler
	types := NewTestCacheTypes(t)
	c := TestCacheWithTypes(t, types)

	require := require.New(t)

	roots, leaf := TestCerts(t)

	// Setup initial values
	types.roots.value.Store(roots)
	types.leaf.value.Store(leaf)
	types.intentions.value.Store(TestIntentions(t))
	types.health.value.Store(
		&structs.IndexedCheckServiceNodes{
			Nodes: TestUpstreamNodes(t),
		})

	logger := log.New(os.Stderr, "", log.LstdFlags)
	state := local.NewState(local.Config{}, logger, &token.Store{})
	source := &structs.QuerySource{
		Node:       "node1",
		Datacenter: "dc1",
	}

	// Stub state syncing
	state.TriggerSyncChanges = func() {}

	// Create manager
	m := NewManager(logger, c, state, source)

	// Register a proxy for "web"
	webProxy := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-sidecar-proxy",
		Service: "web-sidecar-proxy",
		Port:    9999,
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

	// BEFORE we register, we should be able to get a watch channel
	wCh, cancel := m.Watch(webProxy.ID)
	defer cancel()

	// And it should block with nothing sent on it yet
	assertWatchChanBlocks(t, wCh)

	require.NoError(state.AddService(webProxy, "my-token"))

	// We should see the initial config delivered but not until after the
	// coallesce timeout
	expectSnap := &ConfigSnapshot{
		ProxyID: webProxy.ID,
		Address: webProxy.Address,
		Port:    webProxy.Port,
		Proxy:   webProxy.Proxy,
		Roots:   roots,
		Leaf:    leaf,
		UpstreamEndpoints: map[string]structs.CheckServiceNodes{
			"service:db": TestUpstreamNodes(t),
		},
	}
	start := time.Now()
	assertWatchChanRecvs(t, wCh, expectSnap)
	require.True(time.Since(start) >= coallesceTimeout)

	assertLastReqArgs(t, types, "my-token", source)

	// Update NodeConfig
	webProxy.Port = 7777
	require.NoError(state.AddService(webProxy, "my-token"))

	expectSnap.Port = 7777
	assertWatchChanRecvs(t, wCh, expectSnap)

	// Register a second watcher
	wCh2, cancel2 := m.Watch(webProxy.ID)
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
	types.roots.Set(newRoots)

	// Expect new roots in snapshot
	expectSnap.Roots = newRoots
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Update leaf
	types.leaf.Set(newLeaf)

	// Expect new roots in snapshot
	expectSnap.Leaf = newLeaf
	assertWatchChanRecvs(t, wCh, expectSnap)
	assertWatchChanRecvs(t, wCh2, expectSnap)

	// Remove the proxy
	state.RemoveService(webProxy.ID)

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
	case <-time.After(50*time.Millisecond + coallesceTimeout):
		t.Fatal("recv timeout")
	}
}
