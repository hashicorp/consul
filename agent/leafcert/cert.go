// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package leafcert

import (
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/ttlcache"
)

// certData tracks all of the metadata about a leaf cert.
type certData struct {
	// lock locks access to all fields
	lock sync.Mutex

	// index is the last raft index associated with an update of the 'value' field
	index uint64

	// value is the last updated cert contents or nil if not populated initially
	value *structs.IssuedCert

	// state is metadata related to cert generation
	state fetchState

	// fetchedAt was the time when 'value' was last updated
	fetchedAt time.Time

	// refreshing indicates if there is an active request attempting to refresh
	// the current leaf cert contents.
	refreshing bool

	// lastFetchErr is the last error encountered when attempting to populate
	// the 'value' field.
	lastFetchErr error

	// expiry contains information about the expiration of this
	// cert. This is a pointer as its shared as a value in the
	// ExpiryHeap as well.
	expiry *ttlcache.Entry

	// refreshRateLimiter limits the rate at which the cert can be regenerated
	refreshRateLimiter *rate.Limiter
}

func (c *certData) MarkRefreshing(v bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.refreshing = v
}

func (c *certData) GetValueAndState() (*structs.IssuedCert, fetchState) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.value, c.state
}

func (c *certData) GetError() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.lastFetchErr
}

// NOTE: this function only has one goroutine in it per key at all times
func (c *certData) Update(
	newCert *structs.IssuedCert,
	newState fetchState,
	err error,
) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Importantly, always reset the Error. Having both Error and a Value that
	// are non-nil is allowed in the cache entry but it indicates that the Error
	// is _newer_ than the last good value. So if the err is nil then we need to
	// reset to replace any _older_ errors and avoid them bubbling up. If the
	// error is non-nil then we need to set it anyway and used to do it in the
	// code below. See https://github.com/hashicorp/consul/issues/4480.
	c.lastFetchErr = err

	c.state = newState
	if newCert != nil {
		c.index = newCert.ModifyIndex
		c.value = newCert
		c.fetchedAt = time.Now()
	}

	if c.index < 1 {
		// Less than one is invalid unless there was an error and in this case
		// there wasn't since a value was returned. If a badly behaved RPC
		// returns 0 when it has no data, we might get into a busy loop here. We
		// set this to minimum of 1 which is safe because no valid user data can
		// ever be written at raft index 1 due to the bootstrap process for
		// raft. This insure that any subsequent background refresh request will
		// always block, but allows the initial request to return immediately
		// even if there is no data.
		c.index = 1
	}
}

// fetchState is some additional metadata we store with each cert in the cache
// to track things like expiry and coordinate paces root rotations. It's
// important this doesn't contain any pointer types since we rely on the struct
// being copied to avoid modifying the actual state in the cache entry during
// Fetch. Pointers themselves are OK, but if we point to another struct that we
// call a method or modify in some way that would directly mutate the cache and
// cause problems. We'd need to deep-clone in that case in Fetch below.
// time.Time technically contains a pointer to the Location but we ignore that
// since all times we get from our wall clock should point to the same Location
// anyway.
type fetchState struct {
	// authorityKeyId is the ID of the CA key (whether root or intermediate) that signed
	// the current cert.  This is just to save parsing the whole cert everytime
	// we have to check if the root changed.
	authorityKeyID string

	// forceExpireAfter is used to coordinate renewing certs after a CA rotation
	// in a staggered way so that we don't overwhelm the servers.
	forceExpireAfter time.Time

	// activeRootRotationStart is set when the root has changed and we need to get
	// a new cert but haven't got one yet. forceExpireAfter will be set to the
	// next scheduled time we should try our CSR, but this is needed to calculate
	// the retry windows if we are rate limited when we try. See comment on
	// const caChangeJitterWindow above for more.
	activeRootRotationStart time.Time

	// consecutiveRateLimitErrs stores how many rate limit errors we've hit. We
	// use this to choose a new window for the next retry. See comment on
	// const caChangeJitterWindow above for more.
	consecutiveRateLimitErrs int
}
