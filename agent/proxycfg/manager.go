package proxycfg

import (
	"errors"
	"sync"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
)

var (
	// ErrStopped is returned from Run if the manager instance has already been
	// stopped.
	ErrStopped = errors.New("manager stopped")

	// ErrStarted is returned from Run if the manager instance has already run.
	ErrStarted = errors.New("manager was already run")
)

// CancelFunc is a type for a returned function that can be called to cancel a
// watch.
type CancelFunc func()

// Manager is a component that integrates into the agent and manages Connect
// proxy configuration state. This should not be confused with the deprecated
// "managed proxy" concept where the agent supervises the actual proxy process.
// proxycfg.Manager is oblivious to the distinction and manages state for any
// service registered with Kind == connect-proxy.
//
// The Manager ensures that any Connect proxy registered on the agent has all
// the state it needs cached locally via the agent cache. State includes
// certificates, intentions, and service discovery results for any declared
// upstreams. See package docs for more detail.
type Manager struct {
	ManagerConfig

	// stateCh is notified for any service changes in local state. We only use
	// this to trigger on _new_ service addition since it has no data and we don't
	// want to maintain a full copy of the state in order to diff and figure out
	// what changed. Luckily each service has it's own WatchCh so we can figure
	// out changes and removals with those efficiently.
	stateCh chan struct{}

	mu       sync.Mutex
	started  bool
	proxies  map[structs.ServiceID]*state
	watchers map[structs.ServiceID]map[uint64]chan *ConfigSnapshot
}

// ManagerConfig holds the required external dependencies for a Manager
// instance. All fields must be set to something valid or the manager will
// panic. The ManagerConfig is passed by value to NewManager so the passed value
// can be mutated safely.
type ManagerConfig struct {
	// Cache is the agent's cache instance that can be used to retrieve, store and
	// monitor state for the proxies.
	Cache *cache.Cache
	// state is the agent's local state to be watched for new proxy registrations.
	State *local.State
	// source describes the current agent's identity, it's used directly for
	// prepared query discovery but also indirectly as a way to pass current
	// Datacenter name into other request types that need it. This is sufficient
	// for now and cleaner than passing the entire RuntimeConfig.
	Source *structs.QuerySource
	// logger is the agent's logger to be used for logging logs.
	Logger          hclog.Logger
	TLSConfigurator *tlsutil.Configurator
}

// NewManager constructs a manager from the provided agent cache.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.Cache == nil || cfg.State == nil || cfg.Source == nil ||
		cfg.Logger == nil {
		return nil, errors.New("all ManagerConfig fields must be provided")
	}
	m := &Manager{
		ManagerConfig: cfg,
		// Single item buffer is enough since there is no data transferred so this
		// is "level triggering" and we can't miss actual data.
		stateCh:  make(chan struct{}, 1),
		proxies:  make(map[structs.ServiceID]*state),
		watchers: make(map[structs.ServiceID]map[uint64]chan *ConfigSnapshot),
	}
	return m, nil
}

// Run is the long-running method that handles state syncing. It should be run
// in it's own goroutine and will continue until a fatal error is hit or Close
// is called. Run will return an error if it is called more than once, or called
// after Close.
func (m *Manager) Run() error {
	m.mu.Lock()
	alreadyStarted := m.started
	m.started = true
	stateCh := m.stateCh
	m.mu.Unlock()

	// Protect against multiple Run calls.
	if alreadyStarted {
		return ErrStarted
	}

	// Protect against being run after Close.
	if stateCh == nil {
		return ErrStopped
	}

	// Register for notifications about state changes
	m.State.Notify(stateCh)
	defer m.State.StopNotify(stateCh)

	for {
		m.syncState()

		// Wait for a state change
		_, ok := <-stateCh
		if !ok {
			// Stopped
			return nil
		}
	}
}

// syncState is called whenever the local state notifies a change. It holds the
// lock while finding any new or updated proxies and removing deleted ones.
func (m *Manager) syncState() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Traverse the local state and ensure all proxy services are registered
	services := m.State.Services(structs.WildcardEnterpriseMeta())
	for sid, svc := range services {
		if svc.Kind != structs.ServiceKindConnectProxy && svc.Kind != structs.ServiceKindMeshGateway {
			continue
		}
		// TODO(banks): need to work out when to default some stuff. For example
		// Proxy.LocalServicePort is practically necessary for any sidecar and can
		// default to the port of the sidecar service, but only if it's already
		// registered and once we get past here, we don't have enough context to
		// know that so we'd need to set it here if not during registration of the
		// proxy service. Sidecar Service in the interim can do that, but we should
		// validate more generally that that is always true.
		err := m.ensureProxyServiceLocked(svc, m.State.ServiceToken(sid))
		if err != nil {
			m.Logger.Error("failed to watch proxy service",
				"service", sid.String(),
				"error", err,
			)
		}
	}

	// Now see if any proxies were removed
	for proxyID := range m.proxies {
		if _, ok := services[proxyID]; !ok {
			// Remove them
			m.removeProxyServiceLocked(proxyID)
		}
	}
}

// ensureProxyServiceLocked adds or changes the proxy to our state.
func (m *Manager) ensureProxyServiceLocked(ns *structs.NodeService, token string) error {
	sid := ns.CompoundServiceID()
	state, ok := m.proxies[sid]

	if ok {
		if !state.Changed(ns, token) {
			// No change
			return nil
		}

		// We are updating the proxy, close its old state
		state.Close()
	}

	var err error
	state, err = newState(ns, token)
	if err != nil {
		return err
	}

	// Set the necessary dependencies
	state.logger = m.Logger
	state.cache = m.Cache
	state.source = m.Source
	if m.TLSConfigurator != nil {
		state.serverSNIFn = m.TLSConfigurator.ServerSNI
	}

	ch, err := state.Watch()
	if err != nil {
		return err
	}
	m.proxies[sid] = state

	// Start a goroutine that will wait for changes and broadcast them to watchers.
	go func(ch <-chan ConfigSnapshot) {
		// Run until ch is closed
		for snap := range ch {
			m.notify(&snap)
		}
	}(ch)

	return nil
}

// removeProxyService is called when a service deregisters and frees all
// resources for that service.
func (m *Manager) removeProxyServiceLocked(proxyID structs.ServiceID) {
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
		m.deliverLatest(snap, ch)
	}
}

// deliverLatest delivers the snapshot to a watch chan. If the delivery blocks,
// it will drain the chan and then re-attempt delivery so that a slow consumer
// gets the latest config earlier. This MUST be called from a method where m.mu
// is held to be safe since it assumes we are the only goroutine sending on ch.
func (m *Manager) deliverLatest(snap *ConfigSnapshot, ch chan *ConfigSnapshot) {
	// Send if chan is empty
	select {
	case ch <- snap:
		return
	default:
	}

	// Not empty, drain the chan of older snapshots and redeliver. For now we only
	// use 1-buffered chans but this will still work if we change that later.
OUTER:
	for {
		select {
		case <-ch:
			continue
		default:
			break OUTER
		}
	}

	// Now send again
	select {
	case ch <- snap:
		return
	default:
		// This should not be possible since we should be the only sender, enforced
		// by m.mu but error and drop the update rather than panic.
		m.Logger.Error("failed to deliver ConfigSnapshot to proxy",
			"proxy", snap.ProxyID.String(),
		)
	}
}

// Watch registers a watch on a proxy. It might not exist yet in which case this
// will not fail, but no updates will be delivered until the proxy is
// registered. If there is already a valid snapshot in memory, it will be
// delivered immediately.
func (m *Manager) Watch(proxyID structs.ServiceID) (<-chan *ConfigSnapshot, CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// This buffering is crucial otherwise we'd block immediately trying to
	// deliver the current snapshot below if we already have one.
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
			// We rely on ch being buffered above and that it's not been passed
			// anywhere so we must be the only writer so this will never block and
			// deadlock.
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
func (m *Manager) closeWatchLocked(proxyID structs.ServiceID, watchIdx uint64) {
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

	// Then close all states
	for proxyID, state := range m.proxies {
		state.Close()
		delete(m.proxies, proxyID)
	}
	return nil
}
