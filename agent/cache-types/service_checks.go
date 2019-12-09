package cachetype

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
	"github.com/mitchellh/hashstructure"
)

// Recommended name for registration.
const ServiceHTTPChecksName = "service-http-checks"

type Agent interface {
	ServiceHTTPBasedChecks(id structs.ServiceID) []structs.CheckType
	LocalState() *local.State
	LocalBlockingQuery(alwaysBlock bool, hash string, wait time.Duration,
		fn func(ws memdb.WatchSet) (string, interface{}, error)) (string, interface{}, error)
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

	var lastChecks []structs.CheckType
	var lastHash string
	var err error

	// Hash last known result as a baseline
	if opts.LastResult != nil {
		lastChecks, ok = opts.LastResult.Value.([]structs.CheckType)
		if !ok {
			return result, fmt.Errorf(
				"Internal cache failure: last value in cache of wrong type: %T, want: CheckType", req)
		}
		lastHash, err = hashChecks(lastChecks)
		if err != nil {
			return result, fmt.Errorf("Internal cache failure: %v", err)
		}
	}

	hash, resp, err := c.Agent.LocalBlockingQuery(true, lastHash, reqReal.MaxQueryTime,
		func(ws memdb.WatchSet) (string, interface{}, error) {
			// TODO (namespaces) update with the real ent meta once thats plumbed through
			svcState := c.Agent.LocalState().ServiceState(structs.NewServiceID(reqReal.ServiceID, nil))
			if svcState == nil {
				return "", result, fmt.Errorf("Internal cache failure: service '%s' not in agent state", reqReal.ServiceID)
			}

			// WatchCh will receive updates on service (de)registrations and check (de)registrations
			ws.Add(svcState.WatchCh)

			// TODO (namespaces) update with a real entMeta
			reply := c.Agent.ServiceHTTPBasedChecks(structs.NewServiceID(reqReal.ServiceID, nil))

			hash, err := hashChecks(reply)
			if err != nil {
				return "", result, fmt.Errorf("Internal cache failure: %v", err)
			}

			return hash, reply, nil
		},
	)

	result.Value = resp

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
	if len(checks) == 0 {
		return "", nil
	}

	// Wrapper created to use "set" struct tag, that way ordering doesn't lead to false-positives
	wrapper := struct {
		ChkTypes []structs.CheckType `hash:"set"`
	}{
		ChkTypes: checks,
	}

	b, err := hashstructure.Hash(wrapper, nil)
	if err != nil {
		return "", fmt.Errorf("failed to hash checks: %v", err)
	}
	return fmt.Sprintf("%d", b), nil
}
