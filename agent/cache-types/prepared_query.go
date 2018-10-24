package cachetype

import (
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// Recommended name for registration.
const PreparedQueryName = "prepared-query"

// PreparedQuery supports fetching discovering service instances via prepared
// queries.
type PreparedQuery struct {
	RPC RPC
}

func (c *PreparedQuery) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a PreparedQueryExecuteRequest.
	reqReal, ok := req.(*structs.PreparedQueryExecuteRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Allways allow stale - there's no point in hitting leader if the request is
	// going to be served from cache and endup arbitrarily stale anyway. This
	// allows cached service-discover to automatically read scale across all
	// servers too.
	reqReal.AllowStale = true

	// Fetch
	var reply structs.PreparedQueryExecuteResponse
	if err := c.RPC.RPC("PreparedQuery.Execute", reqReal, &reply); err != nil {
		return result, err
	}

	result.Value = &reply
	result.Index = reply.QueryMeta.Index

	return result, nil
}

func (c *PreparedQuery) SupportsBlocking() bool {
	// Prepared queries don't support blocking.
	return false
}
