package cache

import (
	"time"
)

// Request is a cacheable request.
//
// This interface is typically implemented by request structures in
// the agent/structs package.
type Request interface {
	// CacheInfo returns information used for caching this request.
	CacheInfo() RequestInfo
}

// RequestInfo represents cache information for a request. The caching
// framework uses this to control the behavior of caching and to determine
// cacheability.
type RequestInfo struct {
	// Key is a unique cache key for this request. This key should
	// be globally unique to identify this request, since any conflicting
	// cache keys could result in invalid data being returned from the cache.
	// The Key does not need to include ACL or DC information, since the
	// cache already partitions by these values prior to using this key.
	Key string

	// Token is the ACL token associated with this request.
	//
	// Datacenter is the datacenter that the request is targeting.
	//
	// Both of these values are used to partition the cache. The cache framework
	// today partitions data on these values to simplify behavior: by
	// partitioning ACL tokens, the cache doesn't need to be smart about
	// filtering results. By filtering datacenter results, the cache can
	// service the multi-DC nature of Consul. This comes at the expense of
	// working set size, but in general the effect is minimal.
	Token      string
	Datacenter string

	// MinIndex is the minimum index being queried. This is used to
	// determine if we already have data satisfying the query or if we need
	// to block until new data is available. If no index is available, the
	// default value (zero) is acceptable.
	MinIndex uint64

	// Timeout is the timeout for waiting on a blocking query. When the
	// timeout is reached, the last known value is returned (or maybe nil
	// if there was no prior value). This "last known value" behavior matches
	// normal Consul blocking queries.
	Timeout time.Duration
}
