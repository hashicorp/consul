package cachetype

import (
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// Recommended name for registration.
const IntentionMatchName = "intention-match"

// IntentionMatch supports fetching the intentions via match queries.
type IntentionMatch struct {
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

	// Set the minimum query index to our current index so we block
	reqReal.MinQueryIndex = opts.MinIndex
	reqReal.MaxQueryTime = opts.Timeout

	// Fetch
	var reply structs.IndexedIntentionMatches
	if err := c.RPC.RPC("Intention.Match", reqReal, &reply); err != nil {
		return result, err
	}

	result.Value = &reply
	result.Index = reply.Index
	return result, nil
}
