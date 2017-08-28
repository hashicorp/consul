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

// permissionDenied is returned when an ACL based rejection happens.
const permissionDenied = "Permission denied"

// Config is the configuration for the State. It is
// populated during NewLocalAgent from the agent configuration to avoid
// race conditions with the agent configuration.
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

	// Token is the ACL to update the service record on the server.
	Token string

	// InSync contains whether the local state of the service record
	// is in sync with the remote state on the server.
	InSync bool

	// Deleted is true when the service record has been marked as deleted
	// but has not been removed on the server yet.
	Deleted bool
}

// CheckState describes the state of a health check record.
type CheckState struct {
	// Check is the local copy of the health check record.
	Check *structs.HealthCheck

	// Token is the ACL record to update the health check record
	// on the server.
	Token string

	// CriticalTime is the last time the health check status went
	// from non-critical to critical. When the health check is not
	// in critical state the value is the zero value.
	CriticalTime time.Time

	// DeferCheck is used to delay the sync of a health check when
	// only the status has changed.
	// todo(fs): ^^ this needs double checking...
	DeferCheck *time.Timer

	// InSync contains whether the local state of the health check
	// record is in sync with the remote state on the server.
	InSync bool

	// Deleted is true when the health check record has been marked as
	// deleted but has not been removed on the server yet.
	Deleted bool
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

type delegate interface {
	RPC(method string, args interface{}, reply interface{}) error
}

// State is used to represent the node's services,
// and checks. We used it to perform anti-entropy with the
// catalog representation
type State struct {
	sync.RWMutex
	logger *log.Logger

	// Config is the agent config
	config Config

	// delegate is the consul interface to use for keeping in sync
	delegate delegate

	// nodeInfoInSync tracks whether the server has our correct top-level
	// node information in sync
	nodeInfoInSync bool

	// Services tracks the local services
	services map[string]*ServiceState

	// Checks tracks the local checks
	checks map[types.CheckID]*CheckState

	// metadata tracks the local metadata fields
	metadata map[string]string

	// triggerCh is used to inform of a change to local state
	// that requires anti-entropy with the server
	triggerCh chan struct{}

	// discardCheckOutput stores whether the output of health checks
	// is stored in the raft log.
	discardCheckOutput atomic.Value // bool

	// tokens contains the ACL tokens
	tokens *token.Store
}

// NewLocalState creates a  is used to initialize the local state
func NewState(c Config, lg *log.Logger, tokens *token.Store, triggerCh chan struct{}) *State {
	l := &State{
		config:    c,
		logger:    lg,
		services:  make(map[string]*ServiceState),
		checks:    make(map[types.CheckID]*CheckState),
		metadata:  make(map[string]string),
		triggerCh: triggerCh,
		tokens:    tokens,
	}
	l.discardCheckOutput.Store(c.DiscardCheckOutput)
	return l
}

func (l *State) SetDelegate(d delegate) {
	l.delegate = d
}

// changeMade is used to trigger an anti-entropy run
func (l *State) changeMade() {
	// todo(fs): IMO, the non-blocking nature of this call should be hidden in the syncer
	select {
	case l.triggerCh <- struct{}{}:
	default:
	}
}

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
// todo(fs): where is the persistence happening?
func (l *State) AddService(service *structs.NodeService, token string) error {
	l.Lock()
	defer l.Unlock()

	if service == nil {
		return fmt.Errorf("no service")
	}

	// use the service name as id if the id was omitted
	// todo(fs): is this for backwards compatibility?
	if service.ID == "" {
		service.ID = service.Service
	}

	l.services[service.ID] = &ServiceState{
		Service: service,
		Token:   token,
	}
	l.changeMade()

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
	l.changeMade()

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

// CheckToken is used to return the configured health check token for a
// Check, or if none is configured, the default agent ACL token.
func (l *State) CheckToken(checkID types.CheckID) string {
	l.RLock()
	defer l.RUnlock()
	return l.checkToken(checkID)
}

// checkToken returns an ACL token associated with a check.
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
	l.Lock()
	defer l.Unlock()

	if check == nil {
		return fmt.Errorf("no check")
	}

	if l.discardCheckOutput.Load().(bool) {
		check.Output = ""
	}

	// if there is a serviceID associated with the check, make sure it exists before adding it
	// NOTE - This logic may be moved to be handled within the Agent's Addcheck method after a refactor
	if check.ServiceID != "" && l.services[check.ServiceID] == nil {
		return fmt.Errorf("Check %q refers to non-existent service %q does not exist", check.CheckID, check.ServiceID)
	}

	// hard-set the node name
	check.Node = l.config.NodeName

	l.checks[check.CheckID] = &CheckState{
		Check: check,
		Token: token,
	}
	l.changeMade()

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
	l.changeMade()

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
				l.changeMade()
			})
		}
		return
	}

	// Update status and mark out of sync
	c.Check.Status = status
	c.Check.Output = output
	c.InSync = false
	l.changeMade()
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
	l.RLock()
	defer l.RUnlock()

	m := make(map[types.CheckID]*structs.HealthCheck)
	for id, c := range l.checks {
		if c.Deleted {
			continue
		}
		c2 := new(structs.HealthCheck)
		*c2 = *c.Check
		m[id] = c2
	}
	return m
}

// CriticalCheckStates returns the locally registered checks that the
// agent is aware of and are being kept in sync with the server
func (l *State) CriticalCheckStates() map[types.CheckID]*CheckState {
	l.RLock()
	defer l.RUnlock()

	m := make(map[types.CheckID]*CheckState)
	for id, c := range l.checks {
		if c.Deleted || !c.Critical() {
			continue
		}
		m[id] = c
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

// UpdateSyncState does a read of the server state, and updates
// the local sync status as appropriate
func (l *State) UpdateSyncState() error {
	// 1. get all checks and services from the master
	req := structs.NodeSpecificRequest{
		Datacenter:   l.config.Datacenter,
		Node:         l.config.NodeName,
		QueryOptions: structs.QueryOptions{Token: l.tokens.AgentToken()},
	}

	var out1 structs.IndexedNodeServices
	if err := l.delegate.RPC("Catalog.NodeServices", &req, &out1); err != nil {
		return err
	}

	var out2 structs.IndexedHealthChecks
	if err := l.delegate.RPC("Health.NodeChecks", &req, &out2); err != nil {
		return err
	}

	// 2. create useful data structures for traversal
	remoteServices := make(map[string]*structs.NodeService)
	if out1.NodeServices != nil {
		remoteServices = out1.NodeServices.Services
	}

	remoteChecks := make(map[types.CheckID]*structs.HealthCheck, len(out2.HealthChecks))
	for _, rc := range out2.HealthChecks {
		remoteChecks[rc.CheckID] = rc
	}

	// 3. perform sync

	l.Lock()
	defer l.Unlock()

	// sync node info
	if out1.NodeServices == nil || out1.NodeServices.Node == nil ||
		out1.NodeServices.Node.ID != l.config.NodeID ||
		!reflect.DeepEqual(out1.NodeServices.Node.TaggedAddresses, l.config.TaggedAddresses) ||
		!reflect.DeepEqual(out1.NodeServices.Node.Meta, l.metadata) {
		l.nodeInfoInSync = false
	}

	// sync services

	// sync local services that do not exist remotely
	for id, s := range l.services {
		if remoteServices[id] == nil {
			s.InSync = false
		}
	}

	for id, rs := range remoteServices {
		// If we don't have the service locally, deregister it
		ls := l.services[id]
		if ls == nil {
			// The consul service is created automatically and does
			// not need to be deregistered.
			if id == structs.ConsulServiceID {
				continue
			}

			l.services[id] = &ServiceState{Deleted: true}
			continue
		}

		// If the service is scheduled for removal skip it.
		// todo(fs): is this correct?
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

	// sync checks

	// sync local checks which do not exist remotely
	for id, c := range l.checks {
		if remoteChecks[id] == nil {
			c.InSync = false
		}
	}

	for id, rc := range remoteChecks {

		lc := l.checks[id]

		// If we don't have the check locally, deregister it
		if lc == nil {
			// The Serf check is created automatically and does not
			// need to be deregistered.
			if id == structs.SerfCheckID {
				l.logger.Printf("Skipping remote check %q since it is managed automatically", id)
				continue
			}

			l.checks[id] = &CheckState{Deleted: true}
			continue
		}

		// If the check is scheduled for removal skip it.
		// todo(fs): is this correct?
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

// SyncChanges is used to scan the status our local services and checks
// and update any that are out of sync with the server
func (l *State) SyncChanges() error {
	l.Lock()
	defer l.Unlock()

	// We will do node-level info syncing at the end, since it will get
	// updated by a service or check sync anyway, given how the register
	// API works.

	// Sync the services
	for id, s := range l.services {
		var err error
		switch {
		case s.Deleted:
			err = l.deleteService(id)
		case !s.InSync:
			err = l.syncService(id)
		default:
			l.logger.Printf("[DEBUG] agent: Service '%s' in sync", id)
		}
		if err != nil {
			return err
		}
	}

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
			l.logger.Printf("[DEBUG] agent: Check '%s' in sync", id)
		}
		if err != nil {
			return err
		}
	}

	// Now sync the node level info if we need to, and didn't do any of
	// the other sync operations.
	if !l.nodeInfoInSync {
		if err := l.syncNodeInfo(); err != nil {
			return err
		}
	} else {
		l.logger.Printf("[DEBUG] agent: Node info in sync")
	}

	return nil
}

// LoadMetadata loads node metadata fields from the agent config and
// updates them on the local agent.
func (l *State) LoadMetadata(data map[string]string) error {
	l.Lock()
	defer l.Unlock()

	for k, v := range data {
		l.metadata[k] = v
	}
	l.changeMade()
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
	err := l.delegate.RPC("Catalog.Deregister", &req, &out)
	if err == nil || strings.Contains(err.Error(), "Unknown service") {
		delete(l.services, id)
		l.logger.Printf("[INFO] agent: Deregistered service '%s'", id)
		return nil
	}
	if acl.IsErrPermissionDenied(err) {
		// todo(fs): why is the service in sync here?
		l.services[id].InSync = true
		l.logger.Printf("[WARN] agent: Service '%s' deregistration blocked by ACLs", id)
		return nil
	}
	return err
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
	err := l.delegate.RPC("Catalog.Deregister", &req, &out)
	if err == nil || strings.Contains(err.Error(), "Unknown check") {
		// todo(fs): do we need to stop the deferCheck timer here?
		delete(l.checks, id)
		l.logger.Printf("[INFO] agent: Deregistered check '%s'", id)
		return nil
	}
	if acl.IsErrPermissionDenied(err) {
		// todo(fs): why is the check in sync here?
		l.checks[id].InSync = true
		l.logger.Printf("[WARN] agent: Check '%s' deregistration blocked by ACLs", id)
		return nil
	}
	return err
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
	err := l.delegate.RPC("Catalog.Register", &req, &out)
	if err == nil {
		l.services[id].InSync = true
		// Given how the register API works, this info is also updated
		// every time we sync a service.
		l.nodeInfoInSync = true
		for _, check := range checks {
			l.checks[check.CheckID].InSync = true
		}
		l.logger.Printf("[INFO] agent: Synced service '%s'", id)
		return nil
	}
	if acl.IsErrPermissionDenied(err) {
		// todo(fs): why are the service and the checks in sync here?
		// todo(fs): why is the node info not in sync here?
		l.services[id].InSync = true
		for _, check := range checks {
			l.checks[check.CheckID].InSync = true
		}
		l.logger.Printf("[WARN] agent: Service '%s' registration blocked by ACLs", id)
		return nil
	}
	return err
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
	err := l.delegate.RPC("Catalog.Register", &req, &out)
	if err == nil {
		l.checks[id].InSync = true
		// Given how the register API works, this info is also updated
		// every time we sync a check.
		l.nodeInfoInSync = true
		l.logger.Printf("[INFO] agent: Synced check '%s'", id)
		return nil
	}
	if acl.IsErrPermissionDenied(err) {
		// todo(fs): why is the check in sync here?
		l.checks[id].InSync = true
		l.logger.Printf("[WARN] agent: Check '%s' registration blocked by ACLs", id)
		return nil
	}
	return err
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
	err := l.delegate.RPC("Catalog.Register", &req, &out)
	if err == nil {
		l.nodeInfoInSync = true
		l.logger.Printf("[INFO] agent: Synced node info")
		return nil
	}
	if acl.IsErrPermissionDenied(err) {
		// todo(fs): why is the node info in sync here?
		l.nodeInfoInSync = true
		l.logger.Printf("[WARN] agent: Node info update blocked by ACLs")
		return nil
	}
	return err
}
