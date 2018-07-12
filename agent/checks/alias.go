package checks

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
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
	Node      string // Node name of the service. If empty, assumed to be this node.
	ServiceID string // ID (not name) of the service to alias

	CheckID types.CheckID               // ID of this check
	RPC     RPC                         // Used to query remote server if necessary
	RPCReq  structs.NodeSpecificRequest // Base request
	Notify  AliasNotifier               // For updating the check state

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
}

// AliasNotifier is a CheckNotifier specifically for the Alias check.
// This requires additional methods that are satisfied by the agent
// local state.
type AliasNotifier interface {
	CheckNotifier

	AddAliasCheck(types.CheckID, string, chan<- struct{}) error
	RemoveAliasCheck(types.CheckID, string)
	Checks() map[types.CheckID]*structs.HealthCheck
}

// Start is used to start the check, runs until Stop() func (c *CheckAlias) Start() {
func (c *CheckAlias) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	c.stop = false
	c.stopCh = make(chan struct{})
	go c.run(c.stopCh)
}

// Stop is used to stop the check.
func (c *CheckAlias) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}
}

// run is invoked in a goroutine until Stop() is called.
func (c *CheckAlias) run(stopCh chan struct{}) {
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

	for {
		select {
		case <-notifyCh:
			checks := c.Notify.Checks()
			checksList := make([]*structs.HealthCheck, 0, len(checks))
			for _, chk := range checks {
				checksList = append(checksList, chk)
			}
			c.processChecks(checksList)

		case <-stopCh:
			return
		}
	}
}

func (c *CheckAlias) runQuery(stopCh chan struct{}) {
	args := c.RPCReq
	args.Node = c.Node
	args.AllowStale = true
	args.MaxQueryTime = 1 * time.Minute

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
		if err := c.RPC.RPC("Health.NodeChecks", &args, &out); err != nil {
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

		c.processChecks(out.HealthChecks)
	}
}

// processChecks is a common helper for taking a set of health checks and
// using them to update our alias. This is abstracted since the checks can
// come from both the remote server as well as local state.
func (c *CheckAlias) processChecks(checks []*structs.HealthCheck) {
	health := api.HealthPassing
	msg := "No checks found."
	for _, chk := range checks {
		if c.Node != "" && chk.Node != c.Node {
			continue
		}

		// We allow ServiceID == "" so that we also check node checks
		if chk.ServiceID != "" && chk.ServiceID != c.ServiceID {
			continue
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
		}

		msg = "All checks passing."
	}

	// Update our check value
	c.Notify.UpdateCheck(c.CheckID, health, msg)
}
