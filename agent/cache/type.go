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
