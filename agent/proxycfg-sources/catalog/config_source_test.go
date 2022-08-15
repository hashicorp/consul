package catalog

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
)

func TestConfigSource_Success(t *testing.T) {
	serviceID := structs.NewServiceID("web-sidecar-proxy-1", nil)
	nodeName := "node-name"
	token := "token"

	store := testStateStore(t)

	// Register the proxy in the catalog/state store at port 9999.
	require.NoError(t, store.EnsureRegistration(0, &structs.RegisterRequest{
		Node: nodeName,
		Service: &structs.NodeService{
			ID:      serviceID.ID,
			Service: "web-sidecar-proxy",
			Port:    9999,
			Kind:    structs.ServiceKindConnectProxy,
		},
	}))

	// testConfigManager builds a ConfigManager that emits a ConfigSnapshot whenever
	// Register is called, and closes the watch channel when Deregister is called.
	//
	// Though a little odd, this allows us to make assertions on the sync goroutine's
	// behavior without sleeping which leads to slow/racy tests.
	cfgMgr := testConfigManager(t, serviceID, nodeName, token)

	mgr := NewConfigSource(Config{
		Manager:    cfgMgr,
		LocalState: testLocalState(t),
		Logger:     hclog.NewNullLogger(),
		GetStore:   func() Store { return store },
	})
	t.Cleanup(mgr.Shutdown)

	snapCh, cancelWatch1, err := mgr.Watch(serviceID, nodeName, token)
	require.NoError(t, err)

	// Expect Register to have been called with the proxy's inital port.
	select {
	case snap := <-snapCh:
		require.Equal(t, 9999, snap.Port)
		require.Equal(t, token, snap.ProxyID.Token)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for snapshot")
	}

	// Update the proxy's port to 8888.
	require.NoError(t, store.EnsureRegistration(0, &structs.RegisterRequest{
		Node: nodeName,
		Service: &structs.NodeService{
			ID:      serviceID.ID,
			Service: "web-sidecar-proxy",
			Port:    8888,
			Kind:    structs.ServiceKindConnectProxy,
		},
	}))

	// Expect Register to have been called again with the proxy's new port.
	select {
	case snap := <-snapCh:
		require.Equal(t, 8888, snap.Port)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for snapshot")
	}

	// Start another watch.
	_, cancelWatch2, err := mgr.Watch(serviceID, nodeName, token)
	require.NoError(t, err)

	// Expect the service to have not been re-registered by the second watch.
	select {
	case <-snapCh:
		t.Fatal("service shouldn't have been re-registered")
	case <-time.After(100 * time.Millisecond):
	}

	// Expect cancelling the first watch to *not* de-register the service.
	cancelWatch1()
	select {
	case <-snapCh:
		t.Fatal("service shouldn't have been de-registered until other watch went away")
	case <-time.After(100 * time.Millisecond):
	}

	// Expect cancelling the other watch to de-register the service.
	cancelWatch2()
	select {
	case _, ok := <-snapCh:
		require.False(t, ok, "channel should've been closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for service to be de-registered")
	}
}

func TestConfigSource_LocallyManagedService(t *testing.T) {
	serviceID := structs.NewServiceID("web-sidecar-proxy-1", nil)
	nodeName := "node-1"
	token := "token"

	localState := testLocalState(t)
	localState.AddServiceWithChecks(&structs.NodeService{ID: serviceID.ID}, nil, "")

	localWatcher := NewMockWatcher(t)
	localWatcher.On("Watch", serviceID, nodeName, token).
		Return(make(<-chan *proxycfg.ConfigSnapshot), proxycfg.CancelFunc(func() {}), nil)

	mgr := NewConfigSource(Config{
		NodeName:          nodeName,
		LocalState:        localState,
		LocalConfigSource: localWatcher,
		Logger:            hclog.NewNullLogger(),
		GetStore:          func() Store { panic("state store shouldn't have been used") },
	})
	t.Cleanup(mgr.Shutdown)

	_, _, err := mgr.Watch(serviceID, nodeName, token)
	require.NoError(t, err)
}

func TestConfigSource_ErrorRegisteringService(t *testing.T) {
	serviceID := structs.NewServiceID("web-sidecar-proxy-1", nil)
	nodeName := "node-name"
	token := "token"

	store := testStateStore(t)

	require.NoError(t, store.EnsureRegistration(0, &structs.RegisterRequest{
		Node: nodeName,
		Service: &structs.NodeService{
			ID:      serviceID.ID,
			Service: "web-sidecar-proxy",
			Port:    9999,
			Kind:    structs.ServiceKindConnectProxy,
		},
	}))

	var canceledWatch bool
	cancel := proxycfg.CancelFunc(func() { canceledWatch = true })

	cfgMgr := NewMockConfigManager(t)

	cfgMgr.On("Watch", mock.Anything).
		Return(make(<-chan *proxycfg.ConfigSnapshot), cancel)

	cfgMgr.On("Register", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("KABOOM"))

	mgr := NewConfigSource(Config{
		Manager:    cfgMgr,
		LocalState: testLocalState(t),
		Logger:     hclog.NewNullLogger(),
		GetStore:   func() Store { return store },
	})
	t.Cleanup(mgr.Shutdown)

	_, _, err := mgr.Watch(serviceID, nodeName, token)
	require.Error(t, err)
	require.True(t, canceledWatch, "watch should've been canceled")
}

func TestConfigSource_NotProxyService(t *testing.T) {
	serviceID := structs.NewServiceID("web", nil)
	nodeName := "node-name"
	token := "token"

	store := testStateStore(t)

	require.NoError(t, store.EnsureRegistration(0, &structs.RegisterRequest{
		Node: nodeName,
		Service: &structs.NodeService{
			ID:      serviceID.ID,
			Service: "web",
			Port:    9999,
			Kind:    structs.ServiceKindTypical,
		},
	}))

	var canceledWatch bool
	cancel := proxycfg.CancelFunc(func() { canceledWatch = true })

	cfgMgr := NewMockConfigManager(t)

	cfgMgr.On("Watch", mock.Anything).
		Return(make(<-chan *proxycfg.ConfigSnapshot), cancel)

	mgr := NewConfigSource(Config{
		Manager:    cfgMgr,
		LocalState: testLocalState(t),
		Logger:     hclog.NewNullLogger(),
		GetStore:   func() Store { return store },
	})
	t.Cleanup(mgr.Shutdown)

	_, _, err := mgr.Watch(serviceID, nodeName, token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a sidecar proxy or gateway")
	require.True(t, canceledWatch, "watch should've been canceled")
}

func testConfigManager(t *testing.T, serviceID structs.ServiceID, nodeName string, token string) ConfigManager {
	t.Helper()

	cfgMgr := NewMockConfigManager(t)

	proxyID := proxycfg.ProxyID{
		ServiceID: serviceID,
		NodeName:  nodeName,
		Token:     token,
	}

	snapCh := make(chan *proxycfg.ConfigSnapshot, 1)
	cfgMgr.On("Watch", proxyID).
		Return((<-chan *proxycfg.ConfigSnapshot)(snapCh), proxycfg.CancelFunc(func() {}), nil)

	cfgMgr.On("Register", mock.Anything, mock.Anything, source, token, false).
		Run(func(args mock.Arguments) {
			id := args.Get(0).(proxycfg.ProxyID)
			ns := args.Get(1).(*structs.NodeService)

			snapCh <- &proxycfg.ConfigSnapshot{
				ProxyID: id,
				Port:    ns.Port,
			}
		}).
		Return(nil)

	cfgMgr.On("Deregister", proxyID, source).
		Run(func(mock.Arguments) { close(snapCh) }).
		Return()

	return cfgMgr
}

func testStateStore(t *testing.T) *state.Store {
	t.Helper()

	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)
	return state.NewStateStoreWithEventPublisher(gc, stream.NoOpEventPublisher{})
}

func testLocalState(t *testing.T) *local.State {
	t.Helper()

	l := local.NewState(local.Config{}, hclog.NewNullLogger(), &token.Store{})
	l.TriggerSyncChanges = func() {}
	return l
}
