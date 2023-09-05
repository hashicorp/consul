package leafcert

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/ttlcache"
)

const (
	DefaultLastGetTTL = 72 * time.Hour // reasonable default is days

	// DefaultLeafCertRefreshRate is the default rate at which certs can be refreshed.
	// This defaults to not being limited
	DefaultLeafCertRefreshRate = rate.Inf

	// DefaultLeafCertRefreshMaxBurst is the number of cache entry fetches that can
	// occur in a burst.
	DefaultLeafCertRefreshMaxBurst = 2

	DefaultLeafCertRefreshBackoffMin = 3               // 3 attempts before backing off
	DefaultLeafCertRefreshMaxWait    = 1 * time.Minute // maximum backoff wait time

	DefaultQueryTimeout = 10 * time.Minute
)

type Config struct {
	// LastGetTTL is the time that the certs returned by this type remain in
	// the cache after the last get operation. If a cert isn't accessed within
	// this duration, the certs is purged and background refreshing will cease.
	LastGetTTL time.Duration

	// LeafCertRefreshMaxBurst max burst size of RateLimit for a single cache entry
	LeafCertRefreshMaxBurst int

	// LeafCertRefreshRate represents the max calls/sec for a single cache entry
	LeafCertRefreshRate rate.Limit

	// LeafCertRefreshBackoffMin is the number of attempts to wait before
	// backing off.
	//
	// Mostly configurable just for testing.
	LeafCertRefreshBackoffMin uint

	// LeafCertRefreshMaxWait is the maximum backoff wait time.
	//
	// Mostly configurable just for testing.
	LeafCertRefreshMaxWait time.Duration

	// TestOverrideCAChangeInitialDelay allows overriding the random jitter
	// after a root change with a fixed delay. So far ths is only done in
	// tests. If it's zero the caChangeInitialSpreadDefault maximum jitter will
	// be used but if set, it overrides and provides a fixed delay. To
	// essentially disable the delay in tests they can set it to 1 nanosecond.
	// We may separately allow configuring the jitter limit by users later but
	// this is different and for tests only since we need to set a
	// deterministic time delay in order to test the behavior here fully and
	// determinstically.
	TestOverrideCAChangeInitialDelay time.Duration
}

func (c Config) withDefaults() Config {
	if c.LastGetTTL <= 0 {
		c.LastGetTTL = DefaultLastGetTTL
	}
	if c.LeafCertRefreshRate == 0.0 {
		c.LeafCertRefreshRate = DefaultLeafCertRefreshRate
	}
	if c.LeafCertRefreshMaxBurst == 0 {
		c.LeafCertRefreshMaxBurst = DefaultLeafCertRefreshMaxBurst
	}
	if c.LeafCertRefreshBackoffMin == 0 {
		c.LeafCertRefreshBackoffMin = DefaultLeafCertRefreshBackoffMin
	}
	if c.LeafCertRefreshMaxWait == 0 {
		c.LeafCertRefreshMaxWait = DefaultLeafCertRefreshMaxWait
	}
	return c
}

type Deps struct {
	Config Config
	Logger hclog.Logger

	// RootsReader is an interface to access connect CA roots.
	RootsReader RootsReader

	// CertSigner is an interface to remotely sign certificates.
	CertSigner CertSigner
}

type RootsReader interface {
	Get() (*structs.IndexedCARoots, error)
	Notify(ctx context.Context, correlationID string, ch chan<- cache.UpdateEvent) error
}

type CertSigner interface {
	SignCert(ctx context.Context, args *structs.CASignRequest) (*structs.IssuedCert, error)
}

func NewManager(deps Deps) *Manager {
	deps.Config = deps.Config.withDefaults()

	if deps.Logger == nil {
		deps.Logger = hclog.NewNullLogger()
	}
	if deps.RootsReader == nil {
		panic("RootsReader is required")
	}
	if deps.CertSigner == nil {
		panic("CertSigner is required")
	}

	m := &Manager{
		config:      deps.Config,
		logger:      deps.Logger,
		certSigner:  deps.CertSigner,
		rootsReader: deps.RootsReader,
		//
		certs:           make(map[string]*certData),
		certsExpiryHeap: ttlcache.NewExpiryHeap(),
	}

	m.ctx, m.ctxCancel = context.WithCancel(context.Background())

	m.rootWatcher = &rootWatcher{
		ctx:         m.ctx,
		rootsReader: m.rootsReader,
	}

	// Start the expiry watcher
	go m.runExpiryLoop()

	return m
}

type Manager struct {
	logger hclog.Logger

	// config contains agent configuration necessary for the cert manager to operate.
	config Config

	// rootsReader is an interface to access connect CA roots.
	rootsReader RootsReader

	// certSigner is an interface to remotely sign certificates.
	certSigner CertSigner

	// rootWatcher helps let multiple requests for leaf certs to coordinate
	// sharing a single long-lived watch for the root certs. This allows the
	// leaf cert requests to notice when the roots rotate and trigger their
	// reissuance.
	rootWatcher *rootWatcher

	// This is the "top-level" internal context. This is used to cancel
	// background operations.
	ctx       context.Context
	ctxCancel context.CancelFunc

	// lock guards access to certs and certsExpiryHeap
	lock            sync.RWMutex
	certs           map[string]*certData
	certsExpiryHeap *ttlcache.ExpiryHeap

	// certGroup is a singleflight group keyed identically to the certs map.
	// When the leaf cert itself needs replacement requests will coalesce
	// together through this chokepoint.
	certGroup singleflight.Group
}

func (m *Manager) getCertData(key string) *certData {
	m.lock.RLock()
	cd, ok := m.certs[key]
	m.lock.RUnlock()

	if ok {
		return cd
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	cd, ok = m.certs[key]
	if !ok {
		cd = &certData{
			expiry: m.certsExpiryHeap.Add(key, m.config.LastGetTTL),
			refreshRateLimiter: rate.NewLimiter(
				m.config.LeafCertRefreshRate,
				m.config.LeafCertRefreshMaxBurst,
			),
		}

		m.certs[key] = cd

		metrics.SetGauge([]string{"leaf-certs", "entries_count"}, float32(len(m.certs)))
	}
	return cd
}

// Stop stops any background work and frees all resources for the manager.
// Current fetch requests are allowed to continue to completion and callers may
// still access the current leaf cert values so coordination isn't needed with
// callers, however no background activity will continue. It's intended to
// close the manager at agent shutdown so no further requests should be made,
// however concurrent or in-flight ones won't break.
func (m *Manager) Stop() {
	if m.ctxCancel != nil {
		m.ctxCancel()
		m.ctxCancel = nil
	}
}

// Get returns the leaf cert for the request. If data satisfying the
// minimum index is present, it is returned immediately. Otherwise,
// this will block until the cert is refreshed or the request timeout is
// reached.
//
// Multiple Get calls for the same logical request will block on a single
// network request.
//
// The timeout specified by the request will be the timeout on the cache
// Get, and does not correspond to the timeout of any background data
// fetching. If the timeout is reached before data satisfying the minimum
// index is retrieved, the last known value (maybe nil) is returned. No
// error is returned on timeout. This matches the behavior of Consul blocking
// queries.
func (m *Manager) Get(ctx context.Context, req *ConnectCALeafRequest) (*structs.IssuedCert, cache.ResultMeta, error) {
	// Lightweight copy this object so that manipulating req doesn't race.
	dup := *req
	req = &dup

	// We don't want non-blocking queries to return expired leaf certs
	// or leaf certs not valid under the current CA. So always revalidate
	// the leaf cert on non-blocking queries (ie when MinQueryIndex == 0)
	//
	// NOTE: This conditional was formerly only in the API endpoint.
	if req.MinQueryIndex == 0 {
		req.MustRevalidate = true
	}

	return m.internalGet(ctx, req)
}

func (m *Manager) internalGet(ctx context.Context, req *ConnectCALeafRequest) (*structs.IssuedCert, cache.ResultMeta, error) {
	key := req.Key()
	if key == "" {
		return nil, cache.ResultMeta{}, fmt.Errorf("a key is required")
	}

	if req.MaxQueryTime <= 0 {
		req.MaxQueryTime = DefaultQueryTimeout
	}
	timeoutTimer := time.NewTimer(req.MaxQueryTime)
	defer timeoutTimer.Stop()

	// First time through
	first := true

	for {
		// Get the current value
		cd := m.getCertData(key)

		cd.lock.Lock()
		var (
			existing      = cd.value
			existingIndex = cd.index
			refreshing    = cd.refreshing
			fetchedAt     = cd.fetchedAt
			lastFetchErr  = cd.lastFetchErr
			expiry        = cd.expiry
		)
		cd.lock.Unlock()

		shouldReplaceCert := certNeedsUpdate(req, existingIndex, existing, refreshing)

		if expiry != nil {
			// The entry already exists in the TTL heap, touch it to keep it alive since
			// this Get is still interested in the value. Note that we used to only do
			// this in the `entryValid` block below but that means that a cache entry
			// will expire after it's TTL regardless of how many callers are waiting for
			// updates in this method in a couple of cases:
			//
			//  1. If the agent is disconnected from servers for the TTL then the client
			//     will be in backoff getting errors on each call to Get and since an
			//     errored cache entry has Valid = false it won't be touching the TTL.
			//
			//  2. If the value is just not changing then the client's current index
			//     will be equal to the entry index and entryValid will be false. This
			//     is a common case!
			//
			// But regardless of the state of the entry, assuming it's already in the
			// TTL heap, we should touch it every time around here since this caller at
			// least still cares about the value!
			m.lock.Lock()
			m.certsExpiryHeap.Update(expiry.Index(), m.config.LastGetTTL)
			m.lock.Unlock()
		}

		if !shouldReplaceCert {
			meta := cache.ResultMeta{
				Index: existingIndex,
			}

			if first {
				meta.Hit = true
			}

			// For non-background refresh types, the age is just how long since we
			// fetched it last.
			if !fetchedAt.IsZero() {
				meta.Age = time.Since(fetchedAt)
			}

			// We purposely do not return an error here since the cache only works with
			// fetching values that either have a value or have an error, but not both.
			// The Error may be non-nil in the entry in the case that an error has
			// occurred _since_ the last good value, but we still want to return the
			// good value to clients that are not requesting a specific version. The
			// effect of this is that blocking clients will all see an error immediately
			// without waiting a whole timeout to see it, but clients that just look up
			// cache with an older index than the last valid result will still see the
			// result and not the error here. I.e. the error is not "cached" without a
			// new fetch attempt occurring, but the last good value can still be fetched
			// from cache.
			return existing, meta, nil
		}

		// If this isn't our first time through and our last value has an error, then
		// we return the error. This has the behavior that we don't sit in a retry
		// loop getting the same error for the entire duration of the timeout.
		// Instead, we make one effort to fetch a new value, and if there was an
		// error, we return. Note that the invariant is that if both entry.Value AND
		// entry.Error are non-nil, the error _must_ be more recent than the Value. In
		// other words valid fetches should reset the error. See
		// https://github.com/hashicorp/consul/issues/4480.
		if !first && lastFetchErr != nil {
			return existing, cache.ResultMeta{Index: existingIndex}, lastFetchErr
		}

		notifyCh := m.triggerCertRefreshInGroup(req, cd)

		// No longer our first time through
		first = false

		select {
		case <-ctx.Done():
			return nil, cache.ResultMeta{}, ctx.Err()
		case <-notifyCh:
			// Our fetch returned, retry the get from the cache.
			req.MustRevalidate = false

		case <-timeoutTimer.C:
			// Timeout on the cache read, just return whatever we have.
			return existing, cache.ResultMeta{Index: existingIndex}, nil
		}
	}
}

func certNeedsUpdate(req *ConnectCALeafRequest, index uint64, value *structs.IssuedCert, refreshing bool) bool {
	if value == nil {
		return true
	}

	if req.MinQueryIndex > 0 && req.MinQueryIndex >= index {
		// MinIndex was given and matches or is higher than current value so we
		// ignore the cache and fallthrough to blocking on a new value.
		return true
	}

	// Check if re-validate is requested. If so the first time round the
	// loop is not a hit but subsequent ones should be treated normally.
	if req.MustRevalidate {
		// It is important to note that this block ONLY applies when we are not
		// in indefinite refresh mode (where the underlying goroutine will
		// continue to re-query for data).
		//
		// In this mode goroutines have a 1:1 relationship to RPCs that get
		// executed, and importantly they DO NOT SLEEP after executing.
		//
		// This means that a running goroutine for this cache entry extremely
		// strongly implies that the RPC has not yet completed, which is why
		// this check works for the revalidation-avoidance optimization here.
		if refreshing {
			// There is an active goroutine performing a blocking query for
			// this data, which has not returned.
			//
			// We can logically deduce that the contents of the cache are
			// actually current, and we can simply return this while leaving
			// the blocking query alone.
			return false
		} else {
			return true
		}
	}

	return false
}

func (m *Manager) triggerCertRefreshInGroup(req *ConnectCALeafRequest, cd *certData) <-chan singleflight.Result {
	// Lightweight copy this object so that manipulating req doesn't race.
	dup := *req
	req = &dup

	if req.MaxQueryTime == 0 {
		req.MaxQueryTime = DefaultQueryTimeout
	}

	// At this point, we know we either don't have a cert at all or the
	// cert we have is too old. We need to mint a new one.
	//
	// We use a singleflight group to coordinate only one request driving
	// the async update to the key at once.
	//
	// NOTE: this anonymous function only has one goroutine in it per key at all times
	return m.certGroup.DoChan(req.Key(), func() (any, error) {
		cd.lock.Lock()
		var (
			shouldReplaceCert = certNeedsUpdate(req, cd.index, cd.value, cd.refreshing)
			rateLimiter       = cd.refreshRateLimiter
			lastIndex         = cd.index
		)
		cd.lock.Unlock()

		if !shouldReplaceCert {
			// This handles the case where a fetch succeeded after checking for
			// its existence in Get. This ensures that we don't miss updates
			// since we don't hold the lock between the read and then the
			// refresh trigger.
			return nil, nil
		}

		if err := rateLimiter.Wait(m.ctx); err != nil {
			// NOTE: this can only happen when the entire cache is being
			// shutdown and isn't something that can happen normally.
			return nil, nil
		}

		cd.MarkRefreshing(true)
		defer cd.MarkRefreshing(false)

		req.MinQueryIndex = lastIndex

		// Start building the new entry by blocking on the fetch.
		m.refreshLeafAndUpdate(req, cd)

		return nil, nil
	})
}

// testGet is a way for the test code to do a get but from the middle of the
// logic stack, skipping some of the caching logic.
func (m *Manager) testGet(req *ConnectCALeafRequest) (uint64, *structs.IssuedCert, error) {
	cd := m.getCertData(req.Key())

	m.refreshLeafAndUpdate(req, cd)

	cd.lock.Lock()
	var (
		index = cd.index
		cert  = cd.value
		err   = cd.lastFetchErr
	)
	cd.lock.Unlock()

	if err != nil {
		return 0, nil, err
	}

	return index, cert, nil
}

// refreshLeafAndUpdate will try to refresh the leaf and persist the updated
// data back to the in-memory store.
//
// NOTE: this function only has one goroutine in it per key at all times
func (m *Manager) refreshLeafAndUpdate(req *ConnectCALeafRequest, cd *certData) {
	existing, state := cd.GetValueAndState()
	newCert, updatedState, err := m.attemptLeafRefresh(req, existing, state)
	cd.Update(newCert, updatedState, err)
}

// Prepopulate puts a cert in manually. This is useful when the correct initial
// value is known and the cache shouldn't refetch the same thing on startup. It
// is used to set AgentLeafCert when AutoEncrypt.TLS is turned on. The manager
// itself cannot fetch that the first time because it requires a special
// RPCType. Subsequent runs are fine though.
func (m *Manager) Prepopulate(
	ctx context.Context,
	key string,
	index uint64,
	value *structs.IssuedCert,
	authorityKeyID string,
) error {
	if value == nil {
		return errors.New("value is required")
	}
	cd := m.getCertData(key)

	cd.lock.Lock()
	defer cd.lock.Unlock()

	cd.index = index
	cd.value = value
	cd.state = fetchState{
		authorityKeyID:           authorityKeyID,
		forceExpireAfter:         time.Time{},
		consecutiveRateLimitErrs: 0,
		activeRootRotationStart:  time.Time{},
	}

	return nil
}

// runExpiryLoop is a blocking function that watches the expiration
// heap and invalidates cert entries that have expired.
func (m *Manager) runExpiryLoop() {
	for {
		m.lock.RLock()
		timer := m.certsExpiryHeap.Next()
		m.lock.RUnlock()

		select {
		case <-m.ctx.Done():
			timer.Stop()
			return
		case <-m.certsExpiryHeap.NotifyCh:
			timer.Stop()
			continue

		case <-timer.Wait():
			m.lock.Lock()

			entry := timer.Entry

			// Entry expired! Remove it.
			delete(m.certs, entry.Key())
			m.certsExpiryHeap.Remove(entry.Index())

			// Set some metrics
			metrics.IncrCounter([]string{"leaf-certs", "evict_expired"}, 1)
			metrics.SetGauge([]string{"leaf-certs", "entries_count"}, float32(len(m.certs)))

			m.lock.Unlock()
		}
	}
}
