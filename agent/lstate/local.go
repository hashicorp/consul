package lstate

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/rpc"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/serf/serf"
)

const (
	syncStaggerIntv = 3 * time.Second
	syncRetryIntv   = 15 * time.Second
)

var permissionDenied = "Permission denied"

// Config is the configuration for the State. It is
// populated during NewLocalAgent from the agent configuration to avoid
// race conditions with the agent configuration.
type Config struct {
	ACLToken            string
	AEInterval          time.Duration
	AdvertiseAddr       string
	CheckUpdateInterval time.Duration
	Datacenter          string
	NodeID              types.NodeID
	NodeName            string
	TaggedAddresses     map[string]string
	TokenForAgent       string
}

// delegate is the consul rpc interface plus whatever
// the AE/sync code needs right now. This is either an
// *rpc.Server or *rpc.Client.
// The AE/sync code should move into its own package.
type delegate interface {
	LANMembers() []serf.Member
	RPC(method string, args interface{}, reply interface{}) error
}

// ServiceState captures the local state of a service. When Service
// is nil the entry is marked for removal.
type ServiceState struct {
	Service *structs.NodeService
	InSync  bool
	Token   string
}

// CheckState captures the local state of a check. When Check is nil
// the entry is marked for removal.
type CheckState struct {
	Check         *structs.HealthCheck
	InSync        bool
	Token         string
	CriticalSince time.Time
}

// State is used to represent the node's services,
// and checks. We used it to perform anti-entropy with the
// catalog representation
type State struct {
	// paused is used to check if we are paused. Must be the first
	// element due to a go bug.
	paused int32

	logger *log.Logger

	// Config is the state config
	config Config

	// delegate is the consul interface to use for keeping in sync
	delegate delegate

	// nodeInfoInSync tracks whether the server has our correct top-level
	// node information in sync
	nodeInfoInSync bool

	// mu is an explicit lock that guards the data structures
	// below. It is a member instead of being embedded to prevent
	// users of state to control the lock.
	mu sync.RWMutex

	// Services tracks the local services
	services map[string]*ServiceState

	// Checks tracks the local checks
	checks map[types.CheckID]*CheckState

	// metadata tracks the local metadata fields
	metadata map[string]string

	// Used to track checks that are being deferred
	deferCheck map[types.CheckID]*time.Timer

	// consulCh is used to inform of a change to the known
	// consul nodes. This may be used to retry a sync run
	consulCh chan struct{}

	// triggerCh is used to inform of a change to local state
	// that requires anti-entropy with the server
	triggerCh chan struct{}
}

// NewState creates a  is used to initialize the local state
func NewState(c Config, lg *log.Logger) *State {
	return &State{
		config:     c,
		logger:     lg,
		services:   make(map[string]*ServiceState),
		checks:     make(map[types.CheckID]*CheckState),
		metadata:   make(map[string]string),
		deferCheck: make(map[types.CheckID]*time.Timer),
		consulCh:   make(chan struct{}, 1),
		triggerCh:  make(chan struct{}, 1),
	}
}

func (l *State) SetDelegate(d delegate) {
	l.delegate = d
}

// changeMade is used to trigger an anti-entropy run
func (l *State) changeMade() {
	select {
	case l.triggerCh <- struct{}{}:
	default:
	}
}

// ConsulServerUp is used to inform that a new consul server is now
// up. This can be used to speed up the sync process if we are blocking
// waiting to discover a consul server
func (l *State) ConsulServerUp() {
	select {
	case l.consulCh <- struct{}{}:
	default:
	}
}

// Pause is used to pause state synchronization, this can be
// used to make batch changes
func (l *State) Pause() {
	atomic.AddInt32(&l.paused, 1)
}

// Resume is used to resume state synchronization
func (l *State) Resume() {
	paused := atomic.AddInt32(&l.paused, -1)
	if paused < 0 {
		panic("unbalanced State.Resume() detected")
	}
	l.changeMade()
}

// isPaused is used to check if we are paused
func (l *State) isPaused() bool {
	return atomic.LoadInt32(&l.paused) > 0
}

// ServiceToken returns the configured ACL token for the given
// service ID. If none is present, the agent's token is returned.
func (l *State) ServiceToken(id string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.serviceToken(id)
}

// serviceToken returns an ACL token associated with a service.
func (l *State) serviceToken(id string) string {
	var token string
	if s := l.services[id]; s != nil {
		token = s.Token
	}
	if token == "" {
		token = l.config.ACLToken
	}
	return token
}

// AddService is used to add a service entry to the local state.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (l *State) AddService(service *structs.NodeService, token string) {
	// Assign the ID if none given
	if service.ID == "" && service.Service != "" {
		service.ID = service.Service
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.services[service.ID] = &ServiceState{
		Service: service,
		InSync:  false,
		Token:   token,
	}
	l.changeMade()
}

// RemoveService is used to remove a service entry from the local state.
// The agent will make a best effort to ensure it is deregistered
func (l *State) RemoveService(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	s := l.services[id]
	if s == nil || s.Service == nil {
		return fmt.Errorf("Service does not exist")
	}

	// Mark the service as deleted but eave the service token around, if
	// any, until we successfully delete the service.
	s.Service = nil
	s.InSync = false

	// todo(fs): Shouldn't we call l.changeMade() here?

	return nil
}

// Service returns the locally registered service that the
// agent is aware of and are being kept in sync with the server
func (l *State) Service(id string) *ServiceState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if s := l.services[id]; s != nil && s.Service != nil {
		return s
	}
	return nil
}

// Services returns the locally registered services that the
// agent is aware of and are being kept in sync with the server
func (l *State) Services() map[string]*ServiceState {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := make(map[string]*ServiceState)
	for k, v := range l.services {
		if v.Service == nil {
			continue
		}
		m[k] = v
	}
	return m
}

// CheckToken is used to return the configured health check token for a
// Check, or if none is configured, the default agent ACL token.
func (l *State) CheckToken(id types.CheckID) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.checkToken(id)
}

// checkToken returns an ACL token associated with a check.
func (l *State) checkToken(id types.CheckID) string {
	var token string
	if c, ok := l.checks[id]; ok {
		token = c.Token
	}
	if token == "" {
		token = l.config.ACLToken
	}
	return token
}

// AddCheck is used to add a health check to the local state.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (l *State) AddCheck(check *structs.HealthCheck, token string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	check.Node = l.config.NodeName

	l.checks[check.CheckID] = &CheckState{
		Check:  check,
		InSync: false,
		Token:  token,
	}

	l.changeMade()
}

// RemoveCheck is used to remove a health check from the local state.
// The agent will make a best effort to ensure it is deregistered
func (l *State) RemoveCheck(id types.CheckID) {
	l.mu.Lock()
	defer l.mu.Unlock()

	c := l.checks[id]
	if c == nil || c.Check == nil {
		return
	}

	// Leave the check token around, if any, until we successfully
	// delete the check.
	c.Check = nil
	c.InSync = false
	c.CriticalSince = time.Time{}

	l.changeMade()
}

// UpdateCheck is used to update the status of a check
func (l *State) UpdateCheck(id types.CheckID, status, output string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	c := l.checks[id]
	if c == nil || c.Check == nil {
		return
	}

	// Update the critical time tracking (this doesn't cause a server updates
	// so we can always keep this up to date).
	if status == api.HealthCritical {
		if c.CriticalSince.IsZero() {
			c.CriticalSince = time.Now()
		}
	} else {
		c.CriticalSince = time.Time{}
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
		if _, ok := l.deferCheck[id]; !ok {
			intv := time.Duration(uint64(l.config.CheckUpdateInterval)/2) + lib.RandomStagger(l.config.CheckUpdateInterval)
			l.deferCheck[id] = time.AfterFunc(intv, func() {
				l.mu.Lock()
				defer l.mu.Unlock()
				c := l.checks[id]
				if c == nil || c.Check == nil {
					return
				}
				c.InSync = false
				l.changeMade()
				delete(l.deferCheck, id)
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

// Check returns the locally registered checks that the
// agent is aware of and are being kept in sync with the server
func (l *State) Check(id types.CheckID) *CheckState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if c := l.checks[id]; c != nil && c.Check != nil {
		return c
	}
	return nil
}

// Checks returns the locally registered checks that the
// agent is aware of and are being kept in sync with the server
func (l *State) Checks() map[types.CheckID]*CheckState {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := make(map[types.CheckID]*CheckState)
	for k, v := range l.checks {
		m[k] = v
	}
	return m
}

// Metadata returns the local node metadata fields that the
// agent is aware of and are being kept in sync with the server
func (l *State) Metadata() map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := make(map[string]string)
	for k, v := range l.metadata {
		m[k] = v
	}
	return m
}

// Stats is used to get various debugging state from the sub-systems
func (l *State) Stats() map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	checks := 0
	for _, v := range l.checks {
		if v.Check != nil {
			checks++
		}
	}

	services := 0
	for _, v := range l.services {
		if v.Service != nil {
			services++
		}
	}

	return map[string]string{
		"checks":   strconv.Itoa(checks),
		"services": strconv.Itoa(services),
	}
}

// AntiEntropy is a long running method used to perform anti-entropy
// between local and remote state.
func (l *State) AntiEntropy(shutdownCh chan struct{}) {
SYNC:
	// Sync our state with the servers
	for {
		err := l.setSyncState()
		if err == nil {
			break
		}
		l.logger.Printf("[ERR] agent: failed to sync remote state: %v", err)
		select {
		case <-l.consulCh:
			// Stagger the retry on leader election, avoid a thundering heard
			select {
			case <-time.After(lib.RandomStagger(aeScale(syncStaggerIntv, len(l.delegate.LANMembers())))):
			case <-shutdownCh:
				return
			}
		case <-time.After(syncRetryIntv + lib.RandomStagger(aeScale(syncRetryIntv, len(l.delegate.LANMembers())))):
		case <-shutdownCh:
			return
		}
	}

	// Force-trigger AE to pickup any changes
	l.changeMade()

	// Schedule the next full sync, with a random stagger
	aeIntv := aeScale(l.config.AEInterval, len(l.delegate.LANMembers()))
	aeIntv = aeIntv + lib.RandomStagger(aeIntv)
	aeTimer := time.After(aeIntv)

	// Wait for sync events
	for {
		select {
		case <-aeTimer:
			goto SYNC
		case <-l.triggerCh:
			// Skip the sync if we are paused
			if l.isPaused() {
				continue
			}
			if err := l.SyncChanges(); err != nil {
				l.logger.Printf("[ERR] agent: failed to sync changes: %v", err)
			}
		case <-shutdownCh:
			return
		}
	}
}

// setSyncState does a read of the server state, and updates
// the local syncStatus as appropriate
func (l *State) setSyncState() error {
	req := structs.NodeSpecificRequest{
		Datacenter:   l.config.Datacenter,
		Node:         l.config.NodeName,
		QueryOptions: structs.QueryOptions{Token: l.config.TokenForAgent},
	}

	var out1 structs.IndexedNodeServices
	if err := l.delegate.RPC("Catalog.NodeServices", &req, &out1); err != nil {
		return err
	}

	var out2 structs.IndexedHealthChecks
	if err := l.delegate.RPC("Health.NodeChecks", &req, &out2); err != nil {
		return err
	}
	checks := out2.HealthChecks

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check the node info
	if out1.NodeServices == nil || out1.NodeServices.Node == nil ||
		out1.NodeServices.Node.ID != l.config.NodeID ||
		!reflect.DeepEqual(out1.NodeServices.Node.TaggedAddresses, l.config.TaggedAddresses) ||
		!reflect.DeepEqual(out1.NodeServices.Node.Meta, l.metadata) {
		l.nodeInfoInSync = false
	}

	// Check all our services
	services := make(map[string]*structs.NodeService)
	if out1.NodeServices != nil {
		services = out1.NodeServices.Services
	}

	for id, s := range l.services {
		// If the local service doesn't exist remotely, then sync it
		if _, ok := services[id]; !ok {
			s.InSync = false
		}
	}

	for id, service := range services {
		// If we don't have the service locally, deregister it
		s := l.services[id]
		if s == nil {
			l.services[id] = &ServiceState{}
			continue
		}
		if s.Service == nil {
			continue
		}

		// If our definition is different, we need to update it. Make a
		// copy so that we don't retain a pointer to any actual state
		// store info for in-memory RPCs.
		if s.Service.EnableTagOverride {
			s.Service.Tags = make([]string, len(service.Tags))
			copy(s.Service.Tags, service.Tags)
		}
		s.InSync = s.Service.IsSame(service)
	}

	// Index the remote health checks to improve efficiency
	checkIndex := make(map[types.CheckID]*structs.HealthCheck, len(checks))
	for _, check := range checks {
		checkIndex[check.CheckID] = check
	}

	// Sync any check which doesn't exist on the remote side
	for id, c := range l.checks {
		if _, ok := checkIndex[id]; !ok {
			c.InSync = false
		}
	}

	for _, check := range checks {
		id := check.CheckID

		// If we don't have the check locally, deregister it
		c := l.checks[id]

		// The Serf check is created automatically, and does not
		// need to be registered
		if c == nil && id != rpc.SerfCheckID {
			l.checks[id] = &CheckState{}
			continue
		}

		// If our definition is different, we need to update it
		var equal bool
		if l.config.CheckUpdateInterval == 0 {
			equal = c.Check.IsSame(check)
		} else {
			// Copy the existing check before potentially modifying
			// it before the compare operation.
			eCopy := c.Check.Clone()

			// Copy the server's check before modifying, otherwise
			// in-memory RPCs will have side effects.
			cCopy := check.Clone()

			// If there's a defer timer active then we've got a
			// potentially spammy check so we don't sync the output
			// during this sweep since the timer will mark the check
			// out of sync for us. Otherwise, it is safe to sync the
			// output now. This is especially important for checks
			// that don't change state after they are created, in
			// which case we'd never see their output synced back ever.
			if _, ok := l.deferCheck[id]; ok {
				eCopy.Output = ""
				cCopy.Output = ""
			}
			equal = eCopy.IsSame(cCopy)
		}

		// Update the status
		c.InSync = equal
	}
	return nil
}

// SyncChanges is used to scan the status our local services and checks
// and update any that are out of sync with the server
func (l *State) SyncChanges() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// We will do node-level info syncing at the end, since it will get
	// updated by a service or check sync anyway, given how the register
	// API works.

	// Sync the services
	for id, s := range l.services {
		if s.Service == nil {
			if err := l.deleteService(id); err != nil {
				return err
			}
		} else if !s.InSync {
			if err := l.SyncService(id); err != nil {
				return err
			}
		} else {
			l.logger.Printf("[DEBUG] agent: Service '%s' in sync", id)
		}
	}

	// Sync the checks
	for id, c := range l.checks {
		if c.Check == nil {
			if err := l.deleteCheck(id); err != nil {
				return err
			}
		} else if !c.InSync {
			// Cancel a deferred sync
			if timer := l.deferCheck[id]; timer != nil {
				timer.Stop()
				delete(l.deferCheck, id)
			}

			if err := l.syncCheck(id); err != nil {
				return err
			}
		} else {
			l.logger.Printf("[DEBUG] agent: Check '%s' in sync", id)
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
	} else if strings.Contains(err.Error(), permissionDenied) {
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
		delete(l.checks, id)
		l.logger.Printf("[INFO] agent: Deregistered check '%s'", id)
		return nil
	} else if strings.Contains(err.Error(), permissionDenied) {
		l.checks[id].InSync = true
		l.logger.Printf("[WARN] agent: Check '%s' deregistration blocked by ACLs", id)
		return nil
	}
	return err
}

// syncService is used to sync a service to the server
// The lock must already be held.
func (l *State) SyncService(id string) error {
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

	// If the service has associated checks that are out of sync,
	// piggyback them on the service sync so they are part of the
	// same transaction and are registered atomically. We only let
	// checks ride on service registrations with the same token,
	// otherwise we need to register them separately so they don't
	// pick up privileges from the service token.
	var checks structs.HealthChecks
	for _, c := range l.checks {
		if c.Check.ServiceID == id && (l.serviceToken(id) == l.checkToken(c.Check.CheckID)) {
			if !c.InSync {
				checks = append(checks, c.Check)
			}
		}
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
		l.logger.Printf("[INFO] agent: Synced service '%s'", id)
		for _, check := range checks {
			l.checks[check.CheckID].InSync = true
		}
	} else if strings.Contains(err.Error(), permissionDenied) {
		l.services[id].InSync = true
		l.logger.Printf("[WARN] agent: Service '%s' registration blocked by ACLs", id)
		for _, check := range checks {
			l.checks[check.CheckID].InSync = true
		}
		return nil
	}
	return err
}

// syncCheck is used to sync a check to the server
func (l *State) syncCheck(id types.CheckID) error {
	// Pull in the associated service if any
	c := l.checks[id]
	var service *structs.NodeService
	if c.Check.ServiceID != "" {
		if s := l.services[c.Check.ServiceID]; s != nil {
			service = s.Service
		}
	}

	req := structs.RegisterRequest{
		Datacenter:      l.config.Datacenter,
		ID:              l.config.NodeID,
		Node:            l.config.NodeName,
		Address:         l.config.AdvertiseAddr,
		TaggedAddresses: l.config.TaggedAddresses,
		NodeMeta:        l.metadata,
		Service:         service,
		Check:           l.checks[id].Check,
		WriteRequest:    structs.WriteRequest{Token: l.checkToken(id)},
	}
	var out struct{}
	err := l.delegate.RPC("Catalog.Register", &req, &out)
	if err == nil {
		c.InSync = true
		// Given how the register API works, this info is also updated
		// every time we sync a check.
		l.nodeInfoInSync = true
		l.logger.Printf("[INFO] agent: Synced check '%s'", id)
	} else if strings.Contains(err.Error(), permissionDenied) {
		c.InSync = true
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
		WriteRequest:    structs.WriteRequest{Token: l.config.TokenForAgent},
	}
	var out struct{}
	err := l.delegate.RPC("Catalog.Register", &req, &out)
	if err == nil {
		l.nodeInfoInSync = true
		l.logger.Printf("[INFO] agent: Synced node info")
	} else if strings.Contains(err.Error(), permissionDenied) {
		l.nodeInfoInSync = true
		l.logger.Printf("[WARN] agent: Node info update blocked by ACLs")
		return nil
	}
	return err
}

// loadMetadata loads node metadata fields from the agent config and
// updates them on the local agent.
func (l *State) LoadMetadata(meta map[string]string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for k, v := range meta {
		l.metadata[k] = v
	}
	l.changeMade()
	return nil
}

// UnloadMetadata resets the local metadata state
func (l *State) UnloadMetadata() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.metadata = make(map[string]string)
}

// helper functions for tests

func (l *State) TestSetServiceInSync(id string, inSync bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := l.services[id]
	if s != nil && s.Service != nil {
		s.InSync = inSync
	}
}
