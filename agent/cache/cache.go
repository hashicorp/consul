// Package cache provides caching features for data from a Consul server.
//
// While this is similar in some ways to the "agent/ae" package, a key
// difference is that with anti-entropy, the agent is the authoritative
// source so it resolves differences the server may have. With caching (this
// package), the server is the authoritative source and we do our best to
// balance performance and correctness, depending on the type of data being
// requested.
//
// Currently, the cache package supports only continuous, blocking query
// caching. This means that the cache update is edge-triggered by Consul
// server blocking queries.
package cache

import (
	"fmt"
	"sync"
	"time"
)

//go:generate mockery -all -inpkg

// TODO: DC-aware, ACL-aware

// Cache is a agent-local cache of Consul data.
type Cache struct {
	entriesLock sync.RWMutex
	entries     map[string]cacheEntry

	typesLock sync.RWMutex
	types     map[string]typeEntry
}

// cacheEntry stores a single cache entry.
type cacheEntry struct {
	// Fields pertaining to the actual value
	Value interface{}
	Error error
	Index uint64

	// Metadata that is used for internal accounting
	Valid    bool
	Fetching bool
	Waiter   chan struct{}
}

// typeEntry is a single type that is registered with a Cache.
type typeEntry struct {
	Type Type
	Opts *RegisterOptions
}

// Options are options for the Cache.
type Options struct {
	// Nothing currently, reserved.
}

// New creates a new cache with the given RPC client and reasonable defaults.
// Further settings can be tweaked on the returned value.
func New(*Options) *Cache {
	return &Cache{
		entries: make(map[string]cacheEntry),
		types:   make(map[string]typeEntry),
	}
}

// RegisterOptions are options that can be associated with a type being
// registered for the cache. This changes the behavior of the cache for
// this type.
type RegisterOptions struct {
	// Refresh configures whether the data is actively refreshed or if
	// the data is only refreshed on an explicit Get. The default (false)
	// is to only request data on explicit Get.
	Refresh bool

	// RefreshTimer is the time between attempting to refresh data.
	// If this is zero, then data is refreshed immediately when a fetch
	// is returned.
	//
	// RefreshTimeout determines the maximum query time for a refresh
	// operation. This is specified as part of the query options and is
	// expected to be implemented by the Type itself.
	//
	// Using these values, various "refresh" mechanisms can be implemented:
	//
	//   * With a high timer duration and a low timeout, a timer-based
	//     refresh can be set that minimizes load on the Consul servers.
	//
	//   * With a low timer and high timeout duration, a blocking-query-based
	//     refresh can be set so that changes in server data are recognized
	//     within the cache very quickly.
	//
	RefreshTimer   time.Duration
	RefreshTimeout time.Duration
}

// RegisterType registers a cacheable type.
func (c *Cache) RegisterType(n string, typ Type, opts *RegisterOptions) {
	c.typesLock.Lock()
	defer c.typesLock.Unlock()
	c.types[n] = typeEntry{Type: typ, Opts: opts}
}

// Get loads the data for the given type and request. If data satisfying the
// minimum index is present in the cache, it is returned immediately. Otherwise,
// this will block until the data is available or the request timeout is
// reached.
//
// Multiple Get calls for the same Request (matching CacheKey value) will
// block on a single network request.
func (c *Cache) Get(t string, r Request) (interface{}, error) {
	info := r.CacheInfo()
	if info.Key == "" {
		// If no key is specified, then we do not cache this request.
		// Pass directly through to the backend.
		return c.fetchDirect(t, r)
	}

RETRY_GET:
	// Get the current value
	c.entriesLock.RLock()
	entry, ok := c.entries[info.Key]
	c.entriesLock.RUnlock()

	// If we have a current value and the index is greater than the
	// currently stored index then we return that right away. If the
	// index is zero and we have something in the cache we accept whatever
	// we have.
	if ok && entry.Valid {
		if info.MinIndex == 0 || info.MinIndex < entry.Index {
			return entry.Value, nil
		}
	}

	// At this point, we know we either don't have a value at all or the
	// value we have is too old. We need to wait for new data.
	waiter, err := c.fetch(t, r)
	if err != nil {
		return nil, err
	}

	// Wait on our waiter and then retry the cache load
	<-waiter
	goto RETRY_GET
}

func (c *Cache) fetch(t string, r Request) (<-chan struct{}, error) {
	// Get the type that we're fetching
	c.typesLock.RLock()
	tEntry, ok := c.types[t]
	c.typesLock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown type in cache: %s", t)
	}

	// Grab the cache information while we're outside the lock.
	info := r.CacheInfo()

	c.entriesLock.Lock()
	defer c.entriesLock.Unlock()
	entry, ok := c.entries[info.Key]

	// If we already have an entry and it is actively fetching, then return
	// the currently active waiter.
	if ok && entry.Fetching {
		return entry.Waiter, nil
	}

	// If we don't have an entry, then create it. The entry must be marked
	// as invalid so that it isn't returned as a valid value for a zero index.
	if !ok {
		entry = cacheEntry{Valid: false, Waiter: make(chan struct{})}
	}

	// Set that we're fetching to true, which makes it so that future
	// identical calls to fetch will return the same waiter rather than
	// perform multiple fetches.
	entry.Fetching = true
	c.entries[info.Key] = entry

	// The actual Fetch must be performed in a goroutine.
	go func() {
		// Start building the new entry by blocking on the fetch.
		var newEntry cacheEntry
		result, err := tEntry.Type.Fetch(FetchOptions{
			MinIndex: entry.Index,
		}, r)
		newEntry.Value = result.Value
		newEntry.Index = result.Index
		newEntry.Error = err

		// This is a valid entry with a result
		newEntry.Valid = true

		// Create a new waiter that will be used for the next fetch.
		newEntry.Waiter = make(chan struct{})

		// Insert
		c.entriesLock.Lock()
		c.entries[info.Key] = newEntry
		c.entriesLock.Unlock()

		// Trigger the waiter
		close(entry.Waiter)

		// If refresh is enabled, run the refresh in due time. The refresh
		// below might block, but saves us from spawning another goroutine.
		if tEntry.Opts != nil && tEntry.Opts.Refresh {
			c.refresh(tEntry.Opts, t, r)
		}
	}()

	return entry.Waiter, nil
}

// fetchDirect fetches the given request with no caching.
func (c *Cache) fetchDirect(t string, r Request) (interface{}, error) {
	// Get the type that we're fetching
	c.typesLock.RLock()
	tEntry, ok := c.types[t]
	c.typesLock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown type in cache: %s", t)
	}

	// Fetch it with the min index specified directly by the request.
	result, err := tEntry.Type.Fetch(FetchOptions{
		MinIndex: r.CacheInfo().MinIndex,
	}, r)
	if err != nil {
		return nil, err
	}

	// Return the result and ignore the rest
	return result.Value, nil
}

func (c *Cache) refresh(opts *RegisterOptions, t string, r Request) {
	// Sanity-check, we should not schedule anything that has refresh disabled
	if !opts.Refresh {
		return
	}

	// If we have a timer, wait for it
	if opts.RefreshTimer > 0 {
		time.Sleep(opts.RefreshTimer)
	}

	// Trigger
	c.fetch(t, r)
}
