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
const CatalogDatacentersName = "catalog-datacenters"

// Datacenters supports fetching discovering all the known datacenters
type CatalogDatacenters struct {
	RegisterOptionsNoRefresh
	RPC RPC
}

func (c *CatalogDatacenters) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a CatalogDatacentersRequest.
	reqReal, ok := req.(*structs.DatacentersRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Lightweight copy this object so that manipulating QueryOptions doesn't race.
	dup := *reqReal
	reqReal = &dup

	// Always allow stale - there's no point in hitting leader if the request is
	// going to be served from cache and endup arbitrarily stale anyway. This
	// allows cached service-discover to automatically read scale across all
	// servers too.
	reqReal.AllowStale = true

	// Fetch
	var reply []string
	if err := c.RPC.RPC(context.Background(), "Catalog.ListDatacenters", reqReal, &reply); err != nil {
		return result, err
	}

	result.Value = &reply

	// this is a purely synthetic index to keep the caching happy.
	if opts.LastResult != nil {
		equal := true
		previousDCs, ok := opts.LastResult.Value.(*[]string)
		if ok && previousDCs == nil {
			ok = false
		}

		if ok {
			if len(reply) != len(*previousDCs) {
				equal = false
			} else {
				// ordering matters as they should be sorted based on distance
				for i, dc := range reply {
					if dc != (*previousDCs)[i] {
						equal = false
						break
					}
				}
			}
		}

		result.Index = opts.LastResult.Index
		if !equal || !ok {
			result.Index += 1
		}
	} else {
		result.Index = 1
	}

	return result, nil
}
