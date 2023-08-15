// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package checks

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// Constants related to alias check backoff.
const (
	checkAliasBackoffMin     = 3               // 3 attempts before backing off
	checkAliasBackoffMaxWait = 1 * time.Minute // maximum backoff wait time
)

// CheckAlias is a check type that aliases the health of another service
// instance or node. If the service aliased has any critical health checks, then
// this check is critical. If the service has no critical but warnings,
// then this check is warning, and if a service has only passing checks, then
// this check is passing.
type CheckAlias struct {
	Node      string            // Node name of the service. If empty, assumed to be this node.
	ServiceID structs.ServiceID // ID (not name) of the service to alias

	CheckID            structs.CheckID             // ID of this check
	RPC                RPC                         // Used to query remote server if necessary
	RPCReq             structs.NodeSpecificRequest // Base request
	Notify             AliasNotifier               // For updating the check state
	LastCheckStartTime time.Time

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
	stopWg   sync.WaitGroup

	acl.EnterpriseMeta
}

// AliasNotifier is a CheckNotifier specifically for the Alias check.
// This requires additional methods that are satisfied by the agent
// local state.
type AliasNotifier interface {
	CheckNotifier

	AddAliasCheck(structs.CheckID, structs.ServiceID, chan<- struct{}) error
	RemoveAliasCheck(structs.CheckID, structs.ServiceID)
	Checks(*acl.EnterpriseMeta) map[structs.CheckID]*structs.HealthCheck
}

// Start is used to start the check, runs until Stop() func (c *CheckAlias) Start() {
func (c *CheckAlias) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	c.stop = false
	c.stopCh = make(chan struct{})
	c.stopWg.Add(1)
	go c.run(c.stopCh)
}

// Stop is used to stop the check.
func (c *CheckAlias) Stop() {
	c.stopLock.Lock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}
	c.stopLock.Unlock()

	// Wait until the associated goroutine is definitely complete before
	// returning to the caller. This is to prevent the new and old checks from
	// both updating the state of the alias check using possibly stale
	// information.
	c.stopWg.Wait()
}

// run is invoked in a goroutine until Stop() is called.
func (c *CheckAlias) run(stopCh chan struct{}) {
	defer c.stopWg.Done()

	c.LastCheckStartTime = time.Now()
	// If we have a specific node set, then use a blocking query
	if c.Node != "" {
		c.runQuery(stopCh)
		return
	}

	// Use the local state to match the service.
	c.runLocal(stopCh)
}

func (c *CheckAlias) runLocal(stopCh chan struct{}) {
	// Very important this is buffered as 1 so that we do not lose any
	// queued updates. This only has to be exactly 1 since the existence
	// of any update triggers us to load the full health check state.
	notifyCh := make(chan struct{}, 1)
	c.Notify.AddAliasCheck(c.CheckID, c.ServiceID, notifyCh)
	defer c.Notify.RemoveAliasCheck(c.CheckID, c.ServiceID)

	// maxDurationBetweenUpdates is maximum time we go between explicit
	// notifications before we re-query the aliased service checks anyway. This
	// helps in the case we miss an edge triggered event and the alias does not
	// accurately reflect the underlying service health status.
	const maxDurationBetweenUpdates = 1 * time.Minute

	var refreshTimer <-chan time.Time
	extendRefreshTimer := func() {
		refreshTimer = time.After(maxDurationBetweenUpdates)
	}

	updateStatus := func() {
		checks := c.Notify.Checks(c.WithWildcardNamespace())
		checksList := make([]*structs.HealthCheck, 0, len(checks))
		for _, chk := range checks {
			checksList = append(checksList, chk)
		}
		c.processChecks(checksList, func(serviceID *structs.ServiceID) bool {
			return c.Notify.ServiceExists(*serviceID)
		})
		extendRefreshTimer()
	}

	// Immediately run to get the current state of the target service
	updateStatus()

	for {
		select {
		case <-refreshTimer:
			updateStatus()
		case <-notifyCh:
			updateStatus()
		case <-stopCh:
			return
		}
	}
}

// CheckIfServiceIDExists is used to determine if a service exists
type CheckIfServiceIDExists func(*structs.ServiceID) bool

func (c *CheckAlias) checkServiceExistsOnRemoteServer(serviceID *structs.ServiceID) (bool, error) {
	args := c.RPCReq
	args.Node = c.Node
	args.AllowStale = true
	args.EnterpriseMeta = c.EnterpriseMeta
	// We are late at maximum of 15s compared to leader
	args.MaxStaleDuration = 15 * time.Second
	attempts := 0
RETRY_CALL:
	var out structs.IndexedNodeServices
	attempts++
	if err := c.RPC.RPC(context.Background(), "Catalog.NodeServices", &args, &out); err != nil {
		if attempts <= 3 {
			time.Sleep(time.Duration(attempts) * time.Second)
			goto RETRY_CALL
		}
		return false, err
	}
	for _, srv := range out.NodeServices.Services {
		if serviceID.Matches(srv.CompoundServiceID()) {
			return true, nil
		}
	}
	return false, nil
}

func (c *CheckAlias) runQuery(stopCh chan struct{}) {
	args := c.RPCReq
	args.Node = c.Node
	args.AllowStale = true
	args.MaxQueryTime = 1 * time.Minute
	args.EnterpriseMeta = c.EnterpriseMeta
	// We are late at maximum of 15s compared to leader
	args.MaxStaleDuration = 15 * time.Second

	var attempt uint
	for {
		// Check if we're stopped. We fallthrough and block otherwise,
		// which has a maximum time set above so we'll always check for
		// stop within a reasonable amount of time.
		select {
		case <-stopCh:
			return
		default:
		}

		// Backoff if we have to
		if attempt > checkAliasBackoffMin {
			shift := attempt - checkAliasBackoffMin
			if shift > 31 {
				shift = 31 // so we don't overflow to 0
			}
			waitTime := (1 << shift) * time.Second
			if waitTime > checkAliasBackoffMaxWait {
				waitTime = checkAliasBackoffMaxWait
			}
			time.Sleep(waitTime)
		}

		// Get the current health checks for the specified node.
		//
		// NOTE(mitchellh): This currently returns ALL health checks for
		// a node even though we also have the service ID. This can be
		// optimized if we introduce a new RPC endpoint to filter both,
		// but for blocking queries isn't that much more efficient since the checks
		// index is global to the cluster.
		var out structs.IndexedHealthChecks

		if err := c.RPC.RPC(context.Background(), "Health.NodeChecks", &args, &out); err != nil {
			attempt++
			if attempt > 1 {
				c.Notify.UpdateCheck(c.CheckID, api.HealthCritical,
					fmt.Sprintf("Failure checking aliased node or service: %s", err))
			}

			continue
		}

		attempt = 0 // Reset the attempts so we don't backoff the next

		// Set our index for the next request
		args.MinQueryIndex = out.Index

		// We want to ensure that we're always blocking on subsequent requests
		// to avoid hot loops. Index 1 is always safe since the min raft index
		// is at least 5. Note this shouldn't happen but protecting against this
		// case is safer than a 100% CPU loop.
		if args.MinQueryIndex < 1 {
			args.MinQueryIndex = 1
		}
		c.processChecks(out.HealthChecks, func(serviceID *structs.ServiceID) bool {
			ret, err := c.checkServiceExistsOnRemoteServer(serviceID)
			if err != nil {
				// We cannot determine if node has the check, let's assume it exists
				return true
			}
			return ret
		})
	}
}

// processChecks is a common helper for taking a set of health checks and
// using them to update our alias. This is abstracted since the checks can
// come from both the remote server as well as local state.
func (c *CheckAlias) processChecks(checks []*structs.HealthCheck, CheckIfServiceIDExists CheckIfServiceIDExists) {
	health := api.HealthPassing
	msg := "No checks found."
	serviceFound := false
	for _, chk := range checks {
		if c.Node != "" && !strings.EqualFold(c.Node, chk.Node) {
			continue
		}
		serviceMatch := c.ServiceID.Matches(chk.CompoundServiceID())
		if chk.ServiceID != "" && !serviceMatch {
			continue
		}
		// We have at least one healthcheck for this service
		if serviceMatch {
			serviceFound = true
		}
		if chk.Status == api.HealthCritical || chk.Status == api.HealthWarning {
			health = chk.Status
			msg = fmt.Sprintf("Aliased check %q failing: %s", chk.Name, chk.Output)

			// Critical checks exit the for loop immediately since we
			// know that this is the health state. Warnings do not since
			// there may still be a critical check.
			if chk.Status == api.HealthCritical {
				break
			}
		} else {
			// if current health is warning, don't overwrite it
			if health == api.HealthPassing {
				msg = "All checks passing."
			}
		}
	}
	if !serviceFound {
		if !CheckIfServiceIDExists(&c.ServiceID) {
			msg = fmt.Sprintf("Service %s could not be found on node %s", c.ServiceID.ID, c.Node)
			health = api.HealthCritical
		}
	}
	c.Notify.UpdateCheck(c.CheckID, health, msg)
}
