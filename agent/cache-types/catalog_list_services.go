// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cachetype

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// Recommended name for registration.
const CatalogListServicesName = "catalog-list-services"

// CatalogListServices supports fetching discovering service names via the catalog.
type CatalogListServices struct {
	RegisterOptionsBlockingRefresh
	RPC RPC
}

func (c *CatalogListServices) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a DCSpecificRequest.
	reqReal, ok := req.(*structs.DCSpecificRequest)
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
	reqReal.QueryOptions.AllowStale = true

	if opts.LastResult != nil {
		reqReal.QueryOptions.AllowNotModifiedResponse = true
	}

	var reply structs.IndexedServices
	if err := c.RPC.RPC(context.Background(), "Catalog.ListServices", reqReal, &reply); err != nil {
		return result, err
	}

	result.Value = &reply
	result.Index = reply.QueryMeta.Index
	result.NotModified = reply.QueryMeta.NotModified
	return result, nil
}
