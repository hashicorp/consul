package proxycfg

import (
	"log"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
)

// Manager is a component that integrates into the agent and manages Connect
// proxy configuration state. This is used both for "managed" and "unmanaged"
// proxies where "managed" refers to whether the _process_ itself is supervised.
//
// The Manager ensures that any Connect proxy registered on the agent has all
// the state it needs cached locally via the agent cache. State includes
// certificates, intentions, and service discovery results for any declared
// upstreams.
type Manager struct {
	// cache is the agent's cache instance that can be used to retreive, store and
	// monitor state for the proxies.
	cache *cache.Cache
	// state is the agent's local state to be watched for new proxy registrations.
	state *local.State
	// source describes the current agent's identity, it's used directly for
	// prepared query discovery but also indirectly as a way to pass current
	// Datacenter name into other request types that need it. This is sufficient
	// for now and cleaner than passing the entire RuntimeConfig.
	source *structs.QuerySource
	// logger is the agent's logger to be used for logging logs.
	logger *log.Logger

	// stateCh is notified for any service changes in local state. We only use
	// this to triger on _new_ service addition since it has no data and we don't
	// want to maintain a full copy of the state in order to diff and figure out
	// what changed. Luckily each service has it's own WatchCh so we can figure
	// out changes and removals with those efficiently.
	stateCh chan struct{}

	mu       sync.Mutex
	proxies  map[string]*State
	watchers map[string]map[uint64]chan *ConfigSnapshot
}

// NewManager constructs a manager from the provided agent cache.
func NewManager(logger *log.Logger, cache *cache.Cache, state *local.State,
	source *structs.QuerySource) *Manager {
	m := &Manager{
		logger: logger,
		cache:  cache,
		state:  state,
		source: source,
		// Single item buffer is enough since there is no data transferred so this
		// is "level triggering" and we can't miss actual data.
		stateCh:  make(chan struct{}, 1),
		proxies:  make(map[string]*State),
		watchers: make(map[string]map[uint64]chan *ConfigSnapshot),
	}
	go m.run()
	return m
}

// run is the internal state syncing loop that watches the local state for
// changes.
func (m *Manager) run() {

	m.mu.Lock()
	stateCh := m.stateCh
	m.mu.Unlock()

	if stateCh == nil {
		return
	}

	// Register for notifications about state changes
	m.state.Notify(stateCh)
	defer m.state.StopNotify(stateCh)

	for {
		m.mu.Lock()

		// Traverse the local state and ensure all proxy services are registered
		services := m.state.Services()
		for svcID, svc := range services {
			if svc.Kind != structs.ServiceKindConnectProxy {
				continue
			}
			// TODO(banks): need to work out when to default some stuff. For example
			// Proxy.LocalServicePort is practially necessary for any sidecar and can
			// default to the port of the sidecar service, but only if it's already
			// registered and once we get past here, we don't have enough context to
			// know that so we'd need to set it here if not during registration of the
			// proxy service. Sidecar Service and managed proxies in the interim can
			// do that, but we should validate more generally that that is always
			// true.
			err := m.ensureProxyServiceLocked(svc, m.state.ServiceToken(svcID))
			if err != nil {
				m.logger.Printf("[ERR] failed to watch proxy service %s: %s", svc.ID,
					err)
			}
		}

		// Now see if any proxies were removed
		for proxyID := range m.proxies {
			if _, ok := services[proxyID]; !ok {
				// Remove them
				m.removeProxyServiceLocked(proxyID)
			}
		}

		m.mu.Unlock()

		// Wait for a state change
		_, ok := <-stateCh
		if !ok {
			// Stopped
			return
		}
	}
}

// ensureProxyServiceLocked adss or changes the proxy to our state.
func (m *Manager) ensureProxyServiceLocked(ns *structs.NodeService, token string) error {
	state, ok := m.proxies[ns.ID]

	if ok {
		if !state.Changed(ns, token) {
			// No change
			return nil
		}

		// We are updating the proxy, close it's old state
		state.Close()
	}

	var err error
	state, err = NewState(m.logger, m.cache, m.source, ns, token)
	if err != nil {
		return err
	}
	m.proxies[ns.ID] = state

	// Start a goroutine that will wait for changes and broadcast them to watchers.
	go func() {
		ch := state.Watch()
		for {
			select {
			case snap, ok := <-ch:
				if !ok {
					// State closed
					return
				}
				m.notify(&snap)
			}
		}
	}()

	return nil
}

// removeProxyService is called when a service deregisteres and frees all
// resources for that service.
func (m *Manager) removeProxyServiceLocked(proxyID string) {
	state, ok := m.proxies[proxyID]
	if !ok {
		return
	}

	// Closing state will let the goroutine we started in Ensure finish since
	// watch chan is closed.
	state.Close()
	delete(m.proxies, proxyID)

	// We intentionally leave potential watchers hanging here - there is no new
	// config for them and closing their channels might be indistinguishable from
	// an error that they should retry. We rely for them to eventually give up
	// (because they are in fact not running any more) and so the watches be
	// cleaned up naturally.
}

func (m *Manager) notify(snap *ConfigSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	watchers, ok := m.watchers[snap.ProxyID]
	if !ok {
		return
	}

	for _, ch := range watchers {
		// Attempt delivery but don't let slow consumers block us forever. They
		// might miss updates but it's better than breaking everything.
		//
		// TODO(banks): should we close their chan here to force them to eventually
		// notice they are too slow? Not sure if it really helps.
		select {
		case ch <- snap:
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Watch registers a watch on a proxy. It might not exist yet in which case this
// will not fail, but no updates will be delivered until the proxy is
// registered. If there is already a valid snapshot in memory, it will be
// delivered immediately.
func (m *Manager) Watch(proxyID string) (<-chan *ConfigSnapshot, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *ConfigSnapshot, 1)
	watchers, ok := m.watchers[proxyID]
	if !ok {
		watchers = make(map[uint64]chan *ConfigSnapshot)
	}
	idx := uint64(len(watchers))
	watchers[idx] = ch
	m.watchers[proxyID] = watchers

	// Deliver the current snapshot immediately if there is one ready
	if state, ok := m.proxies[proxyID]; ok {
		if snap := state.CurrentSnapshot(); snap != nil {
			ch <- snap
		}
	}

	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.closeWatchLocked(proxyID, idx)
	}
}

// closeWatchLocked cleans up state related to a single watcher. It assumes the
// lock is held.
func (m *Manager) closeWatchLocked(proxyID string, watchIdx uint64) {
	if watchers, ok := m.watchers[proxyID]; ok {
		if ch, ok := watchers[watchIdx]; ok {
			delete(watchers, watchIdx)
			close(ch)
			if len(watchers) == 0 {
				delete(m.watchers, proxyID)
			}
		}
	}
}

// Close removes all state and stops all running goroutines.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stateCh != nil {
		close(m.stateCh)
		m.stateCh = nil
	}

	// Close all current watchers first
	for proxyID, watchers := range m.watchers {
		for idx := range watchers {
			m.closeWatchLocked(proxyID, idx)
		}
	}

	// The close all states
	for proxyID, state := range m.proxies {
		state.Close()
		delete(m.proxies, proxyID)
	}
	return nil
}
