// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cache

import (
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/lib/ttlcache"
)

// cacheEntry stores a single cache entry.
//
// Note that this isn't a very optimized structure currently. There are
// a lot of improvements that can be made here in the long term.
type cacheEntry struct {
	// Fields pertaining to the actual value
	Value interface{}
	// State can be used to store info needed by the cache type but that should
	// not be part of the result the client gets. For example the Connect Leaf
	// type needs to store additional data about when it last attempted a renewal
	// that is not part of the actual IssuedCert struct it returns. It's opaque to
	// the Cache but allows types to store additional data that is coupled to the
	// cache entry's lifetime and will be aged out by TTL etc.
	State interface{}
	Error error
	Index uint64

	// Metadata that is used for internal accounting
	Valid    bool          // True if the Value is set
	Fetching bool          // True if a fetch is already active
	Waiter   chan struct{} // Closed when this entry is invalidated

	// Expiry contains information about the expiration of this
	// entry. This is a pointer as its shared as a value in the
	// ExpiryHeap as well.
	Expiry *ttlcache.Entry

	// FetchedAt stores the time the cache entry was retrieved for determining
	// it's age later.
	FetchedAt time.Time

	// RefreshLostContact stores the time background refresh failed. It gets reset
	// to zero after a background fetch has returned successfully, or after a
	// background request has be blocking for at least 5 seconds, which ever
	// happens first.
	RefreshLostContact time.Time
	// FetchRateLimiter limits the rate at which fetch is called for this entry.
	FetchRateLimiter *rate.Limiter
}
