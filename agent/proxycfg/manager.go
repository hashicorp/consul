package proxycfg

import (
	"errors"
	"sync"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
)

// ProxyID is a handle on a proxy service instance being tracked by Manager.
type ProxyID struct {
	structs.ServiceID

	// NodeName identifies the node to which the proxy is registered.
	NodeName string

	// Token is used to track watches on the same proxy with different ACL tokens
	// separately, to prevent accidental security bugs.
	//
	// Note: this can be different to the ACL token used for authorization that is
	// passed to Register (e.g. agent-local services are registered ahead-of-time
	// with a token that may be different to the one presented in the xDS stream).
	Token string
}

// ProxySource identifies where a proxy service tracked by Manager came from,
// such as the agent's local state or the catalog. It's used to prevent sources
// from overwriting each other's registrations.
type ProxySource string

// CancelFunc is a type for a returned function that can be called to cancel a
// watch.
type CancelFunc func()

// Manager provides an API with which proxy services can be registered, and
// coordinates the fetching (and refreshing) of intentions, upstreams, discovery
// chain, certificates etc.
//
// Consumers such as the xDS server can then subscribe to receive snapshots of
// this data whenever it changes.
//
// See package docs for more detail.
type Manager struct {
	ManagerConfig

	mu         sync.Mutex
	proxies    map[ProxyID]*state
	watchers   map[ProxyID]map[uint64]chan *ConfigSnapshot
	maxWatchID uint64
}

// ManagerConfig holds the required external dependencies for a Manager
// instance. All fields must be set to something valid or the manager will
// panic. The ManagerConfig is passed by value to NewManager so the passed value
// can be mutated safely.
type ManagerConfig struct {
	// DataSources contains the dependencies used to consume data used to configure
	// proxies.
	DataSources DataSources
	// source describes the current agent's identity, it's used directly for
	// prepared query discovery but also indirectly as a way to pass current
	// Datacenter name into other request types that need it. This is sufficient
	// for now and cleaner than passing the entire RuntimeConfig.
	Source *structs.QuerySource
	// DNSConfig is the agent's relevant DNS config for any proxies.
	DNSConfig DNSConfig
	// logger is the agent's logger to be used for logging logs.
	Logger          hclog.Logger
	TLSConfigurator *tlsutil.Configurator

	// IntentionDefaultAllow is set by the agent so that we can pass this
	// information to proxies that need to make intention decisions on their
	// own.
	IntentionDefaultAllow bool

	PeeringEnabled bool
}

// NewManager constructs a Manager.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.Source == nil || cfg.Logger == nil {
		return nil, errors.New("all ManagerConfig fields must be provided")
	}
	m := &Manager{
		ManagerConfig: cfg,
		proxies:       make(map[ProxyID]*state),
		watchers:      make(map[ProxyID]map[uint64]chan *ConfigSnapshot),
	}
	return m, nil
}

// RegisteredProxies returns a list of the proxies tracked by Manager, filtered
// by source.
func (m *Manager) RegisteredProxies(source ProxySource) []ProxyID {
	m.mu.Lock()
	defer m.mu.Unlock()

	proxies := make([]ProxyID, 0, len(m.proxies))
	for id, state := range m.proxies {
		if state.source != source {
			continue
		}
		proxies = append(proxies, id)
	}
	return proxies
}

// Register and start fetching resources for the given proxy service. If the
// given service was already registered by a different source (e.g. we began
// tracking it from the catalog, but then it was registered to the server
// agent locally) the service will be left as-is unless overwrite is true.
func (m *Manager) Register(id ProxyID, ns *structs.NodeService, source ProxySource, token string, overwrite bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.proxies[id]
	if ok {
		if state.source != source && !overwrite {
			// Registered by a different source, leave as-is.
			return nil
		}

		if !state.Changed(ns, token) {
			// No change
			return nil
		}

		// We are updating the proxy, close its old state
		state.Close()
	}

	// TODO: move to a function that translates ManagerConfig->stateConfig
	stateConfig := stateConfig{
		logger:                m.Logger.With("service_id", id.String()),
		dataSources:           m.DataSources,
		source:                m.Source,
		dnsConfig:             m.DNSConfig,
		intentionDefaultAllow: m.IntentionDefaultAllow,
		peeringEnabled:        m.PeeringEnabled,
	}
	if m.TLSConfigurator != nil {
		stateConfig.serverSNIFn = m.TLSConfigurator.ServerSNI
	}

	var err error
	state, err = newState(id, ns, source, token, stateConfig)
	if err != nil {
		return err
	}

	ch, err := state.Watch()
	if err != nil {
		return err
	}
	m.proxies[id] = state

	// Start a goroutine that will wait for changes and broadcast them to watchers.
	go m.notifyBroadcast(ch)
	return nil
}

// Deregister the given proxy service, but only if it was registered by the same
// source.
func (m *Manager) Deregister(id ProxyID, source ProxySource) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.proxies[id]
	if !ok {
		return
	}

	if state.source != source {
		return
	}

	// Closing state will let the goroutine we started in Register finish since
	// watch chan is closed.
	state.Close()
	delete(m.proxies, id)

	// We intentionally leave potential watchers hanging here - there is no new
	// config for them and closing their channels might be indistinguishable from
	// an error that they should retry. We rely for them to eventually give up
	// (because they are in fact not running any more) and so the watches be
	// cleaned up naturally.
}

func (m *Manager) notifyBroadcast(ch <-chan ConfigSnapshot) {
	// Run until ch is closed
	for snap := range ch {
		m.notify(&snap)
	}
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
func (m *Manager) Watch(id ProxyID) (<-chan *ConfigSnapshot, CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// This buffering is crucial otherwise we'd block immediately trying to
	// deliver the current snapshot below if we already have one.
	ch := make(chan *ConfigSnapshot, 1)
	watchers, ok := m.watchers[id]
	if !ok {
		watchers = make(map[uint64]chan *ConfigSnapshot)
	}
	watchID := m.maxWatchID
	m.maxWatchID++
	watchers[watchID] = ch
	m.watchers[id] = watchers

	// Deliver the current snapshot immediately if there is one ready
	if state, ok := m.proxies[id]; ok {
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
		m.closeWatchLocked(id, watchID)
	}
}

// closeWatchLocked cleans up state related to a single watcher. It assumes the
// lock is held.
func (m *Manager) closeWatchLocked(proxyID ProxyID, watchID uint64) {
	if watchers, ok := m.watchers[proxyID]; ok {
		if ch, ok := watchers[watchID]; ok {
			delete(watchers, watchID)
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

	// Close all current watchers first
	for proxyID, watchers := range m.watchers {
		for watchID := range watchers {
			m.closeWatchLocked(proxyID, watchID)
		}
	}

	// Then close all states
	for proxyID, state := range m.proxies {
		state.Close()
		delete(m.proxies, proxyID)
	}
	return nil
}
