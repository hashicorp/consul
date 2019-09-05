package cachetype

import (
	"fmt"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-memdb"
	"github.com/mitchellh/hashstructure"
	"strings"
	"time"
)

// Recommended name for registration.
const ServiceHTTPChecksName = "service-http-checks"

type Agent interface {
	ServiceHTTPBasedChecks(id string) []structs.CheckType
	LocalState() *local.State
	SyncPausedCh() <-chan struct{}
}

// ServiceHTTPBasedChecks supports fetching discovering checks in the local state
type ServiceHTTPChecks struct {
	Agent Agent
}

func (c *ServiceHTTPChecks) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a CatalogDatacentersRequest.
	reqReal, ok := req.(*ServiceHTTPChecksRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: got wrong request type: %T, want: ServiceHTTPChecksRequest", req)
	}

	var lastChecks *[]structs.CheckType
	var lastHash string
	var err error

	// Hash last known result as a baseline
	if opts.LastResult != nil {
		lastChecks, ok = opts.LastResult.Value.(*[]structs.CheckType)
		if !ok {
			return result, fmt.Errorf(
				"Internal cache failure: got wrong request type: %T, want: ServiceHTTPChecksRequest", req)
		}
		lastHash, err = hashChecks(*lastChecks)
		if err != nil {
			return result, fmt.Errorf("Internal cache failure: %v", err)
		}
	}

	var wait time.Duration

	// Adjust wait based on documented limits and add some jitter: https://www.consul.io/api/features/blocking.html
	switch wait = reqReal.MaxQueryTime; {
	case wait == 0*time.Second:
		wait = 5 * time.Minute
	case wait > 10*time.Minute:
		wait = 10 * time.Minute
	}
	timeout := time.NewTimer(wait + lib.RandomStagger(wait/16))

	var resp []structs.CheckType
	var hash string

WATCH_LOOP:
	for {
		// Must reset this every loop in case the Watch set is already closed but
		// hash remains same. In that case we'll need to re-block on ws.Watch()
		ws := memdb.NewWatchSet()

		svcState := c.Agent.LocalState().ServiceState(reqReal.ServiceID)
		if svcState == nil {
			return result, fmt.Errorf("Internal cache failure: service '%s' not in agent state", reqReal.ServiceID)
		}

		// WatchCh will receive updates on service (de)registrations and check (de)registrations
		ws.Add(svcState.WatchCh)

		resp = c.Agent.ServiceHTTPBasedChecks(reqReal.ServiceID)

		hash, err = hashChecks(resp)
		if err != nil {
			return result, fmt.Errorf("Internal cache failure: %v", err)
		}

		// Return immediately if the hash is different or the Watch returns true (indicating timeout fired).
		if lastHash != hash || ws.Watch(timeout.C) {
			break
		}

		// Watch returned false indicating a change was detected, loop and repeat
		// the call to ServiceHTTPBasedChecks to load the new value.
		// If agent sync is paused it means local state is being bulk-edited e.g. config reload.
		if syncPauseCh := c.Agent.SyncPausedCh(); syncPauseCh != nil {
			// Wait for pause to end or for the timeout to elapse.
			select {
			case <-syncPauseCh:
			case <-timeout.C:
				break WATCH_LOOP
			}
		}
	}

	result.Value = &resp

	// Below is a purely synthetic index to keep the caching happy.
	if opts.LastResult == nil {
		result.Index = 1
		return result, nil
	}

	result.Index = opts.LastResult.Index
	if lastHash == "" || hash != lastHash {
		result.Index += 1
	}
	return result, nil
}

func (c *ServiceHTTPChecks) SupportsBlocking() bool {
	return true
}

// ServiceHTTPChecksRequest is the cache.Request implementation for the
// ServiceHTTPBasedChecks cache type. This is implemented here and not in structs
// since this is only used for cache-related requests and not forwarded
// directly to any Consul servers.
type ServiceHTTPChecksRequest struct {
	ServiceID     string
	MinQueryIndex uint64
	MaxQueryTime  time.Duration
}

func (s *ServiceHTTPChecksRequest) CacheInfo() cache.RequestInfo {
	return cache.RequestInfo{
		Token:      "",
		Key:        ServiceHTTPChecksName + ":" + s.ServiceID,
		Datacenter: "",
		MinIndex:   s.MinQueryIndex,
		Timeout:    s.MaxQueryTime,
	}
}

func hashChecks(checks []structs.CheckType) (string, error) {
	var b strings.Builder
	for _, check := range checks {
		raw, err := hashstructure.Hash(check, nil)
		if err != nil {
			return "", fmt.Errorf("failed to hash check '%s': %v", check.CheckID, err)
		}
		fmt.Fprintf(&b, "%x", raw)
	}
	return b.String(), nil
}
