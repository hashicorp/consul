package cache

import (
	"time"
)

// Type implement the logic to fetch certain types of data.
type Type interface {
	// Fetch fetches a single unique item.
	//
	// The FetchOptions contain the index and timeouts for blocking queries.
	// The CacheMinIndex value on the Request itself should NOT be used
	// as the blocking index since a request may be reused multiple times
	// as part of Refresh behavior.
	//
	// The return value is a FetchResult which contains information about
	// the fetch.
	Fetch(FetchOptions, Request) (FetchResult, error)
}

// FetchOptions are various settable options when a Fetch is called.
type FetchOptions struct {
	// RPC is the RPC client to communicate to a Consul server.
	RPC RPC

	// MinIndex is the minimum index to be used for blocking queries.
	// If blocking queries aren't supported for data being returned,
	// this value can be ignored.
	MinIndex uint64

	// Timeout is the maximum time for the query. This must be implemented
	// in the Fetch itself.
	Timeout time.Duration
}

// FetchResult is the result of a Type Fetch operation and contains the
// data along with metadata gathered from that operation.
type FetchResult struct {
	// Value is the result of the fetch.
	Value interface{}

	// Index is the corresponding index value for this data.
	Index uint64
}

/*
type TypeCARoot struct{}

func (c *TypeCARoot) Fetch(delegate RPC, idx uint64, req Request) (interface{}, uint64, error) {
	// The request should be a DCSpecificRequest.
	reqReal, ok := req.(*structs.DCSpecificRequest)
	if !ok {
		return nil, 0, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Set the minimum query index to our current index so we block
	reqReal.QueryOptions.MinQueryIndex = idx

	// Fetch
	var reply structs.IndexedCARoots
	if err := delegate.RPC("ConnectCA.Roots", reqReal, &reply); err != nil {
		return nil, 0, err
	}

	return &reply, reply.QueryMeta.Index, nil
}
*/
