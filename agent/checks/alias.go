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
// instance. If the service aliased has any critical health checks, then
// this check is critical. If the service has no critical but warnings,
// then this check is warning, and if a service has only passing checks, then
// this check is passing.
type CheckAlias struct {
	Node      string // Node name of the service. If empty, assumed to be this node.
	ServiceID string // ID (not name) of the service to alias

	CheckID types.CheckID               // ID of this check
	RPC     RPC                         // Used to query remote server if necessary
	RPCReq  structs.NodeSpecificRequest // Base request
	Notify  CheckNotifier               // For updating the check state

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
}

// Start is used to start a check ttl, runs until Stop() func (c *CheckAlias) Start() {
func (c *CheckAlias) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	c.stop = false
	c.stopCh = make(chan struct{})
	go c.run(c.stopCh)
}

// Stop is used to stop a check ttl.
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
			waitTime := (1 << (attempt - checkAliasBackoffMin)) * time.Second
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
		// but for blocking queries isn't that more efficient since the checks
		// index is global to the cluster.
		var out structs.IndexedHealthChecks
		if err := c.RPC.RPC("Health.NodeChecks", &args, &out); err != nil {
			attempt++
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

		health := api.HealthPassing
		msg := "All checks passing."
		if len(out.HealthChecks) == 0 {
			// No health checks means we're healthy by default
			msg = "No checks found."
		}
		for _, chk := range out.HealthChecks {
			if chk.Node != c.Node {
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
		}

		// Update our check value
		c.Notify.UpdateCheck(c.CheckID, health, msg)
	}
}
