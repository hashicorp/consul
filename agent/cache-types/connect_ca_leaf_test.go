package cachetype

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestCalculateSoftExpire(t *testing.T) {
	tests := []struct {
		name     string
		now      string
		issued   string
		lifetime time.Duration
		wantMin  string
		wantMax  string
	}{
		{
			name:     "72h just issued",
			now:      "2018-01-01 00:00:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Should jitter between 60% and 90% of the lifetime which is 43.2/64.8
			// hours after issued
			wantMin: "2018-01-02 19:12:00",
			wantMax: "2018-01-03 16:48:00",
		},
		{
			name: "72h in renew range",
			// This time should be inside the renewal range.
			now:      "2018-01-02 20:00:20",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min should be the "now" time
			wantMin: "2018-01-02 20:00:20",
			wantMax: "2018-01-03 16:48:00",
		},
		{
			name: "72h in hard renew",
			// This time should be inside the renewal range.
			now:      "2018-01-03 18:00:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-03 18:00:00",
			wantMax: "2018-01-03 18:00:00",
		},
		{
			name: "72h expired",
			// This time is after expiry
			now:      "2018-01-05 00:00:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-05 00:00:00",
			wantMax: "2018-01-05 00:00:00",
		},
		{
			name:     "1h just issued",
			now:      "2018-01-01 00:00:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Should jitter between 60% and 90% of the lifetime which is 36/54 mins
			// hours after issued
			wantMin: "2018-01-01 00:36:00",
			wantMax: "2018-01-01 00:54:00",
		},
		{
			name: "1h in renew range",
			// This time should be inside the renewal range.
			now:      "2018-01-01 00:40:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min should be the "now" time
			wantMin: "2018-01-01 00:40:00",
			wantMax: "2018-01-01 00:54:00",
		},
		{
			name: "1h in hard renew",
			// This time should be inside the renewal range.
			now:      "2018-01-01 00:55:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 00:55:00",
			wantMax: "2018-01-01 00:55:00",
		},
		{
			name: "1h expired",
			// This time is after expiry
			now:      "2018-01-01 01:01:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 01:01:01",
			wantMax: "2018-01-01 01:01:01",
		},
		{
			name: "too short lifetime",
			// This time is after expiry
			now:      "2018-01-01 01:01:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Minute,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 01:01:01",
			wantMax: "2018-01-01 01:01:01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.Parse("2006-01-02 15:04:05", tc.now)
			require.NoError(t, err)
			issued, err := time.Parse("2006-01-02 15:04:05", tc.issued)
			require.NoError(t, err)
			wantMin, err := time.Parse("2006-01-02 15:04:05", tc.wantMin)
			require.NoError(t, err)
			wantMax, err := time.Parse("2006-01-02 15:04:05", tc.wantMax)
			require.NoError(t, err)

			min, max := calculateSoftExpiry(now, &structs.IssuedCert{
				ValidAfter:  issued,
				ValidBefore: issued.Add(tc.lifetime),
			})

			require.Equal(t, wantMin, min)
			require.Equal(t, wantMax, max)
		})
	}
}

// Test that after an initial signing, new CA roots (new ID) will
// trigger a blocking query to execute.
func TestConnectCALeaf_changingRoots(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	if testingRace {
		t.Skip("fails with -race because caRoot.Active is modified concurrently")
	}
	t.Parallel()

	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// We need this later but needs to be defined so we sign second CSR with it
	// otherwise we break the cert root checking.
	caRoot2 := connect.TestCA(t, nil)

	// Instrument ConnectCA.Sign to return signed cert
	var resp *structs.IssuedCert
	var idx uint64

	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			ca := caRoot
			cIdx := atomic.AddUint64(&idx, 1)
			if cIdx > 1 {
				// Second time round use the new CA
				ca = caRoot2
			}
			reply := args.Get(3).(*structs.IssuedCert)
			leaf, _ := connect.TestLeaf(t, "web", ca)
			reply.CertPEM = leaf
			reply.ValidAfter = time.Now().Add(-1 * time.Hour)
			reply.ValidBefore = time.Now().Add(11 * time.Hour)
			reply.CreateIndex = cIdx
			reply.ModifyIndex = reply.CreateIndex
			resp = reply
		})

	// We'll reuse the fetch options and request
	opts := cache.FetchOptions{MinIndex: 0, Timeout: 10 * time.Second}
	req := &ConnectCALeafRequest{Datacenter: "dc1", Service: "web"}

	// First fetch should return immediately
	fetchCh := TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		require.Equal(t, uint64(1), v.Index)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
	}

	// Second fetch should block with set index
	opts.MinIndex = 1
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case result := <-fetchCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}

	// Let's send in new roots, which should trigger the sign req. We need to take
	// care to set the new root as active
	caRoot2.Active = true
	caRoot.Active = false
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot2.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot2,
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: atomic.AddUint64(&idx, 1)},
	}
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		// 3 since the second CA "update" used up 2
		require.Equal(t, uint64(3), v.Index)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
		opts.MinIndex = 3
	}

	// Third fetch should block
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case result := <-fetchCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}
}

// Tests that if the root change jitter is longer than the time left on the
// timeout, we return normally but then still renew the cert on a subsequent
// call.
func TestConnectCALeaf_changingRootsJitterBetweenCalls(t *testing.T) {
	t.Parallel()

	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	// Override the root-change delay so we will timeout first. We can't set it to
	// a crazy high value otherwise we'll have to wait that long in the test to
	// see if it actually happens on subsequent calls. We instead reduce the
	// timeout in FetchOptions to be much shorter than this.
	typ.TestOverrideCAChangeInitialDelay = 100 * time.Millisecond

	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to return signed cert
	var resp *structs.IssuedCert
	var idx uint64
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(3).(*structs.IssuedCert)
			leaf, _ := connect.TestLeaf(t, "web", caRoot)
			reply.CertPEM = leaf
			reply.ValidAfter = time.Now().Add(-1 * time.Hour)
			reply.ValidBefore = time.Now().Add(11 * time.Hour)
			reply.CreateIndex = atomic.AddUint64(&idx, 1)
			reply.ModifyIndex = reply.CreateIndex
			resp = reply
		})

	// We'll reuse the fetch options and request. Timeout must be much shorter
	// than the initial root delay. 20ms means that if we deliver the root change
	// during the first blocking call, we should need to block fully for 5 more
	// calls before the cert is renewed. We pick a timeout that is not an exact
	// multiple of the 100ms delay above to reduce the chance that timing works
	// out in a way that makes it hard to tell a timeout from an early return due
	// to a cert renewal.
	opts := cache.FetchOptions{MinIndex: 0, Timeout: 35 * time.Millisecond}
	req := &ConnectCALeafRequest{Datacenter: "dc1", Service: "web"}

	// First fetch should return immediately
	fetchCh := TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		require.Equal(t, uint64(1), v.Index)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
	}

	// Let's send in new roots, which should eventually trigger the sign req. We
	// need to take care to set the new root as active. Note that this is
	// implicitly testing that root updates that happen in between leaf blocking
	// queries are still noticed too. At this point no leaf blocking query is
	// running so the root watch should be stopped. By pushing this update, the
	// next blocking query will _immediately_ see the new root which means it
	// needs to correctly notice that it is not the same one that generated the
	// current cert and start the rotation. This is good, just not obvious that
	// the behavior is actually well tested here when it is.
	caRoot2 := connect.TestCA(t, nil)
	caRoot2.Active = true
	caRoot.Active = false
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot2.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot2,
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: atomic.AddUint64(&idx, 1)},
	}
	earliestRootDelivery := time.Now()

	// Some number of fetches (2,3,4 likely) should timeout after 20ms and after
	// 100ms has elapsed total we should see the new cert. Since this is all very
	// timing dependent, we don't hard code exact numbers here and instead loop
	// for plenty of time and do as many calls as it takes and just assert on the
	// time taken and that the call either blocks and returns the cached cert, or
	// returns the new one.
	opts.MinIndex = 1
	var shouldExpireAfter time.Time
	i := 1
	rootsDelivered := false
	for rootsDelivered {
		start := time.Now()
		fetchCh = TestFetchCh(t, typ, opts, req)
		select {
		case result := <-fetchCh:
			v := mustFetchResult(t, result)
			timeTaken := time.Since(start)

			// There are two options, either it blocked waiting for the delay after
			// the rotation or it returned the new CA cert before the timeout was
			// done. TO be more robust against timing, we take the value as the
			// decider for which case it is, and assert timing matches our expected
			// bounds rather than vice versa.

			if v.Index > uint64(1) {
				// Got a new cert
				require.Equal(t, resp, v.Value)
				require.Equal(t, uint64(3), v.Index)
				// Should not have been delivered before the delay
				require.True(t, time.Since(earliestRootDelivery) > typ.TestOverrideCAChangeInitialDelay)
				// All good. We are done!
				rootsDelivered = true
			} else {
				// Should be the cached cert
				require.Equal(t, resp, v.Value)
				require.Equal(t, uint64(1), v.Index)
				// Sanity check we blocked for the whole timeout
				require.Truef(t, timeTaken > opts.Timeout,
					"should block for at least %s, returned after %s",
					opts.Timeout, timeTaken)
				// Sanity check that the forceExpireAfter state was set correctly
				shouldExpireAfter = v.State.(*fetchState).forceExpireAfter
				require.True(t, shouldExpireAfter.After(time.Now()))
				require.True(t, shouldExpireAfter.Before(time.Now().Add(typ.TestOverrideCAChangeInitialDelay)))
			}
			// Set the LastResult for subsequent fetches
			opts.LastResult = &v
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("request %d blocked too long", i)
		}
		i++

		// Sanity check that we've not gone way beyond the deadline without a
		// new cert. We give some leeway to make it less brittle.
		require.Falsef(t, time.Now().After(shouldExpireAfter.Add(100*time.Millisecond)),
			"waited extra 100ms and delayed CA rotate renew didn't happen")
	}
}

// Tests that if the root changes in between blocking calls we still pick it up.
func TestConnectCALeaf_changingRootsBetweenBlockingCalls(t *testing.T) {
	t.Parallel()

	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to return signed cert
	var resp *structs.IssuedCert
	var idx uint64
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(3).(*structs.IssuedCert)
			leaf, _ := connect.TestLeaf(t, "web", caRoot)
			reply.CertPEM = leaf
			reply.ValidAfter = time.Now().Add(-1 * time.Hour)
			reply.ValidBefore = time.Now().Add(11 * time.Hour)
			reply.CreateIndex = atomic.AddUint64(&idx, 1)
			reply.ModifyIndex = reply.CreateIndex
			resp = reply
		})

	// We'll reuse the fetch options and request. Short timeout important since we
	// wait the full timeout before chaning roots.
	opts := cache.FetchOptions{MinIndex: 0, Timeout: 35 * time.Millisecond}
	req := &ConnectCALeafRequest{Datacenter: "dc1", Service: "web"}

	// First fetch should return immediately
	fetchCh := TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		require.Equal(t, uint64(1), v.Index)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
	}

	// Next fetch should block for the full timeout
	start := time.Now()
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block for too long waiting for fetch")
	case result := <-fetchCh:
		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		// Still the initial cached result
		require.Equal(t, uint64(1), v.Index)
		// Sanity check that it waited
		require.True(t, time.Since(start) > opts.Timeout)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
	}

	// No active requests, simulate root change now
	caRoot2 := connect.TestCA(t, nil)
	caRoot2.Active = true
	caRoot.Active = false
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot2.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot2,
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: atomic.AddUint64(&idx, 1)},
	}
	earliestRootDelivery := time.Now()

	// We should get the new cert immediately on next fetch (since test override
	// root change jitter to be 1 nanosecond so no delay expected).
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block too long waiting for fetch")
	case result := <-fetchCh:
		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		// Index should be 3 since root change consumed 2
		require.Equal(t, uint64(3), v.Index)
		// Sanity check that we didn't wait too long
		require.True(t, time.Since(earliestRootDelivery) < opts.Timeout)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
	}

}

func TestConnectCALeaf_CSRRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	// Each jitter window will be only 100 ms long to make testing quick but
	// highly likely not to fail based on scheduling issues.
	typ.TestOverrideCAChangeInitialDelay = 100 * time.Millisecond

	// Setup root that will be returned by the mocked Root cache fetch
	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign
	var resp *structs.IssuedCert
	var idx, rateLimitedRPCs uint64

	genCert := func(args mock.Arguments) {
		reply := args.Get(3).(*structs.IssuedCert)
		leaf, _ := connect.TestLeaf(t, "web", caRoot)
		reply.CertPEM = leaf
		reply.ValidAfter = time.Now().Add(-1 * time.Hour)
		reply.ValidBefore = time.Now().Add(11 * time.Hour)
		reply.CreateIndex = atomic.AddUint64(&idx, 1)
		reply.ModifyIndex = reply.CreateIndex
		resp = reply
	}

	incRateLimit := func(args mock.Arguments) {
		atomic.AddUint64(&rateLimitedRPCs, 1)
	}

	// First call return rate limit error. This is important as it checks
	// behavior when cache is empty and we have to return a nil Value but need to
	// save state to do the right thing for retry.
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).
		Return(consul.ErrRateLimited).Once().Run(incRateLimit)
	// Then succeed on second call
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).
		Return(nil).Run(genCert).Once()
	// Then be rate limited again on several further calls
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).
		Return(consul.ErrRateLimited).Twice().Run(incRateLimit)
	// Then fine after that
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).
		Return(nil).Run(genCert)

	opts := cache.FetchOptions{MinIndex: 0, Timeout: 10 * time.Minute}
	req := &ConnectCALeafRequest{Datacenter: "dc1", Service: "web"}

	// First fetch should return rate limit error directly - client is expected to
	// backoff itself.
	fetchCh := TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shouldn't block longer than one jitter window for success")
	case result := <-fetchCh:
		switch v := result.(type) {
		case error:
			require.Error(t, v)
			require.Equal(t, consul.ErrRateLimited.Error(), v.Error())
		case cache.FetchResult:
			t.Fatalf("Expected error")
		}
	}

	// Second call should return correct cert immediately.
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		require.Equal(t, uint64(1), v.Index)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
		// Set MinIndex
		opts.MinIndex = 1
	}

	// Send in new roots, which should trigger the next sign req. We need to take
	// care to set the new root as active
	caRoot2 := connect.TestCA(t, nil)
	caRoot2.Active = true
	caRoot.Active = false
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot2.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot2,
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: atomic.AddUint64(&idx, 1)},
	}
	earliestRootDelivery := time.Now()

	// Sanity check state
	require.Equal(t, uint64(1), atomic.LoadUint64(&rateLimitedRPCs))

	// After root rotation jitter has been waited out, a new CSR will
	// be attempted but will fail and return the previous cached result with no
	// error since we will try again soon.
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shouldn't block too long waiting for fetch")
	case result := <-fetchCh:
		// We should block for _at least_ one jitter period since we set that to
		// 100ms and in test override mode we always pick the max jitter not a
		// random amount.
		require.True(t, time.Since(earliestRootDelivery) > 100*time.Millisecond)
		require.Equal(t, uint64(2), atomic.LoadUint64(&rateLimitedRPCs))

		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		// 1 since this should still be the original cached result as we failed to
		// get a new cert.
		require.Equal(t, uint64(1), v.Index)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
	}

	// Root rotation state is now only captured in the opts.LastResult.State so a
	// subsequent call should also wait for 100ms and then attempt to generate a
	// new cert since we failed last time.
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shouldn't block too long waiting for fetch")
	case result := <-fetchCh:
		// We should block for _at least_ two jitter periods now.
		require.True(t, time.Since(earliestRootDelivery) > 200*time.Millisecond)
		require.Equal(t, uint64(3), atomic.LoadUint64(&rateLimitedRPCs))

		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		// 1 since this should still be the original cached result as we failed to
		// get a new cert.
		require.Equal(t, uint64(1), v.Index)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
	}

	// Now we've had two rate limit failures and seen root rotation state work
	// across both the blocking request that observed the rotation and the
	// subsequent one. The next request should wait out the rest of the backoff
	// and then actually fetch a new cert at last!
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shouldn't block too long waiting for fetch")
	case result := <-fetchCh:
		// We should block for _at least_ three jitter periods now.
		require.True(t, time.Since(earliestRootDelivery) > 300*time.Millisecond)
		require.Equal(t, uint64(3), atomic.LoadUint64(&rateLimitedRPCs))

		v := mustFetchResult(t, result)
		require.Equal(t, resp, v.Value)
		// 3 since the rootCA change used 2
		require.Equal(t, uint64(3), v.Index)
		// Set the LastResult for subsequent fetches
		opts.LastResult = &v
	}
}

// This test runs multiple concurrent callers watching different leaf certs and
// tries to ensure that the background root watch activity behaves correctly.
func TestConnectCALeaf_watchRootsDedupingMultipleCallers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	if testingRace {
		t.Skip("fails with -race because caRoot.Active is modified concurrently")
	}
	t.Parallel()

	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to return signed cert
	var idx uint64
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(3).(*structs.IssuedCert)
			// Note we will sign certs for same service name each time because
			// otherwise we have to re-invent whole CSR endpoint here to be able to
			// control things - parse PEM sign with right key etc. It doesn't matter -
			// we use the CreateIndex to differentiate the "right" results.
			leaf, _ := connect.TestLeaf(t, "web", caRoot)
			reply.CertPEM = leaf
			reply.ValidAfter = time.Now().Add(-1 * time.Hour)
			reply.ValidBefore = time.Now().Add(11 * time.Hour)
			reply.CreateIndex = atomic.AddUint64(&idx, 1)
			reply.ModifyIndex = reply.CreateIndex
		})

	// n is the number of clients we'll run
	n := 3

	// setup/testDoneCh are used for coordinating clients such that each has
	// initial cert delivered and is blocking before the root changes. It's not a
	// wait group since we want to be able to timeout the main test goroutine if
	// one of the clients gets stuck. Instead it's a buffered chan.
	setupDoneCh := make(chan error, n)
	testDoneCh := make(chan error, n)
	// rootsUpdate is used to coordinate clients so they know when they should
	// expect to see leaf renewed after root change.
	rootsUpdatedCh := make(chan struct{})

	// Create a function that models a single client. It should go through the
	// steps of getting an initial cert and then watching for changes until root
	// updates.
	client := func(i int) {
		// We'll reuse the fetch options and request
		opts := cache.FetchOptions{MinIndex: 0, Timeout: 10 * time.Second}
		req := &ConnectCALeafRequest{Datacenter: "dc1", Service: fmt.Sprintf("web-%d", i)}

		// First fetch should return immediately
		fetchCh := TestFetchCh(t, typ, opts, req)
		select {
		case <-time.After(100 * time.Millisecond):
			setupDoneCh <- fmt.Errorf("shouldn't block waiting for fetch")
			return
		case result := <-fetchCh:
			v := mustFetchResult(t, result)
			opts.LastResult = &v
		}

		// Second fetch should block with set index
		opts.MinIndex = 1
		fetchCh = TestFetchCh(t, typ, opts, req)
		select {
		case result := <-fetchCh:
			setupDoneCh <- fmt.Errorf("should not return: %#v", result)
			return
		case <-time.After(100 * time.Millisecond):
		}

		// We're done with setup and the blocking call is still blocking in
		// background.
		setupDoneCh <- nil

		// Wait until all others are also done and roots change incase there are
		// stragglers delaying the root update.
		select {
		case <-rootsUpdatedCh:
		case <-time.After(200 * time.Millisecond):
			testDoneCh <- fmt.Errorf("waited too long for root update")
			return
		}

		// Now we should see root update within a short period
		select {
		case <-time.After(100 * time.Millisecond):
			testDoneCh <- fmt.Errorf("shouldn't block waiting for fetch")
			return
		case result := <-fetchCh:
			v := mustFetchResult(t, result)
			if opts.MinIndex == v.Value.(*structs.IssuedCert).CreateIndex {
				testDoneCh <- fmt.Errorf("index must be different")
				return
			}
		}

		testDoneCh <- nil
	}

	// Sanity check the roots watcher is not running yet
	assertRootsWatchCounts(t, typ, 0, 0)

	for i := 0; i < n; i++ {
		go client(i)
	}

	timeoutCh := time.After(200 * time.Millisecond)

	for i := 0; i < n; i++ {
		select {
		case <-timeoutCh:
			t.Fatal("timed out waiting for clients")
		case err := <-setupDoneCh:
			if err != nil {
				t.Fatalf(err.Error())
			}
		}
	}

	// Should be 3 clients running now, so the roots watcher should have started
	// once and not stopped.
	assertRootsWatchCounts(t, typ, 1, 0)

	// Now we deliver the root update
	caRoot2 := connect.TestCA(t, nil)
	caRoot2.Active = true
	caRoot.Active = false
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot2.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot2,
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: atomic.AddUint64(&idx, 1)},
	}
	// And notify clients
	close(rootsUpdatedCh)

	timeoutCh = time.After(200 * time.Millisecond)
	for i := 0; i < n; i++ {
		select {
		case <-timeoutCh:
			t.Fatalf("timed out waiting for %d of %d clients to renew after root change", n-i, n)
		case err := <-testDoneCh:
			if err != nil {
				t.Fatalf(err.Error())
			}
		}
	}

	// All active requests have returned the new cert so the rootsWatcher should
	// have stopped. This is timing dependent though so retry a few times
	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		assertRootsWatchCounts(r, typ, 1, 1)
	})
}

func assertRootsWatchCounts(t require.TestingT, typ *ConnectCALeaf, wantStarts, wantStops int) {
	if tt, ok := t.(*testing.T); ok {
		tt.Helper()
	}
	starts := atomic.LoadUint32(&typ.testRootWatchStartCount)
	stops := atomic.LoadUint32(&typ.testRootWatchStopCount)
	require.Equal(t, wantStarts, int(starts))
	require.Equal(t, wantStops, int(stops))
}

func mustFetchResult(t *testing.T, result interface{}) cache.FetchResult {
	t.Helper()
	switch v := result.(type) {
	case error:
		require.NoError(t, v)
	case cache.FetchResult:
		return v
	default:
		t.Fatalf("unexpected type from fetch %T", v)
	}
	return cache.FetchResult{}
}

// Test that after an initial signing, an expiringLeaf will trigger a
// blocking query to resign.
func TestConnectCALeaf_expiringLeaf(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to
	var resp *structs.IssuedCert
	var idx uint64
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(3).(*structs.IssuedCert)
			reply.CreateIndex = atomic.AddUint64(&idx, 1)
			reply.ModifyIndex = reply.CreateIndex

			leaf, _ := connect.TestLeaf(t, "web", caRoot)
			reply.CertPEM = leaf

			if reply.CreateIndex == 1 {
				// First call returns expired cert to prime cache with an expired one.
				reply.ValidAfter = time.Now().Add(-13 * time.Hour)
				reply.ValidBefore = time.Now().Add(-1 * time.Hour)
			} else {
				reply.ValidAfter = time.Now().Add(-1 * time.Hour)
				reply.ValidBefore = time.Now().Add(11 * time.Hour)
			}

			resp = reply
		})

	// We'll reuse the fetch options and request
	opts := cache.FetchOptions{MinIndex: 0, Timeout: 10 * time.Second}
	req := &ConnectCALeafRequest{Datacenter: "dc1", Service: "web"}

	// First fetch should return immediately
	fetchCh := TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		switch v := result.(type) {
		case error:
			require.NoError(t, v)
		case cache.FetchResult:
			require.Equal(t, resp, v.Value)
			require.Equal(t, uint64(1), v.Index)
			// Set the LastResult for subsequent fetches
			opts.LastResult = &v
		}
	}

	// Second fetch should return immediately despite there being
	// no updated CA roots, because we issued an expired cert.
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-fetchCh:
		switch v := result.(type) {
		case error:
			require.NoError(t, v)
		case cache.FetchResult:
			require.Equal(t, resp, v.Value)
			require.Equal(t, uint64(2), v.Index)
			// Set the LastResult for subsequent fetches
			opts.LastResult = &v
		}
	}

	// Third fetch should block since the cert is not expiring and
	// we also didn't update CA certs.
	opts.MinIndex = 2
	fetchCh = TestFetchCh(t, typ, opts, req)
	select {
	case result := <-fetchCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestConnectCALeaf_DNSSANForService(t *testing.T) {
	t.Parallel()

	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)

	typ, rootsCh := testCALeafType(t, rpc)
	defer close(rootsCh)

	caRoot := connect.TestCA(t, nil)
	caRoot.Active = true
	rootsCh <- structs.IndexedCARoots{
		ActiveRootID: caRoot.ID,
		TrustDomain:  "fake-trust-domain.consul",
		Roots: []*structs.CARoot{
			caRoot,
		},
		QueryMeta: structs.QueryMeta{Index: 1},
	}

	// Instrument ConnectCA.Sign to
	var caReq *structs.CASignRequest
	rpc.On("RPC", mock.Anything, "ConnectCA.Sign", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			reply := args.Get(3).(*structs.IssuedCert)
			leaf, _ := connect.TestLeaf(t, "web", caRoot)
			reply.CertPEM = leaf

			caReq = args.Get(2).(*structs.CASignRequest)
		})

	opts := cache.FetchOptions{MinIndex: 0, Timeout: 10 * time.Second}
	req := &ConnectCALeafRequest{
		Datacenter: "dc1",
		Service:    "web",
		DNSSAN:     []string{"test.example.com"},
	}
	_, err := typ.Fetch(opts, req)
	require.NoError(t, err)

	pemBlock, _ := pem.Decode([]byte(caReq.CSR))
	csr, err := x509.ParseCertificateRequest(pemBlock.Bytes)
	require.NoError(t, err)
	require.Equal(t, csr.DNSNames, []string{"test.example.com"})
}

// testConnectCaRoot wraps ConnectCARoot to disable refresh so that the gated
// channel controls the request directly. Otherwise, we get background refreshes and
// it screws up the ordering of the channel reads of the testGatedRootsRPC
// implementation.
type testConnectCaRoot struct {
	ConnectCARoot
}

func (r testConnectCaRoot) RegisterOptions() cache.RegisterOptions {
	return cache.RegisterOptions{
		Refresh:          false,
		SupportsBlocking: true,
	}
}

// testCALeafType returns a *ConnectCALeaf that is pre-configured to
// use the given RPC implementation for "ConnectCA.Sign" operations.
func testCALeafType(t *testing.T, rpc RPC) (*ConnectCALeaf, chan structs.IndexedCARoots) {
	// This creates an RPC implementation that will block until the
	// value is sent on the channel. This lets us control when the
	// next values show up.
	rootsCh := make(chan structs.IndexedCARoots, 10)
	rootsRPC := &testGatedRootsRPC{ValueCh: rootsCh}

	// Create a cache
	c := cache.New(cache.Options{})
	c.RegisterType(ConnectCARootName, &testConnectCaRoot{
		ConnectCARoot: ConnectCARoot{RPC: rootsRPC},
	})
	// Create the leaf type
	return &ConnectCALeaf{
		RPC:        rpc,
		Cache:      c,
		Datacenter: "dc1",
		// Override the root-change spread so we don't have to wait up to 20 seconds
		// to see root changes work. Can be changed back for specific tests that
		// need to test this, Note it's not 0 since that used default but is
		// effectively the same.
		TestOverrideCAChangeInitialDelay: 1 * time.Microsecond,
	}, rootsCh
}

// testGatedRootsRPC will send each subsequent value on the channel as the
// RPC response, blocking if it is waiting for a value on the channel. This
// can be used to control when background fetches are returned and what they
// return.
//
// This should be used with Refresh = false for the registration options so
// automatic refreshes don't mess up the channel read ordering.
type testGatedRootsRPC struct {
	ValueCh chan structs.IndexedCARoots
}

func (r *testGatedRootsRPC) RPC(ctx context.Context, method string, args interface{}, reply interface{}) error {
	if method != "ConnectCA.Roots" {
		return fmt.Errorf("invalid RPC method: %s", method)
	}

	replyReal := reply.(*structs.IndexedCARoots)
	*replyReal = <-r.ValueCh
	return nil
}

func TestConnectCALeaf_Key(t *testing.T) {
	key := func(r ConnectCALeafRequest) string {
		return r.Key()
	}
	t.Run("service", func(t *testing.T) {
		t.Run("name", func(t *testing.T) {
			r1 := key(ConnectCALeafRequest{Service: "web"})
			r2 := key(ConnectCALeafRequest{Service: "api"})
			require.True(t, strings.HasPrefix(r1, "service:"), "Key %s does not start with service:", r1)
			require.True(t, strings.HasPrefix(r2, "service:"), "Key %s does not start with service:", r2)
			require.NotEqual(t, r1, r2, "Cache keys for different services should not be equal")
		})
		t.Run("dns-san", func(t *testing.T) {
			r3 := key(ConnectCALeafRequest{Service: "foo", DNSSAN: []string{"a.com"}})
			r4 := key(ConnectCALeafRequest{Service: "foo", DNSSAN: []string{"b.com"}})
			require.NotEqual(t, r3, r4, "Cache keys for different DNSSAN should not be equal")
		})
		t.Run("ip-san", func(t *testing.T) {
			r5 := key(ConnectCALeafRequest{Service: "foo", IPSAN: []net.IP{net.ParseIP("192.168.4.139")}})
			r6 := key(ConnectCALeafRequest{Service: "foo", IPSAN: []net.IP{net.ParseIP("192.168.4.140")}})
			require.NotEqual(t, r5, r6, "Cache keys for different IPSAN should not be equal")
		})
	})
	t.Run("agent", func(t *testing.T) {
		t.Run("name", func(t *testing.T) {
			r1 := key(ConnectCALeafRequest{Agent: "abc"})
			require.True(t, strings.HasPrefix(r1, "agent:"), "Key %s does not start with agent:", r1)
		})
		t.Run("dns-san ignored", func(t *testing.T) {
			r3 := key(ConnectCALeafRequest{Agent: "foo", DNSSAN: []string{"a.com"}})
			r4 := key(ConnectCALeafRequest{Agent: "foo", DNSSAN: []string{"b.com"}})
			require.Equal(t, r3, r4, "DNSSAN is ignored for agent type")
		})
		t.Run("ip-san ignored", func(t *testing.T) {
			r5 := key(ConnectCALeafRequest{Agent: "foo", IPSAN: []net.IP{net.ParseIP("192.168.4.139")}})
			r6 := key(ConnectCALeafRequest{Agent: "foo", IPSAN: []net.IP{net.ParseIP("192.168.4.140")}})
			require.Equal(t, r5, r6, "IPSAN is ignored for agent type")
		})
	})
	t.Run("kind", func(t *testing.T) {
		t.Run("invalid", func(t *testing.T) {
			r1 := key(ConnectCALeafRequest{Kind: "terminating-gateway"})
			require.Empty(t, r1)
		})
		t.Run("mesh-gateway", func(t *testing.T) {
			t.Run("normal", func(t *testing.T) {
				r1 := key(ConnectCALeafRequest{Kind: "mesh-gateway"})
				require.True(t, strings.HasPrefix(r1, "kind:"), "Key %s does not start with kind:", r1)
			})
			t.Run("dns-san", func(t *testing.T) {
				r3 := key(ConnectCALeafRequest{Kind: "mesh-gateway", DNSSAN: []string{"a.com"}})
				r4 := key(ConnectCALeafRequest{Kind: "mesh-gateway", DNSSAN: []string{"b.com"}})
				require.NotEqual(t, r3, r4, "Cache keys for different DNSSAN should not be equal")
			})
			t.Run("ip-san", func(t *testing.T) {
				r5 := key(ConnectCALeafRequest{Kind: "mesh-gateway", IPSAN: []net.IP{net.ParseIP("192.168.4.139")}})
				r6 := key(ConnectCALeafRequest{Kind: "mesh-gateway", IPSAN: []net.IP{net.ParseIP("192.168.4.140")}})
				require.NotEqual(t, r5, r6, "Cache keys for different IPSAN should not be equal")
			})
		})
	})
	t.Run("server", func(t *testing.T) {
		r1 := key(ConnectCALeafRequest{
			Server:     true,
			Datacenter: "us-east",
		})
		require.True(t, strings.HasPrefix(r1, "server:"), "Key %s does not start with server:", r1)
	})
}
