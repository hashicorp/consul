package cache

import (
	"time"
)

// Type implements the logic to fetch certain types of data.
type Type interface {
	// Fetch fetches a single unique item.
	//
	// The FetchOptions contain the index and timeouts for blocking queries.
	// The MinIndex value on the Request itself should NOT be used
	// as the blocking index since a request may be reused multiple times
	// as part of Refresh behavior.
	//
	// The return value is a FetchResult which contains information about
	// the fetch. If an error is given, the FetchResult is ignored. The
	// cache does not support backends that return partial values.
	//
	// On timeout, FetchResult can behave one of two ways. First, it can
	// return the last known value. This is the default behavior of blocking
	// RPC calls in Consul so this allows cache types to be implemented with
	// no extra logic. Second, FetchResult can return an unset value and index.
	// In this case, the cache will reuse the last value automatically.
	Fetch(FetchOptions, Request) (FetchResult, error)
}

// FetchOptions are various settable options when a Fetch is called.
type FetchOptions struct {
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
