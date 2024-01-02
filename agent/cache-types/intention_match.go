// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cachetype

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// Recommended name for registration.
const IntentionMatchName = "intention-match"

// IntentionMatch supports fetching the intentions via match queries.
type IntentionMatch struct {
	RegisterOptionsBlockingRefresh
	RPC RPC
}

func (c *IntentionMatch) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be an IntentionQueryRequest.
	reqReal, ok := req.(*structs.IntentionQueryRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Lightweight copy this object so that manipulating QueryOptions doesn't race.
	dup := *reqReal
	reqReal = &dup

	// Set the minimum query index to our current index so we block
	reqReal.MinQueryIndex = opts.MinIndex
	reqReal.MaxQueryTime = opts.Timeout

	// Fetch
	var reply structs.IndexedIntentionMatches
	if err := c.RPC.RPC(context.Background(), "Intention.Match", reqReal, &reply); err != nil {
		return result, err
	}

	result.Value = &reply
	result.Index = reply.Index
	return result, nil
}
