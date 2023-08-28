// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalog

import (
	"context"
	"errors"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

const source proxycfg.ProxySource = "catalog"

// ConfigSource wraps a proxycfg.Manager to register services with it, from the
// catalog, when they are requested by the xDS server.
type ConfigSource struct {
	Config

	mu      sync.Mutex
	watches map[proxycfg.ProxyID]*watch

	shutdownCh chan struct{}
}

type watch struct {
	numWatchers int // guarded by ConfigSource.mu.
	closeCh     chan chan struct{}
}

// NewConfigSource creates a ConfigSource with the given configuration.
func NewConfigSource(cfg Config) *ConfigSource {
	return &ConfigSource{
		Config:     cfg,
		watches:    make(map[proxycfg.ProxyID]*watch),
		shutdownCh: make(chan struct{}),
	}
}

// Watch wraps the underlying proxycfg.Manager and dynamically registers
// services from the catalog with it when requested by the xDS server.
func (m *ConfigSource) Watch(id *pbresource.ID, nodeName string, token string) (<-chan proxycfg.ProxySnapshot, limiter.SessionTerminatedChan, proxycfg.CancelFunc, error) {
	// Create service ID
	serviceID := structs.NewServiceID(id.Name, GetEnterpriseMetaFromResourceID(id))
	// If the service is registered to the local agent, use the LocalConfigSource
	// rather than trying to configure it from the catalog.
	if nodeName == m.NodeName && m.LocalState.ServiceExists(serviceID) {
		return m.LocalConfigSource.Watch(id, nodeName, token)
	}

	// Begin a session with the xDS session concurrency limiter.
	//
	// We do this here rather than in the xDS server because we don't want to apply
	// the limit to services from the LocalConfigSource.
	//
	// See: https://github.com/hashicorp/consul/issues/15753
	session, err := m.SessionLimiter.BeginSession()
	if err != nil {
		return nil, nil, nil, err
	}

	proxyID := proxycfg.ProxyID{
		ServiceID: serviceID,
		NodeName:  nodeName,
		Token:     token,
	}

	// Start the watch on the real proxycfg Manager.
	snapCh, cancelWatch := m.Manager.Watch(proxyID)

	// Wrap the cancelWatch function with our bookkeeping. m.mu must be held when calling.
	var cancelOnce sync.Once
	cancel := func() {
		cancelOnce.Do(func() {
			cancelWatch()
			m.cleanup(proxyID)
			session.End()
		})
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.watches[proxyID]
	if ok {
		w.numWatchers++
	} else {
		w = &watch{closeCh: make(chan chan struct{}), numWatchers: 1}
		m.watches[proxyID] = w

		if err := m.startSync(w.closeCh, proxyID); err != nil {
			delete(m.watches, proxyID)
			cancelWatch()
			session.End()
			return nil, nil, nil, err
		}
	}

	return snapCh, session.Terminated(), cancel, nil
}

func (m *ConfigSource) Shutdown() {
	close(m.shutdownCh)
}

// startSync fetches a service from the state store's catalog tables and
// registers it with the proxycfg Manager. It spawns a goroutine to watch
// and re-register the service whenever it changes - this goroutine will
// run until a signal is sent on closeCh (at which point the service will
// be deregistered).
//
// If the first attempt to fetch and register the service fails, startSync
// will return an error (and no goroutine will be started).
func (m *ConfigSource) startSync(closeCh <-chan chan struct{}, proxyID proxycfg.ProxyID) error {
	logger := m.Logger.With(
		"proxy_service_id", proxyID.ServiceID.String(),
		"node", proxyID.NodeName,
	)

	logger.Trace("syncing catalog service")

	fetchAndRegister := func() (memdb.WatchSet, error) {
		store := m.GetStore()
		ws := memdb.NewWatchSet()

		// Add the store's AbandonCh to the WatchSet so that if the store is abandoned
		// during a snapshot restore we'll unblock and re-register the service.
		ws.Add(store.AbandonCh())

		_, ns, err := store.NodeService(ws, proxyID.NodeName, proxyID.ID, &proxyID.EnterpriseMeta, structs.DefaultPeerKeyword)
		switch {
		case err != nil:
			logger.Error("failed to read service from state store", "error", err.Error())
			return nil, err
		case ns == nil:
			m.Manager.Deregister(proxyID, source)
			logger.Trace("service does not exist in catalog, de-registering it with proxycfg manager")
			return ws, nil
		case !ns.Kind.IsProxy():
			err := errors.New("service must be a sidecar proxy or gateway")
			logger.Error(err.Error())
			return nil, err
		}

		_, ns, err = configentry.MergeNodeServiceWithCentralConfig(ws, store, ns, logger)
		if err != nil {
			logger.Error("failed to merge with central config", "error", err.Error())
			return nil, err
		}

		if err = m.Manager.Register(proxyID, ns, source, proxyID.Token, false); err != nil {
			logger.Error("failed to register service", "error", err.Error())
			return nil, err
		}

		return ws, nil
	}

	syncLoop := func(ws memdb.WatchSet) {
		// Cancel the context on return to clean up the goroutine started by WatchCh.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for {
			select {
			case <-ws.WatchCh(ctx):
				// Something changed, unblock and re-run the query.
				//
				// It is expected that all other branches of this select will return and
				// cancel the context given to WatchCh (to clean up its goroutine).
			case doneCh := <-closeCh:
				// All watchers of this service (xDS streams) have gone away, so it's time
				// to free its resources.
				//
				// TODO(agentless): we should probably wait for a short grace period before
				// de-registering the service to allow clients to reconnect after a network
				// blip.
				logger.Trace("de-registering service with proxycfg manager because all watchers have gone away")
				m.Manager.Deregister(proxyID, source)
				close(doneCh)
				return
			case <-m.shutdownCh:
				// Manager is shutting down, stop the goroutine.
				return
			}

			var err error
			ws, err = fetchAndRegister()
			if err != nil {
				return
			}
		}
	}

	ws, err := fetchAndRegister()
	if err != nil {
		// Currently, only the first attempt's error is returned to the xDS server,
		// which terminates the stream immediately.
		//
		// We don't (yet) have a way to surface subsequent errors to the xDS server.
		//
		// We could wrap ConfigSnapshot in a sum type (i.e. a struct that contains
		// either a snapshot or an error) but given the relative unlikelihood of a
		// query that succeeds once failing in the future, it doesn't seem worth it.
		//
		// Instead, we log the error and leave any watchers hanging. Perhaps another
		// solution would be to close any watch channels when de-registering a service?
		return err
	}
	go syncLoop(ws)

	return nil
}

// cleanup decrements the watchers counter for the given proxy, and if it has
// reached zero, stops the sync goroutine (and de-registers the service).
func (m *ConfigSource) cleanup(id proxycfg.ProxyID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h := m.watches[id]
	h.numWatchers--

	if h.numWatchers == 0 {
		// We wait for doneCh to be closed by the sync goroutine, so that the lock is
		// held until after the service is de-registered - this prevents a potential
		// race where another sync goroutine is started for the service and we undo
		// its call to register the service.
		//
		// This cannot deadlock because closeCh is unbuffered. Sending will only
		// succeed if the sync goroutine is ready to receive (which always closes
		// doneCh).
		doneCh := make(chan struct{})
		select {
		case h.closeCh <- doneCh:
			<-doneCh
		case <-m.shutdownCh:
			// ConfigSource is shutting down, so the goroutine will be stopped anyway.
		}

		delete(m.watches, id)
	}
}

type Config struct {
	// NodeName is the name of the local agent node.
	NodeName string

	// Manager is the proxycfg Manager with which proxy services will be registered.
	Manager ConfigManager

	// State is the agent's local state that will be used to check if a proxy is
	// registered locally.
	LocalState *local.State

	// LocalConfigSource is used to configure proxies registered in the agent's
	// local state.
	LocalConfigSource Watcher

	// GetStore is used to access the server's state store.
	GetStore func() Store

	// Logger will be used to write log messages.
	Logger hclog.Logger

	// SessionLimiter is used to enforce xDS concurrency limits.
	SessionLimiter SessionLimiter
}

//go:generate mockery --name ConfigManager --inpackage
type ConfigManager interface {
	Watch(req proxycfg.ProxyID) (<-chan proxycfg.ProxySnapshot, proxycfg.CancelFunc)
	Register(proxyID proxycfg.ProxyID, service *structs.NodeService, source proxycfg.ProxySource, token string, overwrite bool) error
	Deregister(proxyID proxycfg.ProxyID, source proxycfg.ProxySource)
}

type Store interface {
	NodeService(ws memdb.WatchSet, nodeName string, serviceID string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.NodeService, error)
	ReadResolvedServiceConfigEntries(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, upstreamIDs []structs.ServiceID, proxyMode structs.ProxyMode) (uint64, *configentry.ResolvedServiceConfigSet, error)
	AbandonCh() <-chan struct{}
}

//go:generate mockery --name Watcher --inpackage
type Watcher interface {
	Watch(proxyID *pbresource.ID, nodeName string, token string) (<-chan proxycfg.ProxySnapshot, limiter.SessionTerminatedChan, proxycfg.CancelFunc, error)
}

//go:generate mockery --name SessionLimiter --inpackage
type SessionLimiter interface {
	BeginSession() (limiter.Session, error)
}
