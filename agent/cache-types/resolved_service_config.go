package cachetype

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// Recommended name for registration.
const ResolvedServiceConfigName = "resolved-service-config"

// ResolvedServiceConfig supports fetching the config for a service resolved from
// the global proxy defaults and the centrally registered service config.
type ResolvedServiceConfig struct {
	RegisterOptionsBlockingRefresh
	RPC RPC
}

func (c *ResolvedServiceConfig) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a ServiceConfigRequest.
	reqReal, ok := req.(*structs.ServiceConfigRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Lightweight copy this object so that manipulating QueryOptions doesn't race.
	dup := *reqReal
	reqReal = &dup

	// Set the minimum query index to our current index so we block
	reqReal.QueryOptions.MinQueryIndex = opts.MinIndex
	reqReal.QueryOptions.MaxQueryTime = opts.Timeout

	// Always allow stale - there's no point in hitting leader if the request is
	// going to be served from cache and endup arbitrarily stale anyway. This
	// allows cached resolved-service-config to automatically read scale across all
	// servers too.
	reqReal.AllowStale = true

	// Fetch
	var reply structs.ServiceConfigResponse
	if err := c.RPC.RPC(context.Background(), "ConfigEntry.ResolveServiceConfig", reqReal, &reply); err != nil {
		return result, err
	}

	result.Value = &reply
	result.Index = reply.QueryMeta.Index
	return result, nil
}
