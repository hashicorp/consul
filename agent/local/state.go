package local

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
)

// Config is the configuration for the State.
type Config struct {
	AdvertiseAddr       string
	CheckUpdateInterval time.Duration
	Datacenter          string
	DiscardCheckOutput  bool
	NodeID              types.NodeID
	NodeName            string
	TaggedAddresses     map[string]string
}

// ServiceState describes the state of a service record.
type ServiceState struct {
	// Service is the local copy of the service record.
	Service *structs.NodeService

	// Token is the ACL to update or delete the service record on the
	// server.
	Token string

	// InSync contains whether the local state of the service record
	// is in sync with the remote state on the server.
	InSync bool

	// Deleted is true when the service record has been marked as deleted
	// but has not been removed on the server yet.
	Deleted bool
}

// Clone returns a shallow copy of the object. The service record still
// points to the original service record and must not be modified.
func (s *ServiceState) Clone() *ServiceState {
	s2 := new(ServiceState)
	*s2 = *s
	return s2
}

// CheckState describes the state of a health check record.
type CheckState struct {
	// Check is the local copy of the health check record.
	Check *structs.HealthCheck

	// Token is the ACL record to update or delete the health check
	// record on the server.
	Token string

	// CriticalTime is the last time the health check status went
	// from non-critical to critical. When the health check is not
	// in critical state the value is the zero value.
	CriticalTime time.Time

	// DeferCheck is used to delay the sync of a health check when
	// only the output has changed. This rate limits changes which
	// do not affect the state of the node and/or service.
	DeferCheck *time.Timer

	// InSync contains whether the local state of the health check
	// record is in sync with the remote state on the server.
	InSync bool

	// Deleted is true when the health check record has been marked as
	// deleted but has not been removed on the server yet.
	Deleted bool
}

// Clone returns a shallow copy of the object. The check record and the
// defer timer still point to the original values and must not be
// modified.
func (c *CheckState) Clone() *CheckState {
	c2 := new(CheckState)
	*c2 = *c
	return c2
}

// Critical returns true when the health check is in critical state.
func (c *CheckState) Critical() bool {
	return !c.CriticalTime.IsZero()
}

// CriticalFor returns the amount of time the service has been in critical
// state. Its value is undefined when the service is not in critical state.
func (c *CheckState) CriticalFor() time.Duration {
	return time.Since(c.CriticalTime)
}

type rpc interface {
	RPC(method string, args interface{}, reply interface{}) error
}

// State is used to represent the node's services,
// and checks. We use it to perform anti-entropy with the
// catalog representation
type State struct {
	sync.RWMutex

	// Delegate the RPC interface to the consul server or agent.
	//
	// It is set after both the state and the consul server/agent have
	// been created.
	Delegate rpc

	// TriggerSyncChanges is used to notify the state syncer that a
	// partial sync should be performed.
	//
	// It is set after both the state and the state syncer have been
	// created.
	TriggerSyncChanges func()

	logger *log.Logger

	// Config is the agent config
	config Config

	// nodeInfoInSync tracks whether the server has our correct top-level
	// node information in sync
	nodeInfoInSync bool

	// Services tracks the local services
	services map[string]*ServiceState

	// Checks tracks the local checks
	checks map[types.CheckID]*CheckState

	// metadata tracks the node metadata fields
	metadata map[string]string

	// discardCheckOutput stores whether the output of health checks
	// is stored in the raft log.
	discardCheckOutput atomic.Value // bool

	// tokens contains the ACL tokens
	tokens *token.Store
}

// NewLocalState creates a new local state for the agent.
func NewState(c Config, lg *log.Logger, tokens *token.Store) *State {
	l := &State{
		config:   c,
		logger:   lg,
		services: make(map[string]*ServiceState),
		checks:   make(map[types.CheckID]*CheckState),
		metadata: make(map[string]string),
		tokens:   tokens,
	}
	l.SetDiscardCheckOutput(c.DiscardCheckOutput)
	return l
}

// SetDiscardCheckOutput configures whether the check output
// is discarded. This can be changed at runtime.
func (l *State) SetDiscardCheckOutput(b bool) {
	l.discardCheckOutput.Store(b)
}

// ServiceToken returns the configured ACL token for the given
// service ID. If none is present, the agent's token is returned.
func (l *State) ServiceToken(id string) string {
	l.RLock()
	defer l.RUnlock()
	return l.serviceToken(id)
}

// serviceToken returns an ACL token associated with a service.
// This method is not synchronized and the lock must already be held.
func (l *State) serviceToken(id string) string {
	var token string
	if s := l.services[id]; s != nil {
		token = s.Token
	}
	if token == "" {
		token = l.tokens.UserToken()
	}
	return token
}

// AddService is used to add a service entry to the local state.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (l *State) AddService(service *structs.NodeService, token string) error {
	if service == nil {
		return fmt.Errorf("no service")
	}

	// use the service name as id if the id was omitted
	if service.ID == "" {
		service.ID = service.Service
	}

	l.SetServiceState(&ServiceState{
		Service: service,
		Token:   token,
	})
	return nil
}

// RemoveService is used to remove a service entry from the local state.
// The agent will make a best effort to ensure it is deregistered.
func (l *State) RemoveService(id string) error {
	l.Lock()
	defer l.Unlock()

	s := l.services[id]
	if s == nil || s.Deleted {
		return fmt.Errorf("Service %q does not exist", id)
	}

	// To remove the service on the server we need the token.
	// Therefore, we mark the service as deleted and keep the
	// entry around until it is actually removed.
	s.InSync = false
	s.Deleted = true
	l.TriggerSyncChanges()

	return nil
}

// Service returns the locally registered service that the
// agent is aware of and are being kept in sync with the server
func (l *State) Service(id string) *structs.NodeService {
	l.RLock()
	defer l.RUnlock()

	s := l.services[id]
	if s == nil || s.Deleted {
		return nil
	}
	return s.Service
}

// Services returns the locally registered services that the
// agent is aware of and are being kept in sync with the server
func (l *State) Services() map[string]*structs.NodeService {
	l.RLock()
	defer l.RUnlock()

	m := make(map[string]*structs.NodeService)
	for id, s := range l.services {
		if s.Deleted {
			continue
		}
		m[id] = s.Service
	}
	return m
}

// ServiceState returns a shallow copy of the current service state
// record. The service record still points to the original service
// record and must not be modified.
func (l *State) ServiceState(id string) *ServiceState {
	l.RLock()
	defer l.RUnlock()

	s := l.services[id]
	if s == nil || s.Deleted {
		return nil
	}
	return s.Clone()
}

// SetServiceState is used to overwrite a raw service state with the given
// state. This method is safe to be called concurrently but should only be used
// during testing. You should most likely call AddService instead.
func (l *State) SetServiceState(s *ServiceState) {
	l.Lock()
	defer l.Unlock()

	l.services[s.Service.ID] = s
	l.TriggerSyncChanges()
}

// ServiceStates returns a shallow copy of all service state records.
// The service record still points to the original service record and
// must not be modified.
func (l *State) ServiceStates() map[string]*ServiceState {
	l.RLock()
	defer l.RUnlock()

	m := make(map[string]*ServiceState)
	for id, s := range l.services {
		if s.Deleted {
			continue
		}
		m[id] = s.Clone()
	}
	return m
}

// CheckToken is used to return the configured health check token for a
// Check, or if none is configured, the default agent ACL token.
func (l *State) CheckToken(checkID types.CheckID) string {
	l.RLock()
	defer l.RUnlock()
	return l.checkToken(checkID)
}

// checkToken returns an ACL token associated with a check.
// This method is not synchronized and the lock must already be held.
func (l *State) checkToken(id types.CheckID) string {
	var token string
	c := l.checks[id]
	if c != nil {
		token = c.Token
	}
	if token == "" {
		token = l.tokens.UserToken()
	}
	return token
}

// AddCheck is used to add a health check to the local state.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (l *State) AddCheck(check *structs.HealthCheck, token string) error {
	if check == nil {
		return fmt.Errorf("no check")
	}

	// clone the check since we will be modifying it.
	check = check.Clone()

	if l.discardCheckOutput.Load().(bool) {
		check.Output = ""
	}

	// if there is a serviceID associated with the check, make sure it exists before adding it
	// NOTE - This logic may be moved to be handled within the Agent's Addcheck method after a refactor
	if check.ServiceID != "" && l.Service(check.ServiceID) == nil {
		return fmt.Errorf("Check %q refers to non-existent service %q", check.CheckID, check.ServiceID)
	}

	// hard-set the node name
	check.Node = l.config.NodeName

	l.SetCheckState(&CheckState{
		Check: check,
		Token: token,
	})
	return nil
}

// RemoveCheck is used to remove a health check from the local state.
// The agent will make a best effort to ensure it is deregistered
// todo(fs): RemoveService returns an error for a non-existant service. RemoveCheck should as well.
// todo(fs): Check code that calls this to handle the error.
func (l *State) RemoveCheck(id types.CheckID) error {
	l.Lock()
	defer l.Unlock()

	c := l.checks[id]
	if c == nil || c.Deleted {
		return fmt.Errorf("Check %q does not exist", id)
	}

	// To remove the check on the server we need the token.
	// Therefore, we mark the service as deleted and keep the
	// entry around until it is actually removed.
	c.InSync = false
	c.Deleted = true
	l.TriggerSyncChanges()

	return nil
}

// UpdateCheck is used to update the status of a check
func (l *State) UpdateCheck(id types.CheckID, status, output string) {
	l.Lock()
	defer l.Unlock()

	c := l.checks[id]
	if c == nil || c.Deleted {
		return
	}

	if l.discardCheckOutput.Load().(bool) {
		output = ""
	}

	// Update the critical time tracking (this doesn't cause a server updates
	// so we can always keep this up to date).
	if status == api.HealthCritical {
		if !c.Critical() {
			c.CriticalTime = time.Now()
		}
	} else {
		c.CriticalTime = time.Time{}
	}

	// Do nothing if update is idempotent
	if c.Check.Status == status && c.Check.Output == output {
		return
	}

	// Defer a sync if the output has changed. This is an optimization around
	// frequent updates of output. Instead, we update the output internally,
	// and periodically do a write-back to the servers. If there is a status
	// change we do the write immediately.
	if l.config.CheckUpdateInterval > 0 && c.Check.Status == status {
		c.Check.Output = output
		if c.DeferCheck == nil {
			d := l.config.CheckUpdateInterval
			intv := time.Duration(uint64(d)/2) + lib.RandomStagger(d)
			c.DeferCheck = time.AfterFunc(intv, func() {
				l.Lock()
				defer l.Unlock()

				c := l.checks[id]
				if c == nil {
					return
				}
				c.DeferCheck = nil
				if c.Deleted {
					return
				}
				c.InSync = false
				l.TriggerSyncChanges()
			})
		}
		return
	}

	// Update status and mark out of sync
	c.Check.Status = status
	c.Check.Output = output
	c.InSync = false
	l.TriggerSyncChanges()
}

// Check returns the locally registered check that the
// agent is aware of and are being kept in sync with the server
func (l *State) Check(id types.CheckID) *structs.HealthCheck {
	l.RLock()
	defer l.RUnlock()

	c := l.checks[id]
	if c == nil || c.Deleted {
		return nil
	}
	return c.Check
}

// Checks returns the locally registered checks that the
// agent is aware of and are being kept in sync with the server
func (l *State) Checks() map[types.CheckID]*structs.HealthCheck {
	m := make(map[types.CheckID]*structs.HealthCheck)
	for id, c := range l.CheckStates() {
		m[id] = c.Check
	}
	return m
}

// CheckState returns a shallow copy of the current health check state
// record. The health check record and the deferred check still point to
// the original values and must not be modified.
func (l *State) CheckState(id types.CheckID) *CheckState {
	l.RLock()
	defer l.RUnlock()

	c := l.checks[id]
	if c == nil || c.Deleted {
		return nil
	}
	return c.Clone()
}

// SetCheckState is used to overwrite a raw check state with the given
// state. This method is safe to be called concurrently but should only be used
// during testing. You should most likely call AddCheck instead.
func (l *State) SetCheckState(c *CheckState) {
	l.Lock()
	defer l.Unlock()

	l.checks[c.Check.CheckID] = c
	l.TriggerSyncChanges()
}

// CheckStates returns a shallow copy of all health check state records.
// The health check records and the deferred checks still point to
// the original values and must not be modified.
func (l *State) CheckStates() map[types.CheckID]*CheckState {
	l.RLock()
	defer l.RUnlock()

	m := make(map[types.CheckID]*CheckState)
	for id, c := range l.checks {
		if c.Deleted {
			continue
		}
		m[id] = c.Clone()
	}
	return m
}

// CriticalCheckStates returns the locally registered checks that the
// agent is aware of and are being kept in sync with the server.
// The map contains a shallow copy of the current check states but
// references to the actual check definition which must not be
// modified.
func (l *State) CriticalCheckStates() map[types.CheckID]*CheckState {
	l.RLock()
	defer l.RUnlock()

	m := make(map[types.CheckID]*CheckState)
	for id, c := range l.checks {
		if c.Deleted || !c.Critical() {
			continue
		}
		m[id] = c.Clone()
	}
	return m
}

// Metadata returns the local node metadata fields that the
// agent is aware of and are being kept in sync with the server
func (l *State) Metadata() map[string]string {
	l.RLock()
	defer l.RUnlock()

	m := make(map[string]string)
	for k, v := range l.metadata {
		m[k] = v
	}
	return m
}

// LoadMetadata loads node metadata fields from the agent config and
// updates them on the local agent.
func (l *State) LoadMetadata(data map[string]string) error {
	l.Lock()
	defer l.Unlock()

	for k, v := range data {
		l.metadata[k] = v
	}
	l.TriggerSyncChanges()
	return nil
}

// UnloadMetadata resets the local metadata state
func (l *State) UnloadMetadata() {
	l.Lock()
	defer l.Unlock()
	l.metadata = make(map[string]string)
}

// Stats is used to get various debugging state from the sub-systems
func (l *State) Stats() map[string]string {
	l.RLock()
	defer l.RUnlock()

	services := 0
	for _, s := range l.services {
		if s.Deleted {
			continue
		}
		services++
	}

	checks := 0
	for _, c := range l.checks {
		if c.Deleted {
			continue
		}
		checks++
	}

	return map[string]string{
		"services": strconv.Itoa(services),
		"checks":   strconv.Itoa(checks),
	}
}

// updateSyncState does a read of the server state, and updates
// the local sync status as appropriate
func (l *State) updateSyncState() error {
	// Get all checks and services from the master
	req := structs.NodeSpecificRequest{
		Datacenter:   l.config.Datacenter,
		Node:         l.config.NodeName,
		QueryOptions: structs.QueryOptions{Token: l.tokens.AgentToken()},
	}

	var out1 structs.IndexedNodeServices
	if err := l.Delegate.RPC("Catalog.NodeServices", &req, &out1); err != nil {
		return err
	}

	var out2 structs.IndexedHealthChecks
	if err := l.Delegate.RPC("Health.NodeChecks", &req, &out2); err != nil {
		return err
	}

	// Create useful data structures for traversal
	remoteServices := make(map[string]*structs.NodeService)
	if out1.NodeServices != nil {
		remoteServices = out1.NodeServices.Services
	}

	remoteChecks := make(map[types.CheckID]*structs.HealthCheck, len(out2.HealthChecks))
	for _, rc := range out2.HealthChecks {
		remoteChecks[rc.CheckID] = rc
	}

	// Traverse all checks, services and the node info to determine
	// which entries need to be updated on or removed from the server

	l.Lock()
	defer l.Unlock()

	// Check if node info needs syncing
	if out1.NodeServices == nil || out1.NodeServices.Node == nil ||
		out1.NodeServices.Node.ID != l.config.NodeID ||
		!reflect.DeepEqual(out1.NodeServices.Node.TaggedAddresses, l.config.TaggedAddresses) ||
		!reflect.DeepEqual(out1.NodeServices.Node.Meta, l.metadata) {
		l.nodeInfoInSync = false
	}

	// Check which services need syncing

	// Look for local services that do not exist remotely and mark them for
	// syncing so that they will be pushed to the server later
	for id, s := range l.services {
		if remoteServices[id] == nil {
			s.InSync = false
		}
	}

	// Traverse the list of services from the server.
	// Remote services which do not exist locally have been deregistered.
	// Otherwise, check whether the two definitions are still in sync.
	for id, rs := range remoteServices {
		ls := l.services[id]
		if ls == nil {
			// The consul service is managed automatically and does
			// not need to be deregistered
			if id == structs.ConsulServiceID {
				continue
			}

			// Mark a remote service that does not exist locally as deleted so
			// that it will be removed on the server later.
			l.services[id] = &ServiceState{Deleted: true}
			continue
		}

		// If the service is already scheduled for removal skip it
		if ls.Deleted {
			continue
		}

		// If our definition is different, we need to update it. Make a
		// copy so that we don't retain a pointer to any actual state
		// store info for in-memory RPCs.
		if ls.Service.EnableTagOverride {
			ls.Service.Tags = make([]string, len(rs.Tags))
			copy(ls.Service.Tags, rs.Tags)
		}
		ls.InSync = ls.Service.IsSame(rs)
	}

	// Check which checks need syncing

	// Look for local checks that do not exist remotely and mark them for
	// syncing so that they will be pushed to the server later
	for id, c := range l.checks {
		if remoteChecks[id] == nil {
			c.InSync = false
		}
	}

	// Traverse the list of checks from the server.
	// Remote checks which do not exist locally have been deregistered.
	// Otherwise, check whether the two definitions are still in sync.
	for id, rc := range remoteChecks {
		lc := l.checks[id]

		if lc == nil {
			// The Serf check is created automatically and does not
			// need to be deregistered.
			if id == structs.SerfCheckID {
				l.logger.Printf("[DEBUG] Skipping remote check %q since it is managed automatically", id)
				continue
			}

			// Mark a remote check that does not exist locally as deleted so
			// that it will be removed on the server later.
			l.checks[id] = &CheckState{Deleted: true}
			continue
		}

		// If the check is already scheduled for removal skip it.
		if lc.Deleted {
			continue
		}

		// If our definition is different, we need to update it
		if l.config.CheckUpdateInterval == 0 {
			lc.InSync = lc.Check.IsSame(rc)
			continue
		}

		// Copy the existing check before potentially modifying
		// it before the compare operation.
		lcCopy := lc.Check.Clone()

		// Copy the server's check before modifying, otherwise
		// in-memory RPCs will have side effects.
		rcCopy := rc.Clone()

		// If there's a defer timer active then we've got a
		// potentially spammy check so we don't sync the output
		// during this sweep since the timer will mark the check
		// out of sync for us. Otherwise, it is safe to sync the
		// output now. This is especially important for checks
		// that don't change state after they are created, in
		// which case we'd never see their output synced back ever.
		if lc.DeferCheck != nil {
			lcCopy.Output = ""
			rcCopy.Output = ""
		}
		lc.InSync = lcCopy.IsSame(rcCopy)
	}
	return nil
}

// SyncFull determines the delta between the local and remote state
// and synchronizes the changes.
func (l *State) SyncFull() error {
	// note that we do not acquire the lock here since the methods
	// we are calling will do that themself.
	//
	// Also note that we don't hold the lock for the entire operation
	// but release it between the two calls. This is not an issue since
	// the algorithm is best-effort to achieve eventual consistency.
	// SyncChanges will sync whatever updateSyncState() has determined
	// needs updating.

	if err := l.updateSyncState(); err != nil {
		return err
	}
	return l.SyncChanges()
}

// SyncChanges pushes checks, services and node info data which has been
// marked out of sync or deleted to the server.
func (l *State) SyncChanges() error {
	l.Lock()
	defer l.Unlock()

	// We will do node-level info syncing at the end, since it will get
	// updated by a service or check sync anyway, given how the register
	// API works.

	// Sync the services
	// (logging happens in the helper methods)
	for id, s := range l.services {
		var err error
		switch {
		case s.Deleted:
			err = l.deleteService(id)
		case !s.InSync:
			err = l.syncService(id)
		default:
			l.logger.Printf("[DEBUG] agent: Service %q in sync", id)
		}
		if err != nil {
			return err
		}
	}

	// Sync the checks
	// (logging happens in the helper methods)
	for id, c := range l.checks {
		var err error
		switch {
		case c.Deleted:
			err = l.deleteCheck(id)
		case !c.InSync:
			if c.DeferCheck != nil {
				c.DeferCheck.Stop()
				c.DeferCheck = nil
			}
			err = l.syncCheck(id)
		default:
			l.logger.Printf("[DEBUG] agent: Check %q in sync", id)
		}
		if err != nil {
			return err
		}
	}

	// Now sync the node level info if we need to, and didn't do any of
	// the other sync operations.
	if l.nodeInfoInSync {
		l.logger.Printf("[DEBUG] agent: Node info in sync")
		return nil
	}
	return l.syncNodeInfo()
}

// deleteService is used to delete a service from the server
func (l *State) deleteService(id string) error {
	if id == "" {
		return fmt.Errorf("ServiceID missing")
	}

	req := structs.DeregisterRequest{
		Datacenter:   l.config.Datacenter,
		Node:         l.config.NodeName,
		ServiceID:    id,
		WriteRequest: structs.WriteRequest{Token: l.serviceToken(id)},
	}
	var out struct{}
	err := l.Delegate.RPC("Catalog.Deregister", &req, &out)
	switch {
	case err == nil || strings.Contains(err.Error(), "Unknown service"):
		delete(l.services, id)
		l.logger.Printf("[INFO] agent: Deregistered service %q", id)
		return nil

	case acl.IsErrPermissionDenied(err):
		// todo(fs): mark the service to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.services[id].InSync = true
		l.logger.Printf("[WARN] agent: Service %q deregistration blocked by ACLs", id)
		return nil

	default:
		l.logger.Printf("[WARN] agent: Deregistering service %q failed. %s", id, err)
		return err
	}
}

// deleteCheck is used to delete a check from the server
func (l *State) deleteCheck(id types.CheckID) error {
	if id == "" {
		return fmt.Errorf("CheckID missing")
	}

	req := structs.DeregisterRequest{
		Datacenter:   l.config.Datacenter,
		Node:         l.config.NodeName,
		CheckID:      id,
		WriteRequest: structs.WriteRequest{Token: l.checkToken(id)},
	}
	var out struct{}
	err := l.Delegate.RPC("Catalog.Deregister", &req, &out)
	switch {
	case err == nil || strings.Contains(err.Error(), "Unknown check"):
		c := l.checks[id]
		if c != nil && c.DeferCheck != nil {
			c.DeferCheck.Stop()
		}
		delete(l.checks, id)
		l.logger.Printf("[INFO] agent: Deregistered check %q", id)
		return nil

	case acl.IsErrPermissionDenied(err):
		// todo(fs): mark the check to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.checks[id].InSync = true
		l.logger.Printf("[WARN] agent: Check %q deregistration blocked by ACLs", id)
		return nil

	default:
		l.logger.Printf("[WARN] agent: Deregistering check %q failed. %s", id, err)
		return err
	}
}

// syncService is used to sync a service to the server
func (l *State) syncService(id string) error {
	// If the service has associated checks that are out of sync,
	// piggyback them on the service sync so they are part of the
	// same transaction and are registered atomically. We only let
	// checks ride on service registrations with the same token,
	// otherwise we need to register them separately so they don't
	// pick up privileges from the service token.
	var checks structs.HealthChecks
	for checkID, c := range l.checks {
		if c.Deleted || c.InSync {
			continue
		}
		if c.Check.ServiceID != id {
			continue
		}
		if l.serviceToken(id) != l.checkToken(checkID) {
			continue
		}
		checks = append(checks, c.Check)
	}

	req := structs.RegisterRequest{
		Datacenter:      l.config.Datacenter,
		ID:              l.config.NodeID,
		Node:            l.config.NodeName,
		Address:         l.config.AdvertiseAddr,
		TaggedAddresses: l.config.TaggedAddresses,
		NodeMeta:        l.metadata,
		Service:         l.services[id].Service,
		WriteRequest:    structs.WriteRequest{Token: l.serviceToken(id)},
	}

	// Backwards-compatibility for Consul < 0.5
	if len(checks) == 1 {
		req.Check = checks[0]
	} else {
		req.Checks = checks
	}

	var out struct{}
	err := l.Delegate.RPC("Catalog.Register", &req, &out)
	switch {
	case err == nil:
		l.services[id].InSync = true
		// Given how the register API works, this info is also updated
		// every time we sync a service.
		l.nodeInfoInSync = true
		for _, check := range checks {
			l.checks[check.CheckID].InSync = true
		}
		l.logger.Printf("[INFO] agent: Synced service %q", id)
		return nil

	case acl.IsErrPermissionDenied(err):
		// todo(fs): mark the service and the checks to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.services[id].InSync = true
		for _, check := range checks {
			l.checks[check.CheckID].InSync = true
		}
		l.logger.Printf("[WARN] agent: Service %q registration blocked by ACLs", id)
		return nil

	default:
		l.logger.Printf("[WARN] agent: Syncing service %q failed. %s", id, err)
		return err
	}
}

// syncCheck is used to sync a check to the server
func (l *State) syncCheck(id types.CheckID) error {
	c := l.checks[id]

	req := structs.RegisterRequest{
		Datacenter:      l.config.Datacenter,
		ID:              l.config.NodeID,
		Node:            l.config.NodeName,
		Address:         l.config.AdvertiseAddr,
		TaggedAddresses: l.config.TaggedAddresses,
		NodeMeta:        l.metadata,
		Check:           c.Check,
		WriteRequest:    structs.WriteRequest{Token: l.checkToken(id)},
	}

	// Pull in the associated service if any
	s := l.services[c.Check.ServiceID]
	if s != nil && !s.Deleted {
		req.Service = s.Service
	}

	var out struct{}
	err := l.Delegate.RPC("Catalog.Register", &req, &out)
	switch {
	case err == nil:
		l.checks[id].InSync = true
		// Given how the register API works, this info is also updated
		// every time we sync a check.
		l.nodeInfoInSync = true
		l.logger.Printf("[INFO] agent: Synced check %q", id)
		return nil

	case acl.IsErrPermissionDenied(err):
		// todo(fs): mark the check to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.checks[id].InSync = true
		l.logger.Printf("[WARN] agent: Check %q registration blocked by ACLs", id)
		return nil

	default:
		l.logger.Printf("[WARN] agent: Syncing check %q failed. %s", id, err)
		return err
	}
}

func (l *State) syncNodeInfo() error {
	req := structs.RegisterRequest{
		Datacenter:      l.config.Datacenter,
		ID:              l.config.NodeID,
		Node:            l.config.NodeName,
		Address:         l.config.AdvertiseAddr,
		TaggedAddresses: l.config.TaggedAddresses,
		NodeMeta:        l.metadata,
		WriteRequest:    structs.WriteRequest{Token: l.tokens.AgentToken()},
	}
	var out struct{}
	err := l.Delegate.RPC("Catalog.Register", &req, &out)
	switch {
	case err == nil:
		l.nodeInfoInSync = true
		l.logger.Printf("[INFO] agent: Synced node info")
		return nil

	case acl.IsErrPermissionDenied(err):
		// todo(fs): mark the node info to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.nodeInfoInSync = true
		l.logger.Printf("[WARN] agent: Node info update blocked by ACLs")
		return nil

	default:
		l.logger.Printf("[WARN] agent: Syncing node info failed. %s", err)
		return err
	}
}
