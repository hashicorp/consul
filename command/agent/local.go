package agent

import (
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"reflect"
	"sync"
	"time"
)

const (
	syncRetryIntv = 30 * time.Second
	maxDelaySync  = 30 * time.Second
)

// syncStatus is used to represent the difference between
// the local and remote state, and if action needs to be taken
type syncStatus struct {
	remoteDelete bool // Should this be deleted from the server
	inSync       bool // Is this in sync with the server
}

// localState is used to represent the node's services,
// and checks. We used it to perform anti-entropy with the
// catalog representation
type localState struct {
	sync.Mutex

	// delaySync is used to delay the initial sync until
	// the client has registered its services and checks.
	delaySync chan struct{}

	// Services tracks the local services
	services      map[string]*structs.NodeService
	serviceStatus map[string]syncStatus

	// Checks tracks the local checks
	checks      map[string]*structs.HealthCheck
	checkStatus map[string]syncStatus

	// triggerCh is used to inform of a change to local state
	// that requires anti-entropy with the server
	triggerCh chan struct{}
}

// changeMade is used to trigger an anti-entropy run
func (l *localState) changeMade() {
	select {
	case l.triggerCh <- struct{}{}:
	default:
	}
}

// RegistrationDone is called by the Agent client once base Services
// and Checks are registered. This is called to prevent a race
// between clients and the anti-entropy routines
func (a *Agent) RegistrationDone() {
	select {
	case a.state.delaySync <- struct{}{}:
	default:
	}
}

// AddService is used to add a service entry to the local state.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (a *Agent) AddService(service *structs.NodeService) {
	// Assign the ID if none given
	if service.ID == "" && service.Service != "" {
		service.ID = service.Service
	}

	a.state.Lock()
	defer a.state.Unlock()

	a.state.services[service.ID] = service
	a.state.serviceStatus[service.ID] = syncStatus{}
	a.state.changeMade()
}

// RemoveService is used to remove a service entry from the local state.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) RemoveService(serviceID string) {
	a.state.Lock()
	defer a.state.Unlock()

	delete(a.state.services, serviceID)
	a.state.serviceStatus[serviceID] = syncStatus{remoteDelete: true}
	a.state.changeMade()
}

// Services returns the locally registered services that the
// agent is aware of and are being kept in sync with the server
func (a *Agent) Services() map[string]*structs.NodeService {
	services := make(map[string]*structs.NodeService)
	a.state.Lock()
	defer a.state.Unlock()

	for name, serv := range a.state.services {
		services[name] = serv
	}
	return services
}

// AddCheck is used to add a health check to the local state.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (a *Agent) AddCheck(check *structs.HealthCheck) {
	// Set the node name
	check.Node = a.config.NodeName

	a.state.Lock()
	defer a.state.Unlock()

	a.state.checks[check.CheckID] = check
	a.state.checkStatus[check.CheckID] = syncStatus{}
	a.state.changeMade()
}

// RemoveCheck is used to remove a health check from the local state.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) RemoveCheck(checkID string) {
	a.state.Lock()
	defer a.state.Unlock()

	delete(a.state.checks, checkID)
	a.state.checkStatus[checkID] = syncStatus{remoteDelete: true}
	a.state.changeMade()
}

// UpdateCheck is used to update the status of a check
func (a *Agent) UpdateCheck(checkID, status string) {
	a.state.Lock()
	defer a.state.Unlock()

	check, ok := a.state.checks[checkID]
	if !ok {
		return
	}

	// Do nothing if update is idempotent
	if check.Status == status {
		return
	}

	// Update status and mark out of sync
	check.Status = status
	a.state.checkStatus[checkID] = syncStatus{inSync: false}
	a.state.changeMade()
}

// Checks returns the locally registered checks that the
// agent is aware of and are being kept in sync with the server
func (a *Agent) Checks() map[string]*structs.HealthCheck {
	checks := make(map[string]*structs.HealthCheck)
	a.state.Lock()
	defer a.state.Unlock()

	for name, check := range a.state.checks {
		checks[name] = check
	}
	return checks
}

// antiEntropy is a long running method used to perform anti-entropy
// between local and remote state.
func (a *Agent) antiEntropy() {
	// Delay the initial sync until client has a chance to register
	select {
	case <-a.state.delaySync:
	case <-time.After(maxDelaySync):
		a.logger.Printf("[WARN] Client failed to call RegisterDone within %v", maxDelaySync)
	case <-a.shutdownCh:
		return
	}

SYNC:
	// Sync our state with the servers
	for !a.shutdown {
		if err := a.setSyncState(); err != nil {
			a.logger.Printf("[ERR] agent: failed to sync remote state: %v", err)
			time.Sleep(aeScale(syncRetryIntv, len(a.LANMembers())))
			continue
		}
		break
	}

	// Force-trigger AE to pickup any changes
	a.state.changeMade()

	// Schedule the next full sync, with a random stagger
	aeIntv := aeScale(a.config.AEInterval, len(a.LANMembers()))
	aeIntv = aeIntv + randomStagger(aeIntv)
	aeTimer := time.After(aeIntv)

	// Wait for sync events
	for {
		select {
		case <-aeTimer:
			goto SYNC
		case <-a.state.triggerCh:
			if err := a.syncChanges(); err != nil {
				a.logger.Printf("[ERR] agent: failed to sync changes: %v", err)
			}
		case <-a.shutdownCh:
			return
		}
	}
}

// setSyncState does a read of the server state, and updates
// the local syncStatus as appropriate
func (a *Agent) setSyncState() error {
	req := structs.NodeSpecificRequest{
		Datacenter: a.config.Datacenter,
		Node:       a.config.NodeName,
	}
	var services structs.NodeServices
	var checks structs.HealthChecks
	if e := a.RPC("Catalog.NodeServices", &req, &services); e != nil {
		return e
	}
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		return err
	}

	a.state.Lock()
	defer a.state.Unlock()

	for id, service := range services.Services {
		// If we don't have the service locally, deregister it
		existing, ok := a.state.services[id]
		if !ok {
			// The Consul service is created automatically, and
			// does not need to be registered
			if id == consul.ConsulServiceID && a.config.Server {
				continue
			}
			a.state.serviceStatus[id] = syncStatus{remoteDelete: true}
			continue
		}

		// If our definition is different, we need to update it
		equal := reflect.DeepEqual(existing, service)
		a.state.serviceStatus[id] = syncStatus{inSync: equal}
	}

	for _, check := range checks {
		// If we don't have the check locally, deregister it
		id := check.CheckID
		existing, ok := a.state.checks[id]
		if !ok {
			// The Serf check is created automatically, and does not
			// need to be registered
			if id == consul.SerfCheckID {
				continue
			}
			a.state.checkStatus[id] = syncStatus{remoteDelete: true}
			continue
		}

		// If our definition is different, we need to update it
		equal := reflect.DeepEqual(existing, check)
		a.state.checkStatus[id] = syncStatus{inSync: equal}
	}
	return nil
}

// syncChanges is used to scan the status our local services and checks
// and update any that are out of sync with the server
func (a *Agent) syncChanges() error {
	a.state.Lock()
	defer a.state.Unlock()

	// Sync the services
	for id, status := range a.state.serviceStatus {
		if status.remoteDelete {
			if err := a.deleteService(id); err != nil {
				return err
			}
		} else if !status.inSync {
			if err := a.syncService(id); err != nil {
				return err
			}
		}
	}

	// Sync the checks
	for id, status := range a.state.checkStatus {
		if status.remoteDelete {
			if err := a.deleteCheck(id); err != nil {
				return err
			}
		} else if !status.inSync {
			if err := a.syncCheck(id); err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteService is used to delete a service from the server
func (a *Agent) deleteService(id string) error {
	req := structs.DeregisterRequest{
		Datacenter: a.config.Datacenter,
		Node:       a.config.NodeName,
		ServiceID:  id,
	}
	var out struct{}
	err := a.RPC("Catalog.Deregister", &req, &out)
	if err == nil {
		delete(a.state.serviceStatus, id)
		a.logger.Printf("[INFO] Deregistered service '%s'", id)
	}
	return err
}

// deleteCheck is used to delete a service from the server
func (a *Agent) deleteCheck(id string) error {
	req := structs.DeregisterRequest{
		Datacenter: a.config.Datacenter,
		Node:       a.config.NodeName,
		CheckID:    id,
	}
	var out struct{}
	err := a.RPC("Catalog.Deregister", &req, &out)
	if err == nil {
		delete(a.state.checkStatus, id)
		a.logger.Printf("[INFO] Deregistered check '%s'", id)
	}
	return err
}

// syncService is used to sync a service to the server
func (a *Agent) syncService(id string) error {
	req := structs.RegisterRequest{
		Datacenter: a.config.Datacenter,
		Node:       a.config.NodeName,
		Address:    a.config.AdvertiseAddr,
		Service:    a.state.services[id],
	}
	var out struct{}
	err := a.RPC("Catalog.Register", &req, &out)
	if err == nil {
		a.state.serviceStatus[id] = syncStatus{inSync: true}
		a.logger.Printf("[INFO] Synced service '%s'", id)
	}
	return err
}

// syncCheck is used to sync a service to the server
func (a *Agent) syncCheck(id string) error {
	req := structs.RegisterRequest{
		Datacenter: a.config.Datacenter,
		Node:       a.config.NodeName,
		Address:    a.config.AdvertiseAddr,
		Check:      a.state.checks[id],
	}
	var out struct{}
	err := a.RPC("Catalog.Register", &req, &out)
	if err == nil {
		a.state.checkStatus[id] = syncStatus{inSync: true}
		a.logger.Printf("[INFO] Synced check '%s'", id)
	}
	return err
}
