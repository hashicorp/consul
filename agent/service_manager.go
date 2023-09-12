package agent

import (
	"fmt"
	"sync"

	"golang.org/x/net/context"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
)

// ServiceManager watches changes to central service config for all services
// registered with it. When a central config changes, the local service will
// be updated with the correct values from the central config.
type ServiceManager struct {
	agent *Agent

	// servicesLock guards the services map, but not the watches contained
	// therein
	servicesLock sync.Mutex

	// services tracks all active watches for registered services
	services map[structs.ServiceID]*serviceConfigWatch

	// ctx is the shared context for all goroutines launched
	ctx context.Context

	// cancel can be used to stop all goroutines launched
	cancel context.CancelFunc

	// running keeps track of live goroutines (worker and watcher)
	running sync.WaitGroup
}

func NewServiceManager(agent *Agent) *ServiceManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ServiceManager{
		agent:    agent,
		services: make(map[structs.ServiceID]*serviceConfigWatch),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Stop forces all background goroutines to terminate and blocks until they complete.
//
// NOTE: the caller must NOT hold the Agent.stateLock!
func (s *ServiceManager) Stop() {
	s.cancel()
	s.running.Wait()
}

// AddService will (re)create a serviceConfigWatch on the given service. For
// each call of this function the first registration will happen inline and
// will read the merged global defaults for the service through the agent cache
// (regardless of whether or not the service was already registered).  This
// lets validation or authorization related errors bubble back up to the
// caller's RPC inline with their request. Upon success a goroutine will keep
// this updated in the background.
//
// If waitForCentralConfig=true is used, the initial registration blocks on
// fetching the merged global config through the cache. If false, no such RPC
// occurs and only the previousDefaults are used.
//
// persistServiceConfig controls if the INITIAL registration will result in
// persisting the service config to disk again. All background updates will
// always persist.
//
// service, chkTypes, persist, token, replaceExistingChecks, and source are
// basically pass-through arguments to Agent.addServiceInternal that follow the
// semantics there. The one key difference is that the service provided will be
// merged with the global defaults before registration.
//
// NOTE: the caller must hold the Agent.stateLock!
func (s *ServiceManager) AddService(req addServiceLockedRequest) error {
	s.servicesLock.Lock()
	defer s.servicesLock.Unlock()

	sid := req.Service.CompoundServiceID()

	// If a service watch already exists, shut it down and replace it.
	oldWatch, updating := s.services[sid]
	if updating {
		oldWatch.Stop()
		delete(s.services, sid)
	}

	// Get the existing global config and do the initial registration with the
	// merged config.
	watch := &serviceConfigWatch{registration: req, agent: s.agent}
	if err := watch.register(s.ctx); err != nil {
		return err
	}
	if err := watch.start(s.ctx, &s.running); err != nil {
		return err
	}

	s.services[sid] = watch

	if updating {
		s.agent.logger.Debug("updated local registration for service", "service", req.Service.ID)
	} else {
		s.agent.logger.Debug("added local registration for service", "service", req.Service.ID)
	}

	return nil
}

// NOTE: the caller must hold the Agent.stateLock!
func (s *ServiceManager) RemoveService(serviceID structs.ServiceID) {
	s.servicesLock.Lock()
	defer s.servicesLock.Unlock()

	if oldWatch, exists := s.services[serviceID]; exists {
		oldWatch.Stop()
		delete(s.services, serviceID)
	}
}

// serviceConfigWatch is a long running helper for composing the end config
// for a given service from both the local registration and the global
// service/proxy defaults.
type serviceConfigWatch struct {
	registration addServiceLockedRequest
	agent        *Agent

	// cacheKey stores the key of the current request, when registration changes
	// we check to see if a new cache watch is needed.
	cacheKey string

	cancelFunc func()
	running    sync.WaitGroup
}

// NOTE: this is called while holding the Agent.stateLock
func (w *serviceConfigWatch) register(ctx context.Context) error {
	serviceDefaults, err := w.registration.serviceDefaults(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve initial service_defaults config for service %q: %v",
			w.registration.Service.ID, err)
	}

	// Merge the local registration with the central defaults and update this service
	// in the local state.
	merged, err := configentry.MergeServiceConfig(serviceDefaults, w.registration.Service)
	if err != nil {
		return err
	}

	// make a copy of the AddServiceRequest
	req := w.registration
	req.Service = merged

	err = w.agent.addServiceInternal(addServiceInternalRequest{
		addServiceLockedRequest: req,
		persistService:          w.registration.Service,
		persistServiceDefaults:  serviceDefaults,
	})
	if err != nil {
		return fmt.Errorf("error updating service registration: %v", err)
	}
	return nil
}

func serviceDefaultsFromStruct(v *structs.ServiceConfigResponse) func(context.Context) (*structs.ServiceConfigResponse, error) {
	return func(_ context.Context) (*structs.ServiceConfigResponse, error) {
		return v, nil
	}
}

func serviceDefaultsFromCache(bd BaseDeps, req AddServiceRequest) func(context.Context) (*structs.ServiceConfigResponse, error) {
	// NOTE: this is called while holding the Agent.stateLock
	return func(ctx context.Context) (*structs.ServiceConfigResponse, error) {
		req := makeConfigRequest(bd, req)

		raw, _, err := bd.Cache.Get(ctx, cachetype.ResolvedServiceConfigName, req)
		if err != nil {
			return nil, err
		}

		serviceConfig, ok := raw.(*structs.ServiceConfigResponse)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		return serviceConfig, nil
	}
}

// Start starts the config watch and a goroutine to handle updates over the
// updateCh. This is safe to call more than once assuming you have called Stop
// after each Start.
//
// NOTE: this is called while holding the Agent.stateLock
func (w *serviceConfigWatch) start(ctx context.Context, wg *sync.WaitGroup) error {
	ctx, w.cancelFunc = context.WithCancel(ctx)

	// Configure and start a cache.Notify goroutine to run a continuous
	// blocking query on the resolved service config for this service.
	req := makeConfigRequest(w.agent.baseDeps, w.registration.AddServiceRequest)
	w.cacheKey = req.CacheInfo().Key

	updateCh := make(chan cache.UpdateEvent, 1)

	// We use the cache key as the correlationID here. Notify in general will not
	// respond on the updateCh after the context is cancelled however there could
	// possible be a race where it has only just got an update and checked the
	// context before we cancel and so might still deliver the old event. Using
	// the cacheKey allows us to ignore updates from the old cache watch and makes
	// even this rare edge case safe.
	err := w.agent.cache.Notify(ctx, cachetype.ResolvedServiceConfigName, req, w.cacheKey, updateCh)
	if err != nil {
		w.cancelFunc()
		return err
	}

	w.running.Add(1)
	wg.Add(1)
	go w.runWatch(ctx, wg, updateCh)

	return nil
}

func (w *serviceConfigWatch) Stop() {
	w.cancelFunc()
	w.running.Wait()
}

// runWatch handles any update events from the cache.Notify until the
// config watch is shut down.
//
// NOTE: the caller must NOT hold the Agent.stateLock!
func (w *serviceConfigWatch) runWatch(ctx context.Context, wg *sync.WaitGroup, updateCh chan cache.UpdateEvent) {
	defer wg.Done()
	defer w.running.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-updateCh:
			if err := w.handleUpdate(ctx, event); err != nil {
				w.agent.logger.Error("error handling service update", "error", err)
			}
		}
	}
}

// handleUpdate receives an update event from the global config defaults, updates
// the local state and re-registers the service with the newly merged config.
//
// NOTE: the caller must NOT hold the Agent.stateLock!
func (w *serviceConfigWatch) handleUpdate(ctx context.Context, event cache.UpdateEvent) error {
	// If we got an error, log a warning if this is the first update; otherwise return the error.
	// We want the initial update to cause a service registration no matter what.
	if event.Err != nil {
		return fmt.Errorf("error watching service config: %v", event.Err)
	}

	serviceDefaults, ok := event.Result.(*structs.ServiceConfigResponse)
	if !ok {
		return fmt.Errorf("unknown update event type: %T", event)
	}

	// Sanity check this even came from the currently active watch to ignore
	// rare races when switching cache keys
	if event.CorrelationID != w.cacheKey {
		// It's a no-op. The new watcher will deliver (or may have already
		// delivered) the correct config so just ignore this old message.
		return nil
	}

	// Merge the local registration with the central defaults and update this service
	// in the local state.
	merged, err := configentry.MergeServiceConfig(serviceDefaults, w.registration.Service)
	if err != nil {
		return err
	}

	// make a copy of the AddServiceRequest
	req := w.registration
	req.Service = merged
	req.persistServiceConfig = true

	args := addServiceInternalRequest{
		addServiceLockedRequest: req,
		persistService:          w.registration.Service,
		persistServiceDefaults:  serviceDefaults,
	}

	if err := w.agent.stateLock.TryLock(ctx); err != nil {
		return nil
	}
	defer w.agent.stateLock.Unlock()

	// The context may have been cancelled after the lock was acquired.
	if err := ctx.Err(); err != nil {
		return nil
	}

	if err := w.agent.addServiceInternal(args); err != nil {
		return fmt.Errorf("error updating service registration: %v", err)
	}
	return nil
}

func makeConfigRequest(bd BaseDeps, addReq AddServiceRequest) *structs.ServiceConfigRequest {
	var (
		ns   = addReq.Service
		name = ns.Service
	)

	var upstreams []structs.ServiceID

	// Note that only sidecar proxies should even make it here for now although
	// later that will change to add the condition.
	if ns.IsSidecarProxy() {
		// This is a sidecar proxy, ignore the proxy service's config since we are
		// managed by the target service config.
		name = ns.Proxy.DestinationServiceName

		// Also if we have any upstreams defined, add them to the request so we can
		// learn about their configs.
		for _, us := range ns.Proxy.Upstreams {
			if us.DestinationType == "" || us.DestinationType == structs.UpstreamDestTypeService {
				sid := us.DestinationID()
				sid.EnterpriseMeta.Merge(&ns.EnterpriseMeta)
				upstreams = append(upstreams, sid)
			}
		}
	}

	req := &structs.ServiceConfigRequest{
		Name:           name,
		Datacenter:     bd.RuntimeConfig.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: addReq.token},
		MeshGateway:    ns.Proxy.MeshGateway,
		Mode:           ns.Proxy.Mode,
		UpstreamIDs:    upstreams,
		EnterpriseMeta: ns.EnterpriseMeta,
	}
	if req.QueryOptions.Token == "" {
		req.QueryOptions.Token = bd.Tokens.AgentToken()
	}
	return req
}
