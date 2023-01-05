package cachetype

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// Recommended name for registration.
const ServiceGatewaysName = "service-gateways"

// GatewayUpstreams supports fetching upstreams for a given gateway name.
type ServiceGateways struct {
	RegisterOptionsBlockingRefresh
	RPC RPC
}

func (g *ServiceGateways) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a ServiceSpecificRequest.
	reqReal, ok := req.(*structs.ServiceSpecificRequest)
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
	// going to be served from cache and end up arbitrarily stale anyway. This
	// allows cached service-discover to automatically read scale across all
	// servers too.
	reqReal.AllowStale = true

	// Fetch
	var reply structs.IndexedCheckServiceNodes
	if err := g.RPC.RPC(context.Background(), "Internal.ServiceGateways", reqReal, &reply); err != nil {
		return result, err
	}

	result.Value = &reply
	result.Index = reply.QueryMeta.Index
	return result, nil
}
