package local

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
)

var StateCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"acl", "blocked", "service", "registration"},
		Help: "Increments whenever a registration fails for a service (blocked by an ACL)",
	},
	{
		Name: []string{"acl", "blocked", "service", "deregistration"},
		Help: "Increments whenever a deregistration fails for a service (blocked by an ACL)",
	},
	{
		Name: []string{"acl", "blocked", "check", "registration"},
		Help: "Increments whenever a registration fails for a check (blocked by an ACL)",
	},
	{
		Name: []string{"acl", "blocked", "check", "deregistration"},
		Help: "Increments whenever a deregistration fails for a check (blocked by an ACL)",
	},
	{
		Name: []string{"acl", "blocked", "node", "registration"},
		Help: "Increments whenever a registration fails for a node (blocked by an ACL)",
	},
	{
		Name: []string{"acl", "blocked", "node", "deregistration"},
		Help: "Increments whenever a deregistration fails for a node (blocked by an ACL)",
	},
}

const fullSyncReadMaxStale = 2 * time.Second

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

	// WatchCh is closed when the service state changes. Suitable for use in a
	// memdb.WatchSet when watching agent local changes with hash-based blocking.
	WatchCh chan struct{}
}

// Clone returns a shallow copy of the object. The service record still points
// to the original service record and must not be modified. The WatchCh is also
// still pointing to the original so the clone will be update when the original
// is.
func (s *ServiceState) Clone() *ServiceState {
	s2 := new(ServiceState)
	*s2 = *s
	return s2
}

// CheckState describes the state of a health check record.
type CheckState struct {
	// Check is the local copy of the health check record.
	//
	// Must Clone() the overall CheckState before mutating this. After mutation
	// reinstall into the checks map. If Deleted is true, this field can be nil.
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

// Clone returns a shallow copy of the object.
//
// The defer timer still points to the original value and must not be modified.
func (c *CheckState) Clone() *CheckState {
	c2 := new(CheckState)
	*c2 = *c
	if c.Check != nil {
		c2.Check = c.Check.Clone()
	}
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
	ResolveTokenToIdentity(secretID string) (structs.ACLIdentity, error)
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

	logger hclog.Logger

	// Config is the agent config
	config Config

	// nodeInfoInSync tracks whether the server has our correct top-level
	// node information in sync
	nodeInfoInSync bool

	// Services tracks the local services
	services map[structs.ServiceID]*ServiceState

	// Checks tracks the local checks. checkAliases are aliased checks.
	checks       map[structs.CheckID]*CheckState
	checkAliases map[structs.ServiceID]map[structs.CheckID]chan<- struct{}

	// metadata tracks the node metadata fields
	metadata map[string]string

	// discardCheckOutput stores whether the output of health checks
	// is stored in the raft log.
	discardCheckOutput atomic.Value // bool

	// tokens contains the ACL tokens
	tokens *token.Store

	// notifyHandlers is a map of registered channel listeners that are sent
	// messages whenever state changes occur. For now these events only include
	// service registration and deregistration since that is all that is needed
	// but the same mechanism could be used for other state changes. Any
	// future notifications should re-use this mechanism.
	notifyHandlers map[chan<- struct{}]struct{}
}

// NewState creates a new local state for the agent.
func NewState(c Config, logger hclog.Logger, tokens *token.Store) *State {
	l := &State{
		config:         c,
		logger:         logger,
		services:       make(map[structs.ServiceID]*ServiceState),
		checks:         make(map[structs.CheckID]*CheckState),
		checkAliases:   make(map[structs.ServiceID]map[structs.CheckID]chan<- struct{}),
		metadata:       make(map[string]string),
		tokens:         tokens,
		notifyHandlers: make(map[chan<- struct{}]struct{}),
	}
	l.SetDiscardCheckOutput(c.DiscardCheckOutput)
	return l
}

// SetDiscardCheckOutput configures whether the check output
// is discarded. This can be changed at runtime.
func (l *State) SetDiscardCheckOutput(b bool) {
	l.discardCheckOutput.Store(b)
}

// ServiceToken returns the ACL token associated with the service. If the service is
// not found, or does not have a token, the empty string is returned.
func (l *State) ServiceToken(id structs.ServiceID) string {
	l.RLock()
	defer l.RUnlock()
	if s := l.services[id]; s != nil {
		return s.Token
	}
	return ""
}

// aclTokenForServiceSync returns an ACL token associated with a service. If there is
// no ACL token associated with the service, fallback is used to return a value.
// This method is not synchronized and the lock must already be held.
func (l *State) aclTokenForServiceSync(id structs.ServiceID, fallback func() string) string {
	if s := l.services[id]; s != nil && s.Token != "" {
		return s.Token
	}
	return fallback()
}

// AddService is used to add a service entry to the local state.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (l *State) AddService(service *structs.NodeService, token string) error {
	l.Lock()
	defer l.Unlock()
	return l.addServiceLocked(service, token)
}

func (l *State) addServiceLocked(service *structs.NodeService, token string) error {
	if service == nil {
		return fmt.Errorf("no service")
	}

	// use the service name as id if the id was omitted
	if service.ID == "" {
		service.ID = service.Service
	}

	l.setServiceStateLocked(&ServiceState{
		Service: service,
		Token:   token,
	})
	return nil
}

// AddServiceWithChecks adds a service and its check tp the local state atomically
func (l *State) AddServiceWithChecks(service *structs.NodeService, checks []*structs.HealthCheck, token string) error {
	l.Lock()
	defer l.Unlock()

	if err := l.addServiceLocked(service, token); err != nil {
		return err
	}

	for _, check := range checks {
		if err := l.addCheckLocked(check, token); err != nil {
			return err
		}
	}
	return nil
}

// RemoveService is used to remove a service entry from the local state.
// The agent will make a best effort to ensure it is deregistered.
func (l *State) RemoveService(id structs.ServiceID) error {
	l.Lock()
	defer l.Unlock()
	return l.removeServiceLocked(id)
}

// RemoveServiceWithChecks removes a service and its check from the local state atomically
func (l *State) RemoveServiceWithChecks(serviceID structs.ServiceID, checkIDs []structs.CheckID) error {
	l.Lock()
	defer l.Unlock()

	if err := l.removeServiceLocked(serviceID); err != nil {
		return err
	}

	for _, id := range checkIDs {
		if err := l.removeCheckLocked(id); err != nil {
			return err
		}
	}

	return nil
}

func (l *State) removeServiceLocked(id structs.ServiceID) error {
	s := l.services[id]
	if s == nil || s.Deleted {
		return fmt.Errorf("Service %q does not exist", id)
	}

	// To remove the service on the server we need the token.
	// Therefore, we mark the service as deleted and keep the
	// entry around until it is actually removed.
	s.InSync = false
	s.Deleted = true
	if s.WatchCh != nil {
		close(s.WatchCh)
		s.WatchCh = nil
	}

	l.notifyIfAliased(id)
	l.TriggerSyncChanges()
	l.broadcastUpdateLocked()

	return nil
}

// Service returns the locally registered service that the
// agent is aware of and are being kept in sync with the server
func (l *State) Service(id structs.ServiceID) *structs.NodeService {
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
func (l *State) Services(entMeta *structs.EnterpriseMeta) map[structs.ServiceID]*structs.NodeService {
	l.RLock()
	defer l.RUnlock()

	m := make(map[structs.ServiceID]*structs.NodeService)
	for id, s := range l.services {
		if s.Deleted {
			continue
		}

		if !entMeta.Matches(&id.EnterpriseMeta) {
			continue
		}
		m[id] = s.Service
	}
	return m
}

// ServiceState returns a shallow copy of the current service state record. The
// service record still points to the original service record and must not be
// modified. The WatchCh for the copy returned will also be closed when the
// actual service state is changed.
func (l *State) ServiceState(id structs.ServiceID) *ServiceState {
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

	l.setServiceStateLocked(s)
}

func (l *State) setServiceStateLocked(s *ServiceState) {
	key := s.Service.CompoundServiceID()
	old, hasOld := l.services[key]
	if hasOld {
		s.InSync = s.Service.IsSame(old.Service)
	}
	l.services[key] = s

	s.WatchCh = make(chan struct{}, 1)
	if hasOld && old.WatchCh != nil {
		close(old.WatchCh)
	}
	if !hasOld {
		// The status of an alias check is updated if the alias service is added/removed
		// Only try notify alias checks if service didn't already exist (!hasOld)
		l.notifyIfAliased(key)
	}

	l.TriggerSyncChanges()
	l.broadcastUpdateLocked()
}

// ServiceStates returns a shallow copy of all service state records.
// The service record still points to the original service record and
// must not be modified.
func (l *State) ServiceStates(entMeta *structs.EnterpriseMeta) map[structs.ServiceID]*ServiceState {
	l.RLock()
	defer l.RUnlock()

	m := make(map[structs.ServiceID]*ServiceState)
	for id, s := range l.services {
		if s.Deleted {
			continue
		}
		if !entMeta.Matches(&id.EnterpriseMeta) {
			continue
		}
		m[id] = s.Clone()
	}
	return m
}

// CheckToken returns the ACL token associated with the check. If the check is
// not found, or does not have a token, the empty string is returned.
func (l *State) CheckToken(id structs.CheckID) string {
	l.RLock()
	defer l.RUnlock()
	if c := l.checks[id]; c != nil {
		return c.Token
	}
	return ""
}

// aclTokenForCheckSync returns an ACL token associated with a check. If there is
// no ACL token associated with the check, the callback is used to return a value.
// This method is not synchronized and the lock must already be held.
func (l *State) aclTokenForCheckSync(id structs.CheckID, fallback func() string) string {
	if c := l.checks[id]; c != nil && c.Token != "" {
		return c.Token
	}
	return fallback()
}

// AddCheck is used to add a health check to the local state.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (l *State) AddCheck(check *structs.HealthCheck, token string) error {
	l.Lock()
	defer l.Unlock()

	return l.addCheckLocked(check, token)
}

func (l *State) addCheckLocked(check *structs.HealthCheck, token string) error {
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
	if _, ok := l.services[check.CompoundServiceID()]; check.ServiceID != "" && !ok {
		return fmt.Errorf("Check %q refers to non-existent service %q", check.CheckID, check.ServiceID)
	}

	// hard-set the node name
	check.Node = l.config.NodeName

	l.setCheckStateLocked(&CheckState{
		Check: check,
		Token: token,
	})
	return nil
}

// AddAliasCheck creates an alias check. When any check for the srcServiceID is
// changed, checkID will reflect that using the same semantics as
// checks.CheckAlias.
//
// This is a local optimization so that the Alias check doesn't need to use
// blocking queries against the remote server for check updates for local
// services.
func (l *State) AddAliasCheck(checkID structs.CheckID, srcServiceID structs.ServiceID, notifyCh chan<- struct{}) error {
	l.Lock()
	defer l.Unlock()

	m, ok := l.checkAliases[srcServiceID]
	if !ok {
		m = make(map[structs.CheckID]chan<- struct{})
		l.checkAliases[srcServiceID] = m
	}
	m[checkID] = notifyCh

	return nil
}

// ServiceExists return true if the given service does exists
func (l *State) ServiceExists(serviceID structs.ServiceID) bool {
	serviceID.EnterpriseMeta.Normalize()

	l.Lock()
	defer l.Unlock()
	return l.services[serviceID] != nil
}

// RemoveAliasCheck removes the mapping for the alias check.
func (l *State) RemoveAliasCheck(checkID structs.CheckID, srcServiceID structs.ServiceID) {
	l.Lock()
	defer l.Unlock()

	if m, ok := l.checkAliases[srcServiceID]; ok {
		delete(m, checkID)
		if len(m) == 0 {
			delete(l.checkAliases, srcServiceID)
		}
	}
}

// RemoveCheck is used to remove a health check from the local state.
// The agent will make a best effort to ensure it is deregistered
// todo(fs): RemoveService returns an error for a non-existent service. RemoveCheck should as well.
// todo(fs): Check code that calls this to handle the error.
func (l *State) RemoveCheck(id structs.CheckID) error {
	l.Lock()
	defer l.Unlock()
	return l.removeCheckLocked(id)
}

func (l *State) removeCheckLocked(id structs.CheckID) error {
	c := l.checks[id]
	if c == nil || c.Deleted {
		return fmt.Errorf("Check %q does not exist", id)
	}

	// If this is a check for an aliased service, then notify the waiters.
	l.notifyIfAliased(c.Check.CompoundServiceID())

	// To remove the check on the server we need the token.
	// Therefore, we mark the service as deleted and keep the
	// entry around until it is actually removed.
	c.InSync = false
	c.Deleted = true
	l.TriggerSyncChanges()

	return nil
}

// UpdateCheck is used to update the status of a check
func (l *State) UpdateCheck(id structs.CheckID, status, output string) {
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

	// Ensure we only mutate a copy of the check state and put the finalized
	// version into the checks map when complete.
	//
	// Note that we are relying upon the earlier deferred mutex unlock to
	// happen AFTER this defer. As per the Go spec this is true, but leaving
	// this note here for the future in case of any refactorings which may not
	// notice this relationship.
	c = c.Clone()
	defer func(c *CheckState) {
		l.checks[id] = c
	}(c)

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

	// If this is a check for an aliased service, then notify the waiters.
	l.notifyIfAliased(c.Check.CompoundServiceID())

	// Update status and mark out of sync
	c.Check.Status = status
	c.Check.Output = output
	c.InSync = false
	l.TriggerSyncChanges()
}

// Check returns the locally registered check that the
// agent is aware of and are being kept in sync with the server
func (l *State) Check(id structs.CheckID) *structs.HealthCheck {
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
func (l *State) Checks(entMeta *structs.EnterpriseMeta) map[structs.CheckID]*structs.HealthCheck {
	m := make(map[structs.CheckID]*structs.HealthCheck)
	for id, c := range l.CheckStates(entMeta) {
		m[id] = c.Check
	}
	return m
}

func (l *State) ChecksForService(serviceID structs.ServiceID, includeNodeChecks bool) map[structs.CheckID]*structs.HealthCheck {
	m := make(map[structs.CheckID]*structs.HealthCheck)

	l.RLock()
	defer l.RUnlock()

	for id, c := range l.checks {
		if c.Deleted {
			continue
		}

		if c.Check.ServiceID != "" {
			sid := c.Check.CompoundServiceID()
			if !serviceID.Matches(sid) {
				continue
			}
		} else if !includeNodeChecks {
			continue
		}

		m[id] = c.Check.Clone()
	}
	return m
}

// CheckState returns a shallow copy of the current health check state record.
//
// The defer timer still points to the original value and must not be modified.
func (l *State) CheckState(id structs.CheckID) *CheckState {
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

	l.setCheckStateLocked(c)
}

func (l *State) setCheckStateLocked(c *CheckState) {
	id := c.Check.CompoundCheckID()
	existing := l.checks[id]
	if existing != nil {
		c.InSync = c.Check.IsSame(existing.Check)
	}

	l.checks[id] = c

	// If this is a check for an aliased service, then notify the waiters.
	l.notifyIfAliased(c.Check.CompoundServiceID())

	l.TriggerSyncChanges()
}

// CheckStates returns a shallow copy of all health check state records.
// The map contains a shallow copy of the current check states.
//
// The defer timers still point to the original values and must not be modified.
func (l *State) CheckStates(entMeta *structs.EnterpriseMeta) map[structs.CheckID]*CheckState {
	l.RLock()
	defer l.RUnlock()

	m := make(map[structs.CheckID]*CheckState)
	for id, c := range l.checks {
		if c.Deleted {
			continue
		}
		if !entMeta.Matches(&id.EnterpriseMeta) {
			continue
		}
		m[id] = c.Clone()
	}
	return m
}

// CriticalCheckStates returns the locally registered checks that the
// agent is aware of and are being kept in sync with the server.
// The map contains a shallow copy of the current check states.
//
// The defer timers still point to the original values and must not be modified.
func (l *State) CriticalCheckStates(entMeta *structs.EnterpriseMeta) map[structs.CheckID]*CheckState {
	l.RLock()
	defer l.RUnlock()

	m := make(map[structs.CheckID]*CheckState)
	for id, c := range l.checks {
		if c.Deleted || !c.Critical() {
			continue
		}
		if !entMeta.Matches(&id.EnterpriseMeta) {
			continue
		}
		m[id] = c.Clone()
	}
	return m
}

// broadcastUpdateLocked assumes l is locked and delivers an update to all
// registered watchers.
func (l *State) broadcastUpdateLocked() {
	for ch := range l.notifyHandlers {
		// Do not block
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Notify will register a channel to receive messages when the local state
// changes. Only service add/remove are supported for now. See notes on
// l.notifyHandlers for more details.
//
// This will not block on channel send so ensure the channel has a buffer. Note
// that any buffer size is generally fine since actual data is not sent over the
// channel, so a dropped send due to a full buffer does not result in any loss
// of data. The fact that a buffer already contains a notification means that
// the receiver will still be notified that changes occurred.
func (l *State) Notify(ch chan<- struct{}) {
	l.Lock()
	defer l.Unlock()
	l.notifyHandlers[ch] = struct{}{}
}

// StopNotify will deregister a channel receiving state change notifications.
// Pair this with all calls to Notify to clean up state.
func (l *State) StopNotify(ch chan<- struct{}) {
	l.Lock()
	defer l.Unlock()
	delete(l.notifyHandlers, ch)
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

// updateSyncState queries the server for all the services and checks in the catalog
// registered to this node, and updates the local entries as InSync or Deleted.
func (l *State) updateSyncState() error {
	// Get all checks and services from the master
	req := structs.NodeSpecificRequest{
		Datacenter: l.config.Datacenter,
		Node:       l.config.NodeName,
		QueryOptions: structs.QueryOptions{
			Token:            l.tokens.AgentToken(),
			AllowStale:       true,
			MaxStaleDuration: fullSyncReadMaxStale,
		},
		EnterpriseMeta: *structs.WildcardEnterpriseMeta(),
	}

	var out1 structs.IndexedNodeServiceList
	remoteServices := make(map[structs.ServiceID]*structs.NodeService)
	var svcNode *structs.Node

	if err := l.Delegate.RPC("Catalog.NodeServiceList", &req, &out1); err == nil {
		for _, svc := range out1.NodeServices.Services {
			remoteServices[svc.CompoundServiceID()] = svc
		}

		svcNode = out1.NodeServices.Node
	} else if errMsg := err.Error(); strings.Contains(errMsg, "rpc: can't find method") {
		// fallback to the old RPC
		var out1 structs.IndexedNodeServices
		if err := l.Delegate.RPC("Catalog.NodeServices", &req, &out1); err != nil {
			return err
		}

		if out1.NodeServices != nil {
			for _, svc := range out1.NodeServices.Services {
				remoteServices[svc.CompoundServiceID()] = svc
			}

			svcNode = out1.NodeServices.Node
		}
	} else {
		return err
	}

	var out2 structs.IndexedHealthChecks
	if err := l.Delegate.RPC("Health.NodeChecks", &req, &out2); err != nil {
		return err
	}

	remoteChecks := make(map[structs.CheckID]*structs.HealthCheck, len(out2.HealthChecks))
	for _, rc := range out2.HealthChecks {
		remoteChecks[rc.CompoundCheckID()] = rc
	}

	// Traverse all checks, services and the node info to determine
	// which entries need to be updated on or removed from the server

	l.Lock()
	defer l.Unlock()

	// Check if node info needs syncing
	if svcNode == nil || svcNode.ID != l.config.NodeID ||
		!reflect.DeepEqual(svcNode.TaggedAddresses, l.config.TaggedAddresses) ||
		!reflect.DeepEqual(svcNode.Meta, l.metadata) {
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
			if id == structs.ConsulCompoundServiceID {
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
			if id == structs.SerfCompoundCheckID {
				l.logger.Debug("Skipping remote check since it is managed automatically", "check", structs.SerfCheckID)
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
	// we are calling will do that themselves.
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

	// Sync the node level info if we need to.
	if l.nodeInfoInSync {
		l.logger.Debug("Node info in sync")
	} else {
		if err := l.syncNodeInfo(); err != nil {
			return err
		}
	}

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
			l.logger.Debug("Service in sync", "service", id.String())
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
			l.logger.Debug("Check in sync", "check", id.String())
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// deleteService is used to delete a service from the server
func (l *State) deleteService(key structs.ServiceID) error {
	if key.ID == "" {
		return fmt.Errorf("ServiceID missing")
	}

	st := l.aclTokenForServiceSync(key, l.tokens.AgentToken)
	req := structs.DeregisterRequest{
		Datacenter:     l.config.Datacenter,
		Node:           l.config.NodeName,
		ServiceID:      key.ID,
		EnterpriseMeta: key.EnterpriseMeta,
		WriteRequest:   structs.WriteRequest{Token: st},
	}
	var out struct{}
	err := l.Delegate.RPC("Catalog.Deregister", &req, &out)
	switch {
	case err == nil || strings.Contains(err.Error(), "Unknown service"):
		delete(l.services, key)
		// service deregister also deletes associated checks
		for _, c := range l.checks {
			if c.Deleted && c.Check != nil {
				sid := c.Check.CompoundServiceID()
				if sid.Matches(key) {
					l.pruneCheck(c.Check.CompoundCheckID())
				}
			}
		}
		l.logger.Info("Deregistered service", "service", key.ID)
		return nil

	case acl.IsErrPermissionDenied(err), acl.IsErrNotFound(err):
		// todo(fs): mark the service to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.services[key].InSync = true
		accessorID := l.aclAccessorID(st)
		l.logger.Warn("Service deregistration blocked by ACLs", "service", key.String(), "accessorID", accessorID)
		metrics.IncrCounter([]string{"acl", "blocked", "service", "deregistration"}, 1)
		return nil

	default:
		l.logger.Warn("Deregistering service failed.",
			"service", key.String(),
			"error", err,
		)
		return err
	}
}

// deleteCheck is used to delete a check from the server
func (l *State) deleteCheck(key structs.CheckID) error {
	if key.ID == "" {
		return fmt.Errorf("CheckID missing")
	}

	ct := l.aclTokenForCheckSync(key, l.tokens.AgentToken)
	req := structs.DeregisterRequest{
		Datacenter:     l.config.Datacenter,
		Node:           l.config.NodeName,
		CheckID:        key.ID,
		EnterpriseMeta: key.EnterpriseMeta,
		WriteRequest:   structs.WriteRequest{Token: ct},
	}
	var out struct{}
	err := l.Delegate.RPC("Catalog.Deregister", &req, &out)
	switch {
	case err == nil || strings.Contains(err.Error(), "Unknown check"):
		l.pruneCheck(key)
		l.logger.Info("Deregistered check", "check", key.String())
		return nil

	case acl.IsErrPermissionDenied(err), acl.IsErrNotFound(err):
		// todo(fs): mark the check to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.checks[key].InSync = true
		accessorID := l.aclAccessorID(ct)
		l.logger.Warn("Check deregistration blocked by ACLs", "check", key.String(), "accessorID", accessorID)
		metrics.IncrCounter([]string{"acl", "blocked", "check", "deregistration"}, 1)
		return nil

	default:
		l.logger.Warn("Deregistering check failed.",
			"check", key.String(),
			"error", err,
		)
		return err
	}
}

func (l *State) pruneCheck(id structs.CheckID) {
	c := l.checks[id]
	if c != nil && c.DeferCheck != nil {
		c.DeferCheck.Stop()
	}
	delete(l.checks, id)
}

// syncService is used to sync a service to the server
func (l *State) syncService(key structs.ServiceID) error {
	st := l.aclTokenForServiceSync(key, l.tokens.UserToken)

	// If the service has associated checks that are out of sync,
	// piggyback them on the service sync so they are part of the
	// same transaction and are registered atomically. We only let
	// checks ride on service registrations with the same token,
	// otherwise we need to register them separately so they don't
	// pick up privileges from the service token.
	var checks structs.HealthChecks
	for checkKey, c := range l.checks {
		if c.Deleted || c.InSync {
			continue
		}
		if !key.Matches(c.Check.CompoundServiceID()) {
			continue
		}
		if st != l.aclTokenForCheckSync(checkKey, l.tokens.UserToken) {
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
		Service:         l.services[key].Service,
		EnterpriseMeta:  key.EnterpriseMeta,
		WriteRequest:    structs.WriteRequest{Token: st},
		SkipNodeUpdate:  l.nodeInfoInSync,
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
		l.services[key].InSync = true
		// Given how the register API works, this info is also updated
		// every time we sync a service.
		l.nodeInfoInSync = true
		for _, check := range checks {
			checkKey := structs.NewCheckID(check.CheckID, &check.EnterpriseMeta)
			l.checks[checkKey].InSync = true
		}
		l.logger.Info("Synced service", "service", key.String())
		return nil

	case acl.IsErrPermissionDenied(err), acl.IsErrNotFound(err):
		// todo(fs): mark the service and the checks to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.services[key].InSync = true
		for _, check := range checks {
			checkKey := structs.NewCheckID(check.CheckID, &check.EnterpriseMeta)
			l.checks[checkKey].InSync = true
		}
		accessorID := l.aclAccessorID(st)
		l.logger.Warn("Service registration blocked by ACLs", "service", key.String(), "accessorID", accessorID)
		metrics.IncrCounter([]string{"acl", "blocked", "service", "registration"}, 1)
		return nil

	default:
		l.logger.Warn("Syncing service failed.",
			"service", key.String(),
			"error", err,
		)
		return err
	}
}

// syncCheck is used to sync a check to the server
func (l *State) syncCheck(key structs.CheckID) error {
	c := l.checks[key]
	ct := l.aclTokenForCheckSync(key, l.tokens.UserToken)
	req := structs.RegisterRequest{
		Datacenter:      l.config.Datacenter,
		ID:              l.config.NodeID,
		Node:            l.config.NodeName,
		Address:         l.config.AdvertiseAddr,
		TaggedAddresses: l.config.TaggedAddresses,
		NodeMeta:        l.metadata,
		Check:           c.Check,
		EnterpriseMeta:  c.Check.EnterpriseMeta,
		WriteRequest:    structs.WriteRequest{Token: ct},
		SkipNodeUpdate:  l.nodeInfoInSync,
	}

	serviceKey := structs.NewServiceID(c.Check.ServiceID, &key.EnterpriseMeta)

	// Pull in the associated service if any
	s := l.services[serviceKey]
	if s != nil && !s.Deleted {
		req.Service = s.Service
	}

	var out struct{}
	err := l.Delegate.RPC("Catalog.Register", &req, &out)
	switch {
	case err == nil:
		l.checks[key].InSync = true
		// Given how the register API works, this info is also updated
		// every time we sync a check.
		l.nodeInfoInSync = true
		l.logger.Info("Synced check", "check", key.String())
		return nil

	case acl.IsErrPermissionDenied(err), acl.IsErrNotFound(err):
		// todo(fs): mark the check to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.checks[key].InSync = true
		accessorID := l.aclAccessorID(ct)
		l.logger.Warn("Check registration blocked by ACLs", "check", key.String(), "accessorID", accessorID)
		metrics.IncrCounter([]string{"acl", "blocked", "check", "registration"}, 1)
		return nil

	default:
		l.logger.Warn("Syncing check failed.",
			"check", key.String(),
			"error", err,
		)
		return err
	}
}

func (l *State) syncNodeInfo() error {
	at := l.tokens.AgentToken()
	req := structs.RegisterRequest{
		Datacenter:      l.config.Datacenter,
		ID:              l.config.NodeID,
		Node:            l.config.NodeName,
		Address:         l.config.AdvertiseAddr,
		TaggedAddresses: l.config.TaggedAddresses,
		NodeMeta:        l.metadata,
		WriteRequest:    structs.WriteRequest{Token: at},
	}
	var out struct{}
	err := l.Delegate.RPC("Catalog.Register", &req, &out)
	switch {
	case err == nil:
		l.nodeInfoInSync = true
		l.logger.Info("Synced node info")
		return nil

	case acl.IsErrPermissionDenied(err), acl.IsErrNotFound(err):
		// todo(fs): mark the node info to be in sync to prevent excessive retrying before next full sync
		// todo(fs): some backoff strategy might be a better solution
		l.nodeInfoInSync = true
		accessorID := l.aclAccessorID(at)
		l.logger.Warn("Node info update blocked by ACLs", "node", l.config.NodeID, "accessorID", accessorID)
		metrics.IncrCounter([]string{"acl", "blocked", "node", "registration"}, 1)
		return nil

	default:
		l.logger.Warn("Syncing node info failed.", "error", err)
		return err
	}
}

// notifyIfAliased will notify waiters of changes to an aliased service
func (l *State) notifyIfAliased(serviceID structs.ServiceID) {
	if aliases, ok := l.checkAliases[serviceID]; ok && len(aliases) > 0 {
		for _, notifyCh := range aliases {
			// Do not block. All notify channels should be buffered to at
			// least 1 in which case not-blocking does not result in loss
			// of data because a failed send means a notification is
			// already queued. This must be called with the lock held.
			select {
			case notifyCh <- struct{}{}:
			default:
			}
		}
	}
}

// aclAccessorID is used to convert an ACLToken's secretID to its accessorID for non-
// critical purposes, such as logging. Therefore we interpret all errors as empty-string
// so we can safely log it without handling non-critical errors at the usage site.
func (l *State) aclAccessorID(secretID string) string {
	ident, err := l.Delegate.ResolveTokenToIdentity(secretID)
	if acl.IsErrNotFound(err) {
		return ""
	}
	if err != nil {
		l.logger.Debug("non-critical error resolving acl token accessor for logging", "error", err)
		return ""
	}
	if ident == nil {
		return ""
	}
	return ident.ID()
}
