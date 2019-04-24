package agent

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
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
	config       *structs.ServiceDefinition

	agent *Agent

	// readyCh is used for ReadyWait in order to block until the first update
	// for the resolved service config is received from the cache.
	readyCh chan error

	updateCh   chan cache.UpdateEvent
	ctx        context.Context
	cancelFunc func()

	lock sync.Mutex
}

// Start starts the config watch and a goroutine to handle updates over
// the updateCh. This is not safe to call more than once.
func (s *serviceConfigWatch) Start() error {
	s.ctx, s.cancelFunc = context.WithCancel(context.Background())
	if err := s.startConfigWatch(); err != nil {
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
		switch event.Result.(type) {
		case *serviceRegistration:
			s.registration = event.Result.(*serviceRegistration)
		case *structs.ServiceConfigResponse:
			resp := event.Result.(*structs.ServiceConfigResponse)
			s.config = &resp.Definition
		default:
			return fmt.Errorf("unknown update event type: %T", event)
		}
	}

	service := s.mergeServiceConfig()
	err := s.agent.addServiceInternal(service, s.registration.chkTypes, s.registration.persist, s.registration.token, s.registration.source)
	if err != nil {
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

// startConfigWatch starts a cache.Notify goroutine to run a continuous blocking query
// on the resolved service config for this service.
func (s *serviceConfigWatch) startConfigWatch() error {
	name := s.registration.service.Service

	req := &structs.ServiceConfigRequest{
		Name:         name,
		Datacenter:   s.agent.config.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.agent.config.ACLAgentToken},
	}
	if s.registration.token != "" {
		req.QueryOptions.Token = s.registration.token
	}
	err := s.agent.cache.Notify(s.ctx, cachetype.ResolvedServiceConfigName, req, fmt.Sprintf("service-config:%s", name), s.updateCh)

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
func (s *serviceConfigWatch) mergeServiceConfig() *structs.NodeService {
	if s.config == nil {
		return s.registration.service
	}

	svc := s.config.NodeService()
	svc.Merge(s.registration.service)

	return svc
}
