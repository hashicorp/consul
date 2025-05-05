// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cacheshim

import (
	"context"
	"time"
)

// cacheshim defines any shared cache types for any packages that don't want to have a dependency on the agent cache.
// This was created as part of a refactor to remove agent/leafcert package's dependency on agent/cache.

type ResultMeta struct {
	// Hit indicates whether or not the request was a cache hit
	Hit bool

	// Age identifies how "stale" the result is. It's semantics differ based on
	// whether or not the cache type performs background refresh or not as defined
	// in https://developer.hashicorp.com/api/index.html#agent-caching.
	//
	// For background refresh types, Age is 0 unless the background blocking query
	// is currently in a failed state and so not keeping up with the server's
	// values. If it is non-zero it represents the time since the first failure to
	// connect during background refresh, and is reset after a background request
	// does manage to reconnect and either return successfully, or block for at
	// least the yamux keepalive timeout of 30 seconds (which indicates the
	// connection is OK but blocked as expected).
	//
	// For simple cache types, Age is the time since the result being returned was
	// fetched from the servers.
	Age time.Duration

	// Index is the internal ModifyIndex for the cache entry. Not all types
	// support blocking and all that do will likely have this in their result type
	// already but this allows generic code to reason about whether cache values
	// have changed.
	Index uint64
}

type Request interface {
	// CacheInfo returns information used for caching this request.
	CacheInfo() RequestInfo
}

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
	// PeerName is the peer that the request is targeting.
	//
	// All of these values are used to partition the cache. The cache framework
	// today partitions data on these values to simplify behavior: by
	// partitioning ACL tokens, the cache doesn't need to be smart about
	// filtering results. By filtering datacenter/peer results, the cache can
	// service the multi-DC/multi-peer nature of Consul. This comes at the expense of
	// working set size, but in general the effect is minimal.
	Token      string
	Datacenter string
	PeerName   string

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

	// MaxAge if set limits how stale a cache entry can be. If it is non-zero and
	// there is an entry in cache that is older than specified, it is treated as a
	// cache miss and re-fetched. It is ignored for cachetypes with Refresh =
	// true.
	MaxAge time.Duration

	// MustRevalidate forces a new lookup of the cache even if there is an
	// existing one that has not expired. It is implied by HTTP requests with
	// `Cache-Control: max-age=0` but we can't distinguish that case from the
	// unset case for MaxAge. Later we may support revalidating the index without
	// a full re-fetch but for now the only option is to refetch. It is ignored
	// for cachetypes with Refresh = true.
	MustRevalidate bool
}

type UpdateEvent struct {
	// CorrelationID is used by the Notify API to allow correlation of updates
	// with specific requests. We could return the full request object and
	// cachetype for consumers to match against the calls they made but in
	// practice it's cleaner for them to choose the minimal necessary unique
	// identifier given the set of things they are watching. They might even
	// choose to assign random IDs for example.
	CorrelationID string
	Result        interface{}
	Meta          ResultMeta
	Err           error
}

type Callback func(ctx context.Context, event UpdateEvent)

type Cache interface {
	Get(ctx context.Context, t string, r Request) (interface{}, ResultMeta, error)
	NotifyCallback(ctx context.Context, t string, r Request, correlationID string, cb Callback) error
	Notify(ctx context.Context, t string, r Request, correlationID string, ch chan<- UpdateEvent) error
}

const ConnectCARootName = "connect-ca-root"
