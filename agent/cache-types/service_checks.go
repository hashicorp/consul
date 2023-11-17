package cachetype

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
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
	RegisterOptionsBlockingRefresh
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
			sid := structs.NewServiceID(reqReal.ServiceID, &reqReal.EnterpriseMeta)
			svcState := c.Agent.LocalState().ServiceState(sid)
			if svcState == nil {
				return "", nil, fmt.Errorf("Internal cache failure: service '%s' not in agent state", reqReal.ServiceID)
			}

			// WatchCh will receive updates on service (de)registrations and check (de)registrations
			ws.Add(svcState.WatchCh)

			reply := c.Agent.ServiceHTTPBasedChecks(sid)

			hash, err := hashChecks(reply)
			if err != nil {
				return "", nil, fmt.Errorf("Internal cache failure: %v", err)
			}

			return hash, reply, nil
		},
	)
	if err != nil {
		return result, err
	}

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

// ServiceHTTPChecksRequest is the cache.Request implementation for the
// ServiceHTTPBasedChecks cache type. This is implemented here and not in structs
// since this is only used for cache-related requests and not forwarded
// directly to any Consul servers.
type ServiceHTTPChecksRequest struct {
	ServiceID     string
	NodeName      string
	MinQueryIndex uint64
	MaxQueryTime  time.Duration
	acl.EnterpriseMeta
}

func (s *ServiceHTTPChecksRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:      "",
		Datacenter: "",
		MinIndex:   s.MinQueryIndex,
		Timeout:    s.MaxQueryTime,
	}

	v, err := hashstructure.Hash([]interface{}{
		s.ServiceID,
		s.EnterpriseMeta,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
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
