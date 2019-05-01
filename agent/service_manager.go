package agent

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/copystructure"
	"golang.org/x/net/context"
)

// The ServiceManager is a layer for service registration in between the agent
// and the local state. Any services must be registered with the ServiceManager,
// which then maintains a long-running watch of any globally-set service or proxy
// configuration that applies to the service in order to register the final, merged
// service configuration locally in the agent state.
type ServiceManager struct {
	services map[string]*serviceConfigWatch
	agent    *Agent

	lock sync.Mutex
}

func NewServiceManager(agent *Agent) *ServiceManager {
	return &ServiceManager{
		services: make(map[string]*serviceConfigWatch),
		agent:    agent,
	}
}

// AddService starts a new serviceConfigWatch if the service has not been registered, and
// updates the existing registration if it has. For a new service, a call will also be made
// to fetch the merged global defaults that apply to the service in order to compose the
// initial registration.
func (s *ServiceManager) AddService(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, source configSource) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// TODO(banks): this is jank - it fixes the issue for now and makes the
	// integration test work but we need to support registration-configured
	// sidecar registrations too. It's a bit gross because if they manually
	// register a sidecar AND the central config enables one, we could end up
	// doing two lots of watches and have the ServiceManager duplicating work of
	// updating the sidecar's registration on config changes. That might be OK
	// though since the cache will de-dupe the RPC watch in that case anyway.
	if service.Kind == "connect-proxy" {
		return nil
	}

	reg := serviceRegistration{
		service:  service,
		chkTypes: chkTypes,
		persist:  persist,
		token:    token,
		source:   source,
	}

	// If a service watch already exists, update the registration. Otherwise,
	// start a new config watcher.
	watch, ok := s.services[service.ID]
	if ok {
		if err := watch.updateRegistration(&reg); err != nil {
			return err
		}
		s.agent.logger.Printf("[DEBUG] agent.manager: updated local registration for service %q", service.ID)
	} else {
		// This is a new entry, so get the existing global config and do the initial
		// registration with the merged config.
		watch := &serviceConfigWatch{
			registration: &reg,
			readyCh:      make(chan error),
			updateCh:     make(chan cache.UpdateEvent, 1),
			agent:        s.agent,
		}

		// Start the config watch, which starts a blocking query for the resolved service config
		// in the background.
		if err := watch.Start(); err != nil {
			return err
		}

		// Call ReadyWait to block until the cache has returned the initial config and the service
		// has been registered.
		if err := watch.ReadyWait(); err != nil {
			watch.Stop()
			return err
		}

		s.services[service.ID] = watch

		s.agent.logger.Printf("[DEBUG] agent.manager: added local registration for service %q", service.ID)
	}

	return nil
}

func (s *ServiceManager) RemoveService(serviceID string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	serviceWatch, ok := s.services[serviceID]
	if !ok {
		return
	}

	serviceWatch.Stop()
	delete(s.services, serviceID)
}

// serviceRegistration represents a locally registered service.
type serviceRegistration struct {
	service  *structs.NodeService
	chkTypes []*structs.CheckType
	persist  bool
	token    string
	source   configSource
}

// serviceConfigWatch is a long running helper for composing the end config
// for a given service from both the local registration and the global
// service/proxy defaults.
type serviceConfigWatch struct {
	registration *serviceRegistration
	defaults     *structs.ServiceDefinition

	agent *Agent

	// readyCh is used for ReadyWait in order to block until the first update
	// for the resolved service config is received from the cache.
	readyCh chan error

	// ctx and cancelFunc store the overall context that lives as long as the
	// Watch instance is needed, possibly spanning multiple cache.Notify
	// lifetimes.
	ctx        context.Context
	cancelFunc func()

	// cacheKey stores the key of the current request, when registration changes
	// we check to see if a new cache watch is needed.
	cacheKey string

	// updateCh receives changes from cache watchers or registration changes.
	updateCh chan cache.UpdateEvent

	// notifyCancel, if non-nil it the cancel func that will stop the currently
	// active Notify loop. It does not cancel ctx and is used when we need to
	// switch to a new Notify call because cache key changed.
	notifyCancel func()

	lock sync.Mutex
}

// Start starts the config watch and a goroutine to handle updates over
// the updateCh. This is not safe to call more than once.
func (s *serviceConfigWatch) Start() error {
	s.ctx, s.cancelFunc = context.WithCancel(context.Background())
	if err := s.ensureConfigWatch(); err != nil {
		return err
	}
	go s.runWatch()

	return nil
}

func (s *serviceConfigWatch) Stop() {
	s.cancelFunc()
}

// ReadyWait blocks until the readyCh is closed, which means the initial
// registration of the service has been completed. If there was an error
// with the initial registration, it will be returned.
func (s *serviceConfigWatch) ReadyWait() error {
	err := <-s.readyCh
	return err
}

// runWatch handles any update events from the cache.Notify until the
// config watch is shut down.
func (s *serviceConfigWatch) runWatch() {
	firstRun := true
	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.updateCh:
			if err := s.handleUpdate(event, false, firstRun); err != nil {
				s.agent.logger.Printf("[ERR] agent.manager: error handling service update: %v", err)
			}
			firstRun = false
		}
	}
}

// handleUpdate receives an update event about either the service registration or the
// global config defaults, updates the local state and re-registers the service with
// the newly merged config. This function takes the serviceConfigWatch lock to ensure
// only one update can be happening at a time.
func (s *serviceConfigWatch) handleUpdate(event cache.UpdateEvent, locked, firstRun bool) error {
	// Take the agent state lock if needed. This is done before the local config watch
	// lock in order to prevent a race between this config watch and others - the config
	// watch lock is the inner lock and the agent stateLock is the outer lock. If this is the
	// first run we also don't need to take the stateLock, as this is being waited on
	// synchronously by a caller that already holds it.
	if !locked && !firstRun {
		s.agent.stateLock.Lock()
		defer s.agent.stateLock.Unlock()
	}
	s.lock.Lock()
	defer s.lock.Unlock()

	// If we got an error, log a warning if this is the first update; otherwise return the error.
	// We want the initial update to cause a service registration no matter what.
	if event.Err != nil {
		if firstRun {
			s.agent.logger.Printf("[WARN] could not retrieve initial service_defaults config for service %q: %v",
				s.registration.service.ID, event.Err)
		} else {
			return fmt.Errorf("error watching service config: %v", event.Err)
		}
	} else {
		switch res := event.Result.(type) {
		case *serviceRegistration:
			s.registration = res
			// We may need to restart watch if upstreams changed
			if err := s.ensureConfigWatch(); err != nil {
				return err
			}
		case *structs.ServiceConfigResponse:
			// Sanity check this even came from the currently active watch to ignore
			// rare races when switching cache keys
			if event.CorrelationID != s.cacheKey {
				// It's a no-op. The new watcher will deliver (or may have already
				// delivered) the correct config so just ignore this old message.
				return nil
			}
			s.defaults = &res.Definition
		default:
			return fmt.Errorf("unknown update event type: %T", event)
		}
	}

	// Merge the local registration with the central defaults and update this service
	// in the local state.
	service, err := s.mergeServiceConfig()
	if err != nil {
		return err
	}
	if err := s.updateAgentRegistration(service); err != nil {
		// If this is the initial registration, return the error through the readyCh
		// so it can be passed back to the original caller.
		if firstRun {
			s.readyCh <- err
		}
		return fmt.Errorf("error updating service registration: %v", err)
	}

	// If this is the first registration, set the ready status by closing the channel.
	if firstRun {
		close(s.readyCh)
	}

	return nil
}

// updateAgentRegistration updates the service (and its sidecar, if applicable) in the
// local state.
func (s *serviceConfigWatch) updateAgentRegistration(ns *structs.NodeService) error {
	// Grab and validate sidecar if there is one too
	sidecar, sidecarChecks, sidecarToken, err := s.agent.sidecarServiceFromNodeService(ns, s.registration.token)
	if err != nil {
		return fmt.Errorf("Failed to validate sidecar for service %q: %v", ns.Service, err)
	}

	// Remove sidecar from NodeService now it's done it's job it's just a config
	// syntax sugar and shouldn't be persisted in local or server state.
	ns.Connect.SidecarService = nil

	if err := s.agent.addServiceInternal(ns, s.registration.chkTypes, s.registration.persist, s.registration.token, s.registration.source); err != nil {
		return fmt.Errorf("Failed to register service %q: %v", ns.Service, err)
	}

	// If there is a sidecar service, register that too.
	if sidecar != nil {
		if err := s.agent.addServiceInternal(sidecar, sidecarChecks, s.registration.persist, sidecarToken, s.registration.source); err != nil {
			return fmt.Errorf("Failed to register sidecar for service %q: %v", ns.Service, err)
		}
	}

	return nil
}

// ensureConfigWatch starts a cache.Notify goroutine to run a continuous
// blocking query on the resolved service config for this service. If the
// registration has changed in a way that requires a new blocking query, it will
// cancel any current watch and start a new one. It is a no-op if there is an
// existing watch that is sufficient for the current registration. It is not
// thread-safe and must only be called from the Start method (which is only safe
// to call once as documented) or from inside the run loop.
func (s *serviceConfigWatch) ensureConfigWatch() error {
	name := s.registration.service.Service

	var upstreams []string

	if ss := s.registration.service.Connect.SidecarService; ss != nil {
		if ss.Proxy != nil {
			for _, us := range ss.Proxy.Upstreams {
				if us.DestinationType == "" || us.DestinationType == structs.UpstreamDestTypeService {
					upstreams = append(upstreams, us.DestinationName)
				}
			}
		}
	}

	req := &structs.ServiceConfigRequest{
		Name:         name,
		Datacenter:   s.agent.config.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.agent.config.ACLAgentToken},
		Upstreams:    upstreams,
	}
	if s.registration.token != "" {
		req.QueryOptions.Token = s.registration.token
	}

	// See if this request is different from the current one
	cacheKey := req.CacheInfo().Key
	if cacheKey == s.cacheKey {
		return nil
	}

	// If there is an existing notify running, stop it first. This may leave a
	// blocking query running in the background but the Notify loop will swallow
	// the response and exit when it next unblocks so we can consider it stopped.
	if s.notifyCancel != nil {
		s.notifyCancel()
	}

	// Make a new context just for this Notify call
	ctx, cancel := context.WithCancel(s.ctx)
	s.notifyCancel = cancel
	s.cacheKey = cacheKey
	// We use the cache key as the correlationID here. Notify in general will not
	// respond on the updateCh after the context is cancelled however there could
	// possible be a race where it has only just got an update and checked the
	// context before we cancel and so might still deliver the old event. Using
	// the cacheKey allows us to ignore updates from the old cache watch and makes
	// even this rare edge case safe.
	err := s.agent.cache.Notify(ctx, cachetype.ResolvedServiceConfigName, req,
		s.cacheKey, s.updateCh)

	return err
}

// updateRegistration does a synchronous update of the local service registration and
// returns the result. The agent stateLock should be held when calling this function.
func (s *serviceConfigWatch) updateRegistration(registration *serviceRegistration) error {
	return s.handleUpdate(cache.UpdateEvent{
		Result: registration,
	}, true, false)
}

// mergeServiceConfig returns the final effective config for the watched service,
// including the latest known global defaults from the servers.
func (s *serviceConfigWatch) mergeServiceConfig() (*structs.NodeService, error) {
	if s.defaults == nil {
		return s.registration.service, nil
	}

	// We don't want to change s.registration in place since it is our source of
	// truth about what was actually registered before defaults. So copy it first.
	nsRaw, err := copystructure.Copy(s.registration.service)
	if err != nil {
		return nil, err
	}
	ns := nsRaw.(*structs.NodeService)
	err = ns.MergeDefaults(s.defaults.NodeService())
	return ns, err
}
