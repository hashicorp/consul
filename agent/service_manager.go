package agent

import (
	"fmt"
	"sync"

	"github.com/imdario/mergo"
	"github.com/mitchellh/copystructure"
	"golang.org/x/net/context"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
)

// The ServiceManager is a layer for service registration in between the agent
// and the local state. Any services must be registered with the ServiceManager,
// which then maintains a long-running watch of any globally-set service or proxy
// configuration that applies to the service in order to register the final, merged
// service configuration locally in the agent state.
type ServiceManager struct {
	agent *Agent

	// servicesLock guards the services map, but not the watches contained
	// therein
	servicesLock sync.Mutex

	// services tracks all active watches for registered services
	services map[structs.ServiceID]*serviceConfigWatch

	// registerCh is a channel for receiving service registration requests from
	// from serviceConfigWatchers.
	// The registrations are handled in the background when watches are notified of
	// changes. All sends and receives must also obey the ctx.Done() channel to
	// avoid a deadlock during shutdown.
	registerCh chan *asyncRegisterRequest

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
		agent:      agent,
		services:   make(map[structs.ServiceID]*serviceConfigWatch),
		registerCh: make(chan *asyncRegisterRequest), // must be unbuffered
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Stop forces all background goroutines to terminate and blocks until they complete.
//
// NOTE: the caller must NOT hold the Agent.stateLock!
func (s *ServiceManager) Stop() {
	s.cancel()
	s.running.Wait()
}

// Start starts a background worker goroutine that writes back into the Agent
// state. This only exists to keep the need to lock the agent state lock out of
// the main AddService/RemoveService codepaths to avoid deadlocks.
func (s *ServiceManager) Start() {
	s.running.Add(1)

	go func() {
		defer s.running.Done()
		for {
			select {
			case <-s.ctx.Done():
				return
			case req := <-s.registerCh:
				req.Reply <- s.registerOnce(req.Args)
			}
		}
	}()
}

// runOnce will process a single registration request
func (s *ServiceManager) registerOnce(args addServiceInternalRequest) error {
	s.agent.stateLock.Lock()
	defer s.agent.stateLock.Unlock()

	if args.snap == nil {
		args.snap = s.agent.snapshotCheckState()
	}

	err := s.agent.addServiceInternal(args)
	if err != nil {
		return fmt.Errorf("error updating service registration: %v", err)
	}
	return nil
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
func (s *ServiceManager) AddService(req AddServiceRequest) error {
	req.Service.EnterpriseMeta.Normalize()

	// For now only proxies have anything that can be configured
	// centrally. So bypass the whole manager for regular services.
	if !req.Service.IsSidecarProxy() && !req.Service.IsGateway() {
		req.persistServiceConfig = false
		return s.agent.addServiceInternal(addServiceInternalRequest{AddServiceRequest: req})
	}

	// TODO: replace serviceRegistration with AddServiceRequest
	reg := &serviceRegistration{
		service:               req.Service,
		chkTypes:              req.chkTypes,
		persist:               req.persist,
		token:                 req.token,
		replaceExistingChecks: req.replaceExistingChecks,
		source:                req.Source,
	}

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
	watch := &serviceConfigWatch{
		registration: reg,
		agent:        s.agent,
		registerCh:   s.registerCh,
	}

	err := watch.RegisterAndStart(
		s.ctx,
		req.previousDefaults,
		req.waitForCentralConfig,
		req.persistServiceConfig,
		&s.running,
	)
	if err != nil {
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

// serviceRegistration represents a locally registered service.
type serviceRegistration struct {
	service               *structs.NodeService
	chkTypes              []*structs.CheckType
	persist               bool
	token                 string
	replaceExistingChecks bool
	source                configSource
}

// serviceConfigWatch is a long running helper for composing the end config
// for a given service from both the local registration and the global
// service/proxy defaults.
type serviceConfigWatch struct {
	registration *serviceRegistration

	agent      *Agent
	registerCh chan<- *asyncRegisterRequest

	// cacheKey stores the key of the current request, when registration changes
	// we check to see if a new cache watch is needed.
	cacheKey string

	cancelFunc func()
	running    sync.WaitGroup
}

// NOTE: this is called while holding the Agent.stateLock
func (w *serviceConfigWatch) RegisterAndStart(
	ctx context.Context,
	serviceDefaults *structs.ServiceConfigResponse,
	waitForCentralConfig bool,
	persistServiceConfig bool,
	wg *sync.WaitGroup,
) error {
	// Either we explicitly block waiting for defaults before registering,
	// or we feed it some seed data (or NO data) and bypass the blocking
	// operation. Either way the watcher will end up with something flagged
	// as defaults even if they don't actually reflect actual defaults.
	if waitForCentralConfig {
		var err error
		serviceDefaults, err = w.fetchDefaults(ctx)
		if err != nil {
			return fmt.Errorf("could not retrieve initial service_defaults config for service %q: %v",
				w.registration.service.ID, err)
		}
	}

	// Merge the local registration with the central defaults and update this service
	// in the local state.
	merged, err := mergeServiceConfig(serviceDefaults, w.registration.service)
	if err != nil {
		return err
	}

	// The first time we do this interactively, we need to know if it
	// failed for validation reasons which we only get back from the
	// initial underlying add service call.
	err = w.agent.addServiceInternal(addServiceInternalRequest{
		AddServiceRequest: AddServiceRequest{
			Service:               merged,
			chkTypes:              w.registration.chkTypes,
			persist:               w.registration.persist,
			persistServiceConfig:  persistServiceConfig,
			token:                 w.registration.token,
			replaceExistingChecks: w.registration.replaceExistingChecks,
			Source:                w.registration.source,
			snap:                  w.agent.snapshotCheckState(),
		},
		persistService:  w.registration.service,
		persistDefaults: serviceDefaults,
	})
	if err != nil {
		return fmt.Errorf("error updating service registration: %v", err)
	}

	// Start the config watch, which starts a blocking query for the
	// resolved service config in the background.
	return w.start(ctx, wg)
}

// NOTE: this is called while holding the Agent.stateLock
func (w *serviceConfigWatch) fetchDefaults(ctx context.Context) (*structs.ServiceConfigResponse, error) {
	req := makeConfigRequest(w.agent, w.registration)

	raw, _, err := w.agent.cache.Get(ctx, cachetype.ResolvedServiceConfigName, req)
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

// Start starts the config watch and a goroutine to handle updates over the
// updateCh. This is safe to call more than once assuming you have called Stop
// after each Start.
//
// NOTE: this is called while holding the Agent.stateLock
func (w *serviceConfigWatch) start(ctx context.Context, wg *sync.WaitGroup) error {
	ctx, w.cancelFunc = context.WithCancel(ctx)

	// Configure and start a cache.Notify goroutine to run a continuous
	// blocking query on the resolved service config for this service.
	req := makeConfigRequest(w.agent, w.registration)
	w.cacheKey = req.CacheInfo().Key

	updateCh := make(chan cache.UpdateEvent, 1)

	// We use the cache key as the correlationID here. Notify in general will not
	// respond on the updateCh after the context is cancelled however there could
	// possible be a race where it has only just got an update and checked the
	// context before we cancel and so might still deliver the old event. Using
	// the cacheKey allows us to ignore updates from the old cache watch and makes
	// even this rare edge case safe.
	err := w.agent.cache.Notify(
		ctx,
		cachetype.ResolvedServiceConfigName,
		req,
		w.cacheKey,
		updateCh,
	)
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

// handleUpdate receives an update event the global config defaults, updates
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
	merged, err := mergeServiceConfig(serviceDefaults, w.registration.service)
	if err != nil {
		return err
	}

	// While we were waiting on the agent state lock we may have been shutdown.
	// So avoid doing a registration in that case.
	if err := ctx.Err(); err != nil {
		return nil
	}

	registerReq := &asyncRegisterRequest{
		Args: addServiceInternalRequest{
			AddServiceRequest: AddServiceRequest{
				Service:               merged,
				chkTypes:              w.registration.chkTypes,
				persist:               w.registration.persist,
				persistServiceConfig:  true,
				token:                 w.registration.token,
				replaceExistingChecks: w.registration.replaceExistingChecks,
				Source:                w.registration.source,
			},
			persistService:  w.registration.service,
			persistDefaults: serviceDefaults,
		},
		Reply: make(chan error, 1),
	}

	select {
	case <-ctx.Done():
		return nil
	case w.registerCh <- registerReq:
	}

	select {
	case <-ctx.Done():
		return nil

	case err := <-registerReq.Reply:
		if err != nil {
			return fmt.Errorf("error updating service registration: %v", err)
		}
		return nil
	}
}

type asyncRegisterRequest struct {
	Args  addServiceInternalRequest
	Reply chan error
}

func makeConfigRequest(agent *Agent, registration *serviceRegistration) *structs.ServiceConfigRequest {
	ns := registration.service
	name := ns.Service
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
		Datacenter:     agent.config.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: agent.tokens.AgentToken()},
		UpstreamIDs:    upstreams,
		EnterpriseMeta: ns.EnterpriseMeta,
	}
	if registration.token != "" {
		req.QueryOptions.Token = registration.token
	}
	return req
}

// mergeServiceConfig from service into defaults to produce the final effective
// config for the watched service.
func mergeServiceConfig(defaults *structs.ServiceConfigResponse, service *structs.NodeService) (*structs.NodeService, error) {
	if defaults == nil {
		return service, nil
	}

	// We don't want to change s.registration in place since it is our source of
	// truth about what was actually registered before defaults applied. So copy
	// it first.
	nsRaw, err := copystructure.Copy(service)
	if err != nil {
		return nil, err
	}

	// Merge proxy defaults
	ns := nsRaw.(*structs.NodeService)

	if err := mergo.Merge(&ns.Proxy.Config, defaults.ProxyConfig); err != nil {
		return nil, err
	}

	if err := mergo.Merge(&ns.Proxy.Expose, defaults.Expose); err != nil {
		return nil, err
	}

	if ns.Proxy.MeshGateway.Mode == structs.MeshGatewayModeDefault {
		ns.Proxy.MeshGateway.Mode = defaults.MeshGateway.Mode
	}

	// Merge upstream defaults if there were any returned
	for i := range ns.Proxy.Upstreams {
		// Get a pointer not a value copy of the upstream struct
		us := &ns.Proxy.Upstreams[i]
		if us.DestinationType != "" && us.DestinationType != structs.UpstreamDestTypeService {
			continue
		}

		// default the upstreams gateway mode if it didn't specify one
		if us.MeshGateway.Mode == structs.MeshGatewayModeDefault {
			us.MeshGateway.Mode = ns.Proxy.MeshGateway.Mode
		}

		usCfg, ok := defaults.UpstreamIDConfigs.GetUpstreamConfig(us.DestinationID())
		if !ok {
			// No config defaults to merge
			continue
		}
		if err := mergo.Merge(&us.Config, usCfg); err != nil {
			return nil, err
		}
	}
	return ns, err
}
