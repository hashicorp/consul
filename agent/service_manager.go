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

func (s *ServiceManager) AddService(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, source configSource) error {
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
		s.agent.logger.Printf("[DEBUG] agent: updating local registration for service %q", service.ID)
		if err := watch.updateRegistration(&reg); err != nil {
			return err
		}
	} else {
		// This is a new entry, so get the existing global config and do the initial
		// registration with the merged config.
		args := structs.ServiceConfigRequest{
			Name:         service.Service,
			Datacenter:   s.agent.config.Datacenter,
			QueryOptions: structs.QueryOptions{Token: s.agent.config.ACLAgentToken},
		}
		if token != "" {
			args.QueryOptions.Token = token
		}
		var resp structs.ServiceConfigResponse
		if err := s.agent.RPC("ConfigEntry.ResolveServiceConfig", &args, &resp); err != nil {
			s.agent.logger.Printf("[WARN] agent: could not retrieve central configuration for service %q: %v",
				service.Service, err)
		}

		watch := &serviceConfigWatch{
			updateCh: make(chan cache.UpdateEvent, 1),
			agent:    s.agent,
			config:   &resp.Definition,
		}

		// Force an update/register immediately.
		if err := watch.updateRegistration(&reg); err != nil {
			return err
		}

		s.services[service.ID] = watch
		if err := watch.Start(); err != nil {
			return err
		}
	}

	return nil
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

	sync.Mutex
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
			if err := s.handleUpdate(event, false); err != nil {
				s.agent.logger.Printf("[ERR] agent: error handling service update: %v", err)
				continue
			}
		}
	}
}

func (s *serviceConfigWatch) handleUpdate(event cache.UpdateEvent, locked bool) error {
	s.Lock()
	defer s.Unlock()

	if event.Err != nil {
		return fmt.Errorf("error watching service config: %v", event.Err)
	}

	switch event.Result.(type) {
	case *serviceRegistration:
		s.registration = event.Result.(*serviceRegistration)
	case *structs.ServiceConfigResponse:
		resp := event.Result.(*structs.ServiceConfigResponse)
		s.config = &resp.Definition
	default:
		return fmt.Errorf("unknown update event type: %T", event)
	}

	service := s.mergeServiceConfig()

	if !locked {
		s.agent.stateLock.Lock()
		defer s.agent.stateLock.Unlock()
	}

	err := s.agent.addServiceInternal(service, s.registration.chkTypes, s.registration.persist, s.registration.token, s.registration.source)
	if err != nil {
		return fmt.Errorf("error updating service registration: %v", err)
	}

	return nil
}

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

func (s *serviceConfigWatch) updateRegistration(registration *serviceRegistration) error {
	return s.handleUpdate(cache.UpdateEvent{
		Result: registration,
	}, true)
}

func (s *serviceConfigWatch) mergeServiceConfig() *structs.NodeService {
	if s.config == nil {
		return s.registration.service
	}

	svc := s.config.NodeService()
	svc.Merge(s.registration.service)

	return svc
}

func (s *serviceConfigWatch) Stop() {
	s.cancelFunc()
}
