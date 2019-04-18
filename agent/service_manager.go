package agent

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"golang.org/x/net/context"
)

type ServiceManager struct {
	services map[string]*serviceConfigWatch
	agent    *Agent

	sync.Mutex
}

func NewServiceManager(agent *Agent) *ServiceManager {
	return &ServiceManager{
		services: make(map[string]*serviceConfigWatch),
		agent:    agent,
	}
}

func (s *ServiceManager) AddService(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, source configSource) {
	s.Lock()
	defer s.Unlock()

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
		watch.updateRegistration(&reg)
	} else {
		watch := &serviceConfigWatch{
			registration: &reg,
			updateCh:     make(chan cache.UpdateEvent, 1),
			agent:        s.agent,
		}

		s.services[service.ID] = watch
		watch.Start()
	}
}

func (s *ServiceManager) RemoveService(serviceID string) {
	s.Lock()
	defer s.Unlock()

	serviceWatch, ok := s.services[serviceID]
	if !ok {
		return
	}

	serviceWatch.Stop()
	delete(s.services, serviceID)
}

type serviceRegistration struct {
	service  *structs.NodeService
	chkTypes []*structs.CheckType
	persist  bool
	token    string
	source   configSource
}

type serviceConfigWatch struct {
	registration *serviceRegistration
	config       *structs.ServiceDefinition

	agent *Agent

	updateCh   chan cache.UpdateEvent
	ctx        context.Context
	cancelFunc func()

	sync.RWMutex
}

func (s *serviceConfigWatch) Start() error {
	s.ctx, s.cancelFunc = context.WithCancel(context.Background())
	if err := s.startConfigWatch(); err != nil {
		return err
	}
	go s.runWatch()

	return nil
}

func (s *serviceConfigWatch) runWatch() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.updateCh:
			s.handleUpdate(event)
		}
	}
}

func (s *serviceConfigWatch) handleUpdate(event cache.UpdateEvent) {
	switch event.Result.(type) {
	case serviceRegistration:
		s.Lock()
		s.registration = event.Result.(*serviceRegistration)
		s.Unlock()
	case structs.ServiceConfigResponse:
		s.Lock()
		s.config = &event.Result.(*structs.ServiceConfigResponse).Definition
		s.Unlock()
	default:
		s.agent.logger.Printf("[ERR] unknown update event type: %T", event)
	}

	service := s.mergeServiceConfig()
	s.agent.logger.Printf("[INFO] updating service registration: %v, %v", service.ID, service.Meta)
	/*err := s.agent.AddService(service, s.registration.chkTypes, s.registration.persist, s.registration.token, s.registration.source)
	if err != nil {
		s.agent.logger.Printf("[ERR] error updating service registration: %v", err)
	}*/
}

func (s *serviceConfigWatch) startConfigWatch() error {
	s.RLock()
	name := s.registration.service.Service
	s.RUnlock()

	req := &structs.ServiceConfigRequest{
		Name:       name,
		Datacenter: s.agent.config.Datacenter,
	}
	err := s.agent.cache.Notify(s.ctx, cachetype.ResolvedServiceConfigName, req, fmt.Sprintf("service-config:%s", name), s.updateCh)

	return err
}

func (s *serviceConfigWatch) updateRegistration(registration *serviceRegistration) {
	s.updateCh <- cache.UpdateEvent{
		Result: registration,
	}
}

func (s *serviceConfigWatch) mergeServiceConfig() *structs.NodeService {
	return nil
}

func (s *serviceConfigWatch) Stop() {
	s.cancelFunc()
}

/*
// Construct the service config request. This will be re-used with an updated
	// index to watch for changes in the effective service config.
	req := structs.ServiceConfigRequest{
		Name:         s.registration.service.Service,
		Datacenter:   s.agent.config.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.agent.tokens.AgentToken()},
	}

	consul.RetryLoopBackoff(s.shutdownCh, func() error {
		var reply structs.ServiceConfigResponse
		if err := s.agent.RPC("ConfigEntry.ResolveServiceConfig", &req, &reply); err != nil {
			return err
		}

		s.updateConfig(&reply.Definition)

		req.QueryOptions.MinQueryIndex = reply.QueryMeta.Index
		return nil
	}, func(err error) {
		s.agent.logger.Printf("[ERR] Error getting service config: %v", err)
	})
*/
