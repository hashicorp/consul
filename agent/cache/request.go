package cache

// Request is a cache-able request.
//
// This interface is typically implemented by request structures in
// the agent/structs package.
type Request interface {
	// CacheKey is a unique cache key for this request. This key should
	// absolutely uniquely identify this request, since any conflicting
	// cache keys could result in invalid data being returned from the cache.
	CacheKey() string

	// CacheMinIndex is the minimum index being queried. This is used to
	// determine if we already have data satisfying the query or if we need
	// to block until new data is available.
	CacheMinIndex() uint64
}
