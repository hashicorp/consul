package cachetype

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

// Recommended name for registration.
const ConnectCALeafName = "connect-ca-leaf"

// caChangeJitterWindow is the time over which we spread each round of retries
// when attempting to get a new certificate following a root rotation. It's
// selected to be a trade-off between not making rotation unnecessarily slow on
// a tiny cluster while not hammering the servers on a huge cluster
// unnecessarily hard. Servers rate limit to protect themselves from the
// expensive crypto work, but in practice have 10k+ RPCs all in the same second
// will cause a major disruption even on large servers due to downloading the
// payloads, parsing msgpack etc. Instead we pick a window that for now is fixed
// but later might be either user configurable (not nice since it would become
// another hard-to-tune value) or set dynamically by the server based on it's
// knowledge of how many certs need to be rotated. Currently the server doesn't
// know that so we pick something that is reasonable. We err on the side of
// being slower that we need in trivial cases but gentler for large deployments.
// 30s means that even with a cluster of 10k service instances, the server only
// has to cope with ~333 RPCs a second which shouldn't be too bad if it's rate
// limiting the actual expensive crypto work.
//
// The actual backoff strategy when we are rate limited is to have each cert
// only retry once with each window of this size, at a point in the window
// selected at random. This performs much better than exponential backoff in
// terms of getting things rotated quickly with more predictable load and so
// fewer rate limited requests. See the full simulation this is based on at
// https://github.com/banks/sim-rate-limit-backoff/blob/master/README.md for
// more detail.
const caChangeJitterWindow = 30 * time.Second

// ConnectCALeaf supports fetching and generating Connect leaf
// certificates.
type ConnectCALeaf struct {
	RegisterOptionsBlockingNoRefresh
	caIndex uint64 // Current index for CA roots

	// rootWatchMu protects access to the rootWatchSubscribers map and
	// rootWatchCancel
	rootWatchMu sync.Mutex
	// rootWatchSubscribers is a set of chans, one for each currently in-flight
	// Fetch. These chans have root updates delivered from the root watcher.
	rootWatchSubscribers map[chan struct{}]struct{}
	// rootWatchCancel is a func to call to stop the background root watch if any.
	// You must hold inflightMu to read (e.g. call) or write the value.
	rootWatchCancel func()

	// testRootWatchStart/StopCount are testing helpers that allow tests to
	// observe the reference counting behavior that governs the shared root watch.
	// It's not exactly pretty to expose internals like this, but seems cleaner
	// than constructing elaborate and brittle test cases that we can infer
	// correct behavior from, and simpler than trying to probe runtime goroutine
	// traces to infer correct behavior that way. They must be accessed
	// atomically.
	testRootWatchStartCount uint32
	testRootWatchStopCount  uint32

	RPC        RPC          // RPC client for remote requests
	Cache      *cache.Cache // Cache that has CA root certs via ConnectCARoot
	Datacenter string       // This agent's datacenter

	// TestOverrideCAChangeInitialDelay allows overriding the random jitter after a
	// root change with a fixed delay. So far ths is only done in tests. If it's
	// zero the caChangeInitialSpreadDefault maximum jitter will be used but if
	// set, it overrides and provides a fixed delay. To essentially disable the
	// delay in tests they can set it to 1 nanosecond. We may separately allow
	// configuring the jitter limit by users later but this is different and for
	// tests only since we need to set a deterministic time delay in order to test
	// the behavior here fully and determinstically.
	TestOverrideCAChangeInitialDelay time.Duration
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
	// caChangeJitterWindow above for more.
	activeRootRotationStart time.Time

	// consecutiveRateLimitErrs stores how many rate limit errors we've hit. We
	// use this to choose a new window for the next retry. See comment on
	// caChangeJitterWindow above for more.
	consecutiveRateLimitErrs int
}

func ConnectCALeafSuccess(authorityKeyID string) interface{} {
	return fetchState{
		authorityKeyID:           authorityKeyID,
		forceExpireAfter:         time.Time{},
		consecutiveRateLimitErrs: 0,
		activeRootRotationStart:  time.Time{},
	}
}

// fetchStart is called on each fetch that is about to block and wait for
// changes to the leaf. It subscribes a chan to receive updates from the shared
// root watcher and triggers root watcher if it's not already running.
func (c *ConnectCALeaf) fetchStart(rootUpdateCh chan struct{}) {
	c.rootWatchMu.Lock()
	defer c.rootWatchMu.Unlock()
	// Lazy allocation
	if c.rootWatchSubscribers == nil {
		c.rootWatchSubscribers = make(map[chan struct{}]struct{})
	}
	// Make sure a root watcher is running. We don't only do this on first request
	// to be more tolerant of errors that could cause the root watcher to fail and
	// exit.
	if c.rootWatchCancel == nil {
		ctx, cancel := context.WithCancel(context.Background())
		c.rootWatchCancel = cancel
		go c.rootWatcher(ctx)
	}
	c.rootWatchSubscribers[rootUpdateCh] = struct{}{}
}

// fetchDone is called when a blocking call exits to unsubscribe from root
// updates and possibly stop the shared root watcher if it's no longer needed.
// Note that typically root CA is still being watched by clients directly and
// probably by the ProxyConfigManager so it will stay hot in cache for a while,
// we are just not monitoring it for updates any more.
func (c *ConnectCALeaf) fetchDone(rootUpdateCh chan struct{}) {
	c.rootWatchMu.Lock()
	defer c.rootWatchMu.Unlock()
	delete(c.rootWatchSubscribers, rootUpdateCh)
	if len(c.rootWatchSubscribers) == 0 && c.rootWatchCancel != nil {
		// This was the last request. Stop the root watcher.
		c.rootWatchCancel()
		c.rootWatchCancel = nil
	}
}

// rootWatcher is the shared rootWatcher that runs in a background goroutine
// while needed by one or more inflight Fetch calls.
func (c *ConnectCALeaf) rootWatcher(ctx context.Context) {
	atomic.AddUint32(&c.testRootWatchStartCount, 1)
	defer atomic.AddUint32(&c.testRootWatchStopCount, 1)

	ch := make(chan cache.UpdateEvent, 1)
	err := c.Cache.Notify(ctx, ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter: c.Datacenter,
	}, "roots", ch)

	notifyChange := func() {
		c.rootWatchMu.Lock()
		defer c.rootWatchMu.Unlock()

		for ch := range c.rootWatchSubscribers {
			select {
			case ch <- struct{}{}:
			default:
				// Don't block - chans are 1-buffered so act as an edge trigger and
				// reload CA state directly from cache so they never "miss" updates.
			}
		}
	}

	if err != nil {
		// Trigger all inflight watchers. We don't pass the error, but they will
		// reload from cache and observe the same error and return it to the caller,
		// or if it's transient, will continue and the next Fetch will get us back
		// into the right state. Seems better than busy loop-retrying here given
		// that almost any error we would see here would also be returned from the
		// cache get this will trigger.
		notifyChange()
		return
	}

	var oldRoots *structs.IndexedCARoots
	// Wait for updates to roots or all requests to stop
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-ch:
			// Root response changed in some way. Note this might be the initial
			// fetch.
			if e.Err != nil {
				// See above rationale about the error propagation
				notifyChange()
				continue
			}

			roots, ok := e.Result.(*structs.IndexedCARoots)
			if !ok {
				// See above rationale about the error propagation
				notifyChange()
				continue
			}

			// Check that the active root is actually different from the last CA
			// config there are many reasons the config might have changed without
			// actually updating the CA root that is signing certs in the cluster.
			// The Fetch calls will also validate this since the first call here we
			// don't know if it changed or not, but there is no point waking up all
			// Fetch calls to check this if we know none of them will need to act on
			// this update.
			if oldRoots != nil && oldRoots.ActiveRootID == roots.ActiveRootID {
				continue
			}

			// Distribute the update to all inflight requests - they will decide
			// whether or not they need to act on it.
			notifyChange()
			oldRoots = roots
		}
	}
}

// calculateSoftExpiry encapsulates our logic for when to renew a cert based on
// it's age. It returns a pair of times min, max which makes it easier to test
// the logic without non-deterministic jitter to account for. The caller should
// choose a time randomly in between these.
//
// We want to balance a few factors here:
//   - renew too early and it increases the aggregate CSR rate in the cluster
//   - renew too late and it risks disruption to the service if a transient
//     error prevents the renewal
//   - we want a broad amount of jitter so if there is an outage, we don't end
//     up with all services in sync and causing a thundering herd every
//     renewal period. Broader is better for smoothing requests but pushes
//     both earlier and later tradeoffs above.
//
// Somewhat arbitrarily the current strategy looks like this:
//
//	       0                              60%             90%
//	Issued [------------------------------|===============|!!!!!] Expires
//
// 72h TTL: 0                             ~43h            ~65h
//
//	1h TTL: 0                              36m             54m
//
// Where |===| is the soft renewal period where we jitter for the first attempt
// and |!!!| is the danger zone where we just try immediately.
//
// In the happy path (no outages) the average renewal occurs half way through
// the soft renewal region or at 75% of the cert lifetime which is ~54 hours for
// a 72 hour cert, or 45 mins for a 1 hour cert.
//
// If we are already in the softRenewal period, we randomly pick a time between
// now and the start of the danger zone.
//
// We pass in now to make testing easier.
func calculateSoftExpiry(now time.Time, cert *structs.IssuedCert) (min time.Time, max time.Time) {

	certLifetime := cert.ValidBefore.Sub(cert.ValidAfter)
	if certLifetime < 10*time.Minute {
		// Shouldn't happen as we limit to 1 hour shortest elsewhere but just be
		// defensive against strange times or bugs.
		return now, now
	}

	// Find the 60% mark in diagram above
	softRenewTime := cert.ValidAfter.Add(time.Duration(float64(certLifetime) * 0.6))
	hardRenewTime := cert.ValidAfter.Add(time.Duration(float64(certLifetime) * 0.9))

	if now.After(hardRenewTime) {
		// In the hard renew period, or already expired. Renew now!
		return now, now
	}

	if now.After(softRenewTime) {
		// Already in the soft renew period, make now the lower bound for jitter
		softRenewTime = now
	}
	return softRenewTime, hardRenewTime
}

func (c *ConnectCALeaf) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// Get the correct type
	reqReal, ok := req.(*ConnectCALeafRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Lightweight copy this object so that manipulating QueryOptions doesn't race.
	dup := *reqReal
	reqReal = &dup

	// Do we already have a cert in the cache?
	var existing *structs.IssuedCert
	// Really important this is not a pointer type since otherwise we would set it
	// to point to the actual fetchState in the cache entry below and then would
	// be directly modifying that in the cache entry even when we might later
	// return an error and not update index etc. By being a value, we force a copy
	var state fetchState
	if opts.LastResult != nil {
		existing, ok = opts.LastResult.Value.(*structs.IssuedCert)
		if !ok {
			return result, fmt.Errorf(
				"Internal cache failure: last value wrong type: %T", opts.LastResult.Value)
		}
		if opts.LastResult.State != nil {
			state, ok = opts.LastResult.State.(fetchState)
			if !ok {
				return result, fmt.Errorf(
					"Internal cache failure: last state wrong type: %T", opts.LastResult.State)
			}
		}
	}

	// Handle brand new request first as it's simplest.
	if existing == nil {
		return c.generateNewLeaf(reqReal, result)
	}

	// Setup result to mirror the current value for if we timeout or hit a rate
	// limit. This allows us to update the state (e.g. for backoff or retry
	// coordination on root change) even if we don't get a new cert.
	result.Value = existing
	result.Index = existing.ModifyIndex
	result.State = state

	// Since state is not a pointer, we can't just set it once in result and then
	// continue to update it later since we will be updating only our copy.
	// Instead we have a helper function that is used to make sure the state is
	// updated in the result when we return.
	lastResultWithNewState := func() cache.FetchResult {
		return cache.FetchResult{
			Value: existing,
			Index: existing.ModifyIndex,
			State: state,
		}
	}

	// Beyond this point we need to only return lastResultWithNewState() not just
	// result since otherwise we might "loose" state updates we expect not to.

	// We have a certificate in cache already. Check it's still valid.
	now := time.Now()
	minExpire, maxExpire := calculateSoftExpiry(now, existing)
	expiresAt := minExpire.Add(lib.RandomStagger(maxExpire.Sub(minExpire)))

	// Check if we have been force-expired by a root update that jittered beyond
	// the timeout of the query it was running.
	if !state.forceExpireAfter.IsZero() && state.forceExpireAfter.Before(expiresAt) {
		expiresAt = state.forceExpireAfter
	}

	if expiresAt.Equal(now) || expiresAt.Before(now) {
		// Already expired, just make a new one right away
		return c.generateNewLeaf(reqReal, lastResultWithNewState())
	}

	// If we called Fetch() with MustRevalidate then this call came from a non-blocking query.
	// Any prior CA rotations should've already expired the cert.
	// All we need to do is check whether the current CA is the one that signed the leaf. If not, generate a new leaf.
	// This is not a perfect solution (as a CA rotation update can be missed) but it should take care of instances like
	// see https://github.com/hashicorp/consul/issues/10871, https://github.com/hashicorp/consul/issues/9862
	// This seems to me like a hack, so maybe we can revisit the caching/ fetching logic in this case
	if req.CacheInfo().MustRevalidate {
		roots, err := c.rootsFromCache()
		if err != nil {
			return lastResultWithNewState(), err
		}
		if activeRootHasKey(roots, state.authorityKeyID) {
			return lastResultWithNewState(), nil
		}

		// if we reach here then the current leaf was not signed by the same CAs, just regen
		return c.generateNewLeaf(reqReal, lastResultWithNewState())
	}

	// We are about to block and wait for a change or timeout.

	// Make a chan we can be notified of changes to CA roots on. It must be
	// buffered so we don't miss broadcasts from rootsWatch. It is an edge trigger
	// so a single buffer element is sufficient regardless of whether we consume
	// the updates fast enough since as soon as we see an element in it, we will
	// reload latest CA from cache.
	rootUpdateCh := make(chan struct{}, 1)

	// The roots may have changed in between blocking calls. We need to verify
	// that the existing cert was signed by the current root. If it was we still
	// want to do the whole jitter thing. We could code that again here but it's
	// identical to the select case below so we just trigger our own update chan
	// and let the logic below handle checking if the CA actually changed in the
	// common case where it didn't it is a no-op anyway.
	rootUpdateCh <- struct{}{}

	// Subscribe our chan to get root update notification.
	c.fetchStart(rootUpdateCh)
	defer c.fetchDone(rootUpdateCh)

	// Setup the timeout chan outside the loop so we don't keep bumping the timeout
	// later if we loop around.
	timeoutTimer := time.NewTimer(opts.Timeout)
	defer timeoutTimer.Stop()

	// Setup initial expiry chan. We may change this if root update occurs in the
	// loop below.
	expiresTimer := time.NewTimer(expiresAt.Sub(now))
	defer func() {
		// Resolve the timer reference at defer time, so we use the latest one each time.
		expiresTimer.Stop()
	}()

	// Current cert is valid so just wait until it expires or we time out.
	for {
		select {
		case <-timeoutTimer.C:
			// We timed out the request with same cert.
			return lastResultWithNewState(), nil

		case <-expiresTimer.C:
			// Cert expired or was force-expired by a root change.
			return c.generateNewLeaf(reqReal, lastResultWithNewState())

		case <-rootUpdateCh:
			// A root cache change occurred, reload roots from cache.
			roots, err := c.rootsFromCache()
			if err != nil {
				return lastResultWithNewState(), err
			}

			// Handle _possibly_ changed roots. We still need to verify the new active
			// root is not the same as the one our current cert was signed by since we
			// can be notified spuriously if we are the first request since the
			// rootsWatcher didn't know about the CA we were signed by. We also rely
			// on this on every request to do the initial check that the current roots
			// are the same ones the current cert was signed by.
			if activeRootHasKey(roots, state.authorityKeyID) {
				// Current active CA is the same one that signed our current cert so
				// keep waiting for a change.
				continue
			}
			state.activeRootRotationStart = time.Now()

			// CA root changed. We add some jitter here to avoid a thundering herd.
			// See docs on caChangeJitterWindow const.
			delay := lib.RandomStagger(caChangeJitterWindow)
			if c.TestOverrideCAChangeInitialDelay > 0 {
				delay = c.TestOverrideCAChangeInitialDelay
			}
			// Force the cert to be expired after the jitter - the delay above might
			// be longer than we have left on our timeout. We set forceExpireAfter in
			// the cache state so the next request will notice we still need to renew
			// and do it at the right time. This is cleared once a new cert is
			// returned by generateNewLeaf.
			state.forceExpireAfter = state.activeRootRotationStart.Add(delay)
			// If the delay time is within the current timeout, we want to renew the
			// as soon as it's up. We change the expire time and chan so that when we
			// loop back around, we'll wait at most delay until generating a new cert.
			if state.forceExpireAfter.Before(expiresAt) {
				expiresAt = state.forceExpireAfter
				// Stop the former one and create a new one.
				expiresTimer.Stop()
				expiresTimer = time.NewTimer(delay)
			}
			continue
		}
	}
}

func activeRootHasKey(roots *structs.IndexedCARoots, currentSigningKeyID string) bool {
	for _, ca := range roots.Roots {
		if ca.Active {
			return ca.SigningKeyID == currentSigningKeyID
		}
	}
	// Shouldn't be possible since at least one root should be active.
	return false
}

func (c *ConnectCALeaf) rootsFromCache() (*structs.IndexedCARoots, error) {
	// Background is fine here because this isn't a blocking query as no index is set.
	// Therefore this will just either be a cache hit or return once the non-blocking query returns.
	rawRoots, _, err := c.Cache.Get(context.Background(), ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter: c.Datacenter,
	})
	if err != nil {
		return nil, err
	}
	roots, ok := rawRoots.(*structs.IndexedCARoots)
	if !ok {
		return nil, errors.New("invalid RootCA response type")
	}
	return roots, nil
}

// generateNewLeaf does the actual work of creating a new private key,
// generating a CSR and getting it signed by the servers. result argument
// represents the last result currently in cache if any along with its state.
func (c *ConnectCALeaf) generateNewLeaf(req *ConnectCALeafRequest,
	result cache.FetchResult) (cache.FetchResult, error) {

	var state fetchState
	if result.State != nil {
		var ok bool
		state, ok = result.State.(fetchState)
		if !ok {
			return result, fmt.Errorf(
				"Internal cache failure: result state wrong type: %T", result.State)
		}
	}

	// Need to lookup RootCAs response to discover trust domain. This should be a
	// cache hit.
	roots, err := c.rootsFromCache()
	if err != nil {
		return result, err
	}
	if roots.TrustDomain == "" {
		return result, errors.New("cluster has no CA bootstrapped yet")
	}

	// Build the cert uri
	var id connect.CertURI
	var dnsNames []string
	var ipAddresses []net.IP
	if req.Service != "" {
		id = &connect.SpiffeIDService{
			Host:       roots.TrustDomain,
			Datacenter: req.Datacenter,
			Partition:  req.TargetPartition(),
			Namespace:  req.TargetNamespace(),
			Service:    req.Service,
		}
		dnsNames = append(dnsNames, req.DNSSAN...)
	} else if req.Agent != "" {
		id = &connect.SpiffeIDAgent{
			Host:       roots.TrustDomain,
			Datacenter: req.Datacenter,
			Partition:  req.TargetPartition(),
			Agent:      req.Agent,
		}
		dnsNames = append([]string{"localhost"}, req.DNSSAN...)
		ipAddresses = append([]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}, req.IPSAN...)
	} else if req.Kind != "" {
		if req.Kind != structs.ServiceKindMeshGateway {
			return result, fmt.Errorf("unsupported kind: %s", req.Kind)
		}

		id = &connect.SpiffeIDMeshGateway{
			Host:       roots.TrustDomain,
			Datacenter: req.Datacenter,
			Partition:  req.TargetPartition(),
		}
		dnsNames = append(dnsNames, req.DNSSAN...)
	} else {
		return result, errors.New("URI must be either service, agent, or kind")
	}

	// Create a new private key

	// TODO: for now we always generate EC keys on clients regardless of the key
	// type being used by the active CA. This is fine and allowed in TLS1.2 and
	// signing EC CSRs with an RSA key is supported by all current CA providers so
	// it's OK. IFF we ever need to support a CA provider that refuses to sign a
	// CSR with a different signature algorithm, or if we have compatibility
	// issues with external PKI systems that require EC certs be signed with ECDSA
	// from the CA (this was required in TLS1.1 but not in 1.2) then we can
	// instead intelligently pick the key type we generate here based on the key
	// type of the active signing CA. We already have that loaded since we need
	// the trust domain.
	pk, pkPEM, err := connect.GeneratePrivateKey()
	if err != nil {
		return result, err
	}

	// Create a CSR.
	csr, err := connect.CreateCSR(id, pk, dnsNames, ipAddresses)
	if err != nil {
		return result, err
	}

	// Request signing
	var reply structs.IssuedCert
	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: req.Token},
		Datacenter:   req.Datacenter,
		CSR:          csr,
	}
	if err := c.RPC.RPC("ConnectCA.Sign", &args, &reply); err != nil {
		if err.Error() == consul.ErrRateLimited.Error() {
			if result.Value == nil {
				// This was a first fetch - we have no good value in cache. In this case
				// we just return the error to the caller rather than rely on surprising
				// semi-blocking until the rate limit is appeased or we timeout
				// behavior. It's likely the caller isn't expecting this to block since
				// it's an initial fetch. This also massively simplifies this edge case.
				return result, err
			}

			if state.activeRootRotationStart.IsZero() {
				// We hit a rate limit error by chance - for example a cert expired
				// before the root rotation was observed (not triggered by rotation) but
				// while server is working through high load from a recent rotation.
				// Just pretend there is a rotation and the retry logic here will start
				// jittering and retrying in the same way from now.
				state.activeRootRotationStart = time.Now()
			}

			// Increment the errors in the state
			state.consecutiveRateLimitErrs++

			delay := lib.RandomStagger(caChangeJitterWindow)
			if c.TestOverrideCAChangeInitialDelay > 0 {
				delay = c.TestOverrideCAChangeInitialDelay
			}

			// Find the start of the next window we can retry in. See comment on
			// caChangeJitterWindow for details of why we use this strategy.
			windowStart := state.activeRootRotationStart.Add(
				time.Duration(state.consecutiveRateLimitErrs) * delay)

			// Pick a random time in that window
			state.forceExpireAfter = windowStart.Add(delay)

			// Return a result with the existing cert but the new state - the cache
			// will see this as no change. Note that we always have an existing result
			// here due to the nil value check above.
			result.State = state
			return result, nil
		}
		return result, err
	}
	reply.PrivateKeyPEM = pkPEM

	// Reset rotation state
	state.forceExpireAfter = time.Time{}
	state.consecutiveRateLimitErrs = 0
	state.activeRootRotationStart = time.Time{}

	cert, err := connect.ParseCert(reply.CertPEM)
	if err != nil {
		return result, err
	}
	// Set the CA key ID so we can easily tell when a active root has changed.
	state.authorityKeyID = connect.EncodeSigningKeyID(cert.AuthorityKeyId)

	result.Value = &reply
	// Store value not pointer so we don't accidentally mutate the cache entry
	// state in Fetch.
	result.State = state
	result.Index = reply.ModifyIndex
	return result, nil
}

// ConnectCALeafRequest is the cache.Request implementation for the
// ConnectCALeaf cache type. This is implemented here and not in structs
// since this is only used for cache-related requests and not forwarded
// directly to any Consul servers.
type ConnectCALeafRequest struct {
	Token          string
	Datacenter     string
	Service        string              // Service name, not ID
	Agent          string              // Agent name, not ID
	Kind           structs.ServiceKind // only mesh-gateway for now
	DNSSAN         []string
	IPSAN          []net.IP
	MinQueryIndex  uint64
	MaxQueryTime   time.Duration
	MustRevalidate bool

	acl.EnterpriseMeta
}

func (r *ConnectCALeafRequest) Key() string {
	r.EnterpriseMeta.Normalize()

	switch {
	case r.Agent != "":
		v, err := hashstructure.Hash([]interface{}{
			r.Agent,
			r.PartitionOrDefault(),
		}, nil)
		if err == nil {
			return fmt.Sprintf("agent:%d", v)
		}
	case r.Kind == structs.ServiceKindMeshGateway:
		v, err := hashstructure.Hash([]interface{}{
			r.PartitionOrDefault(),
			r.DNSSAN,
			r.IPSAN,
		}, nil)
		if err == nil {
			return fmt.Sprintf("kind:%d", v)
		}
	case r.Kind != "":
		// this is not valid
	default:
		v, err := hashstructure.Hash([]interface{}{
			r.Service,
			r.EnterpriseMeta,
			r.DNSSAN,
			r.IPSAN,
		}, nil)
		if err == nil {
			return fmt.Sprintf("service:%d", v)
		}
	}

	// If there is an error, we don't set the key. A blank key forces
	// no cache for this request so the request is forwarded directly
	// to the server.
	return ""
}

func (req *ConnectCALeafRequest) TargetPartition() string {
	return req.PartitionOrDefault()
}

func (r *ConnectCALeafRequest) CacheInfo() cache.RequestInfo {
	return cache.RequestInfo{
		Token:          r.Token,
		Key:            r.Key(),
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MustRevalidate: r.MustRevalidate,
	}
}
