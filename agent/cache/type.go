package cache

import (
	"time"
)

// Type implements the logic to fetch certain types of data.
//
//go:generate mockery --name Type --inpackage
type Type interface {
	// Fetch fetches a single unique item.
	//
	// The FetchOptions contain the index and timeouts for blocking queries. The
	// MinIndex value on the Request itself should NOT be used as the blocking
	// index since a request may be reused multiple times as part of Refresh
	// behavior.
	//
	// The return value is a FetchResult which contains information about the
	// fetch. If an error is given, the FetchResult is ignored. The cache does not
	// support backends that return partial values. Optional State can be added to
	// the FetchResult which will be stored with the cache entry and provided to
	// the next Fetch call but will not be returned to clients. This allows types
	// to add additional bookkeeping data per cache entry that will still be aged
	// out along with the entry's TTL.
	//
	// On timeout, FetchResult can behave one of two ways. First, it can return
	// the last known value. This is the default behavior of blocking RPC calls in
	// Consul so this allows cache types to be implemented with no extra logic.
	// Second, FetchResult can return an unset value and index. In this case, the
	// cache will reuse the last value automatically. If an unset Value is
	// returned, the State field will still be updated which allows maintaining
	// metadata even when there is no result.
	Fetch(FetchOptions, Request) (FetchResult, error)

	// RegisterOptions are used when the type is registered to configure the
	// behaviour of cache entries for this type.
	RegisterOptions() RegisterOptions
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

	// LastResult is the result from the last successful Fetch and represents the
	// value currently stored in the cache at the time Fetch is invoked. It will
	// be nil on first call where there is no current cache value. There may have
	// been other Fetch attempts that resulted in an error in the mean time. These
	// are not explicitly represented currently. We could add that if needed this
	// was just simpler for now.
	//
	// The FetchResult read-only! It is constructed per Fetch call so modifying
	// the struct directly (e.g. changing it's Index of Value field) will have no
	// effect, however the Value and State fields may be pointers to the actual
	// values stored in the cache entry. It is thread-unsafe to modify the Value
	// or State via pointers since readers may be concurrently inspecting those
	// values under the entry lock (although we guarantee only one Fetch call per
	// entry) and modifying them even if the index doesn't change or the Fetch
	// eventually errors will likely break logical invariants in the cache too!
	LastResult *FetchResult
}

// FetchResult is the result of a Type Fetch operation and contains the
// data along with metadata gathered from that operation.
type FetchResult struct {
	// Value is the result of the fetch.
	Value interface{}

	// State is opaque data stored in the cache but not returned to clients. It
	// can be used by Types to maintain any bookkeeping they need between fetches
	// (using FetchOptions.LastResult) in a way that gets automatically cleaned up
	// by TTL expiry etc.
	State interface{}

	// Index is the corresponding index value for this data.
	Index uint64

	// NotModified indicates that the Value has not changed since LastResult, and
	// the LastResult value should be used instead of Value.
	NotModified bool
}
