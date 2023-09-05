package leafcert

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// Test that after an initial signing, new CA roots (new ID) will
// trigger a blocking query to execute.
func TestManager_changingRoots(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	m, signer := testManager(t, nil)

	caRoot := signer.UpdateCA(t, nil)

	// We'll reuse the fetch options and request
	req := &ConnectCALeafRequest{
		Datacenter: "dc1", Service: "web",
		MinQueryIndex: 0, MaxQueryTime: 10 * time.Second,
	}

	// First fetch should return immediately
	getCh := testAsyncGet(t, m, req)
	var idx uint64
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.NotNil(t, result.Value)
		requireLeafValidUnderCA(t, result.Value, caRoot)
		require.True(t, result.Index > 0)

		idx = result.Index
	}

	// Second fetch should block with set index
	req.MinQueryIndex = idx
	getCh = testAsyncGet(t, m, req)
	select {
	case result := <-getCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}

	// Let's send in new roots, which should trigger the sign req. We need to take
	// care to set the new root as active
	caRoot2 := signer.UpdateCA(t, nil)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.NotNil(t, result.Value)
		require.True(t, result.Index > idx)
		requireLeafValidUnderCA(t, result.Value, caRoot2)
	}

	// Third fetch should block
	getCh = testAsyncGet(t, m, req)
	select {
	case result := <-getCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}
}

// Tests that if the root change jitter is longer than the time left on the
// timeout, we return normally but then still renew the cert on a subsequent
// call.
func TestManager_changingRootsJitterBetweenCalls(t *testing.T) {
	t.Parallel()

	const TestOverrideCAChangeInitialDelay = 100 * time.Millisecond

	m, signer := testManager(t, func(cfg *Config) {
		// Override the root-change delay so we will timeout first. We can't set it to
		// a crazy high value otherwise we'll have to wait that long in the test to
		// see if it actually happens on subsequent calls. We instead reduce the
		// timeout in FetchOptions to be much shorter than this.
		cfg.TestOverrideCAChangeInitialDelay = TestOverrideCAChangeInitialDelay
	})

	caRoot := signer.UpdateCA(t, nil)

	// We'll reuse the fetch options and request. Timeout must be much shorter
	// than the initial root delay. 20ms means that if we deliver the root change
	// during the first blocking call, we should need to block fully for 5 more
	// calls before the cert is renewed. We pick a timeout that is not an exact
	// multiple of the 100ms delay above to reduce the chance that timing works
	// out in a way that makes it hard to tell a timeout from an early return due
	// to a cert renewal.
	req := &ConnectCALeafRequest{
		Datacenter: "dc1", Service: "web",
		MinQueryIndex: 0, MaxQueryTime: 35 * time.Millisecond,
	}

	// First fetch should return immediately
	getCh := testAsyncGet(t, m, req)
	var (
		idx    uint64
		issued *structs.IssuedCert
	)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.NotNil(t, result.Value)
		require.True(t, result.Index > 0)
		requireLeafValidUnderCA(t, result.Value, caRoot)
		idx = result.Index
		issued = result.Value
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
	caRoot2 := signer.UpdateCA(t, nil)
	earliestRootDelivery := time.Now()

	// Some number of fetches (2,3,4 likely) should timeout after 20ms and after
	// 100ms has elapsed total we should see the new cert. Since this is all very
	// timing dependent, we don't hard code exact numbers here and instead loop
	// for plenty of time and do as many calls as it takes and just assert on the
	// time taken and that the call either blocks and returns the cached cert, or
	// returns the new one.
	req.MinQueryIndex = idx
	var shouldExpireAfter time.Time
	i := 1
	rootsDelivered := false
	for rootsDelivered {
		start := time.Now()
		getCh = testAsyncGet(t, m, req)
		select {
		case result := <-getCh:
			require.NoError(t, result.Err)
			timeTaken := time.Since(start)

			// There are two options, either it blocked waiting for the delay after
			// the rotation or it returned the new CA cert before the timeout was
			// done. TO be more robust against timing, we take the value as the
			// decider for which case it is, and assert timing matches our expected
			// bounds rather than vice versa.

			if result.Index > idx {
				// Got a new cert
				require.NotEqual(t, issued, result.Value)
				require.NotNil(t, result.Value)
				requireLeafValidUnderCA(t, result.Value, caRoot2)
				// Should not have been delivered before the delay
				require.True(t, time.Since(earliestRootDelivery) > TestOverrideCAChangeInitialDelay)
				// All good. We are done!
				rootsDelivered = true
			} else {
				// Should be the cached cert
				require.Equal(t, issued, result.Value)
				require.Equal(t, idx, result.Index)
				requireLeafValidUnderCA(t, result.Value, caRoot)
				// Sanity check we blocked for the whole timeout
				require.Truef(t, timeTaken > req.MaxQueryTime,
					"should block for at least %s, returned after %s",
					req.MaxQueryTime, timeTaken)
				// Sanity check that the forceExpireAfter state was set correctly
				shouldExpireAfter := testObserveLeafCert(m, req, func(cd *certData) time.Time {
					return cd.state.forceExpireAfter
				})
				require.True(t, shouldExpireAfter.After(time.Now()))
				require.True(t, shouldExpireAfter.Before(time.Now().Add(TestOverrideCAChangeInitialDelay)))
			}
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

func testObserveLeafCert[T any](m *Manager, req *ConnectCALeafRequest, cb func(*certData) T) T {
	key := req.Key()

	cd := m.getCertData(key)

	cd.lock.Lock()
	defer cd.lock.Unlock()

	return cb(cd)
}

// Tests that if the root changes in between blocking calls we still pick it up.
func TestManager_changingRootsBetweenBlockingCalls(t *testing.T) {
	t.Parallel()

	m, signer := testManager(t, nil)

	caRoot := signer.UpdateCA(t, nil)

	// We'll reuse the fetch options and request. Short timeout important since we
	// wait the full timeout before chaning roots.
	req := &ConnectCALeafRequest{
		Datacenter: "dc1", Service: "web",
		MinQueryIndex: 0, MaxQueryTime: 35 * time.Millisecond,
	}

	// First fetch should return immediately
	getCh := testAsyncGet(t, m, req)
	var (
		idx    uint64
		issued *structs.IssuedCert
	)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.NotNil(t, result.Value)
		requireLeafValidUnderCA(t, result.Value, caRoot)
		require.True(t, result.Index > 0)
		idx = result.Index
		issued = result.Value
	}

	// Next fetch should block for the full timeout
	start := time.Now()
	getCh = testAsyncGet(t, m, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block for too long waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.Equal(t, issued, result.Value)
		// Still the initial cached result
		require.Equal(t, idx, result.Index)
		// Sanity check that it waited
		require.True(t, time.Since(start) > req.MaxQueryTime)
	}

	// No active requests, simulate root change now
	caRoot2 := signer.UpdateCA(t, nil)
	earliestRootDelivery := time.Now()

	// We should get the new cert immediately on next fetch (since test override
	// root change jitter to be 1 nanosecond so no delay expected).
	getCh = testAsyncGet(t, m, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block too long waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.NotEqual(t, issued, result.Value)
		requireLeafValidUnderCA(t, result.Value, caRoot2)
		require.True(t, result.Index > idx)
		// Sanity check that we didn't wait too long
		require.True(t, time.Since(earliestRootDelivery) < req.MaxQueryTime)
	}
}

func TestManager_CSRRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	m, signer := testManager(t, func(cfg *Config) {
		// Each jitter window will be only 100 ms long to make testing quick but
		// highly likely not to fail based on scheduling issues.
		cfg.TestOverrideCAChangeInitialDelay = 100 * time.Millisecond
	})

	signer.UpdateCA(t, nil)

	signer.SetSignCallErrors(
		// First call return rate limit error. This is important as it checks
		// behavior when cache is empty and we have to return a nil Value but need to
		// save state to do the right thing for retry.
		consul.ErrRateLimited, // inc
		// Then succeed on second call
		nil,
		// Then be rate limited again on several further calls
		consul.ErrRateLimited, // inc
		consul.ErrRateLimited, // inc
	// Then fine after that
	)

	req := &ConnectCALeafRequest{
		Datacenter:     "dc1",
		Service:        "web",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}

	// First fetch should return rate limit error directly - client is expected to
	// backoff itself.
	getCh := testAsyncGet(t, m, req)
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shouldn't block longer than one jitter window for success")
	case result := <-getCh:
		require.Error(t, result.Err)
		require.Equal(t, consul.ErrRateLimited.Error(), result.Err.Error())
	}

	// Second call should return correct cert immediately.
	getCh = testAsyncGet(t, m, req)
	var (
		idx    uint64
		issued *structs.IssuedCert
	)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.NotNil(t, result.Value)
		require.True(t, result.Index > 0)
		idx = result.Index
		issued = result.Value
	}

	// Send in new roots, which should trigger the next sign req. We need to take
	// care to set the new root as active
	signer.UpdateCA(t, nil)
	earliestRootDelivery := time.Now()

	// Sanity check state
	require.Equal(t, uint64(1), signer.GetSignCallErrorCount())

	// After root rotation jitter has been waited out, a new CSR will
	// be attempted but will fail and return the previous cached result with no
	// error since we will try again soon.
	getCh = testAsyncGet(t, m, req)
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shouldn't block too long waiting for fetch")
	case result := <-getCh:
		// We should block for _at least_ one jitter period since we set that to
		// 100ms and in test override mode we always pick the max jitter not a
		// random amount.
		require.True(t, time.Since(earliestRootDelivery) > 100*time.Millisecond)
		require.Equal(t, uint64(2), signer.GetSignCallErrorCount())

		require.NoError(t, result.Err)
		require.Equal(t, issued, result.Value)
		// 1 since this should still be the original cached result as we failed to
		// get a new cert.
		require.Equal(t, idx, result.Index)
	}

	// Root rotation state is now only captured in the opts.LastResult.State so a
	// subsequent call should also wait for 100ms and then attempt to generate a
	// new cert since we failed last time.
	getCh = testAsyncGet(t, m, req)
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shouldn't block too long waiting for fetch")
	case result := <-getCh:
		// We should block for _at least_ two jitter periods now.
		require.True(t, time.Since(earliestRootDelivery) > 200*time.Millisecond)
		require.Equal(t, uint64(3), signer.GetSignCallErrorCount())

		require.NoError(t, result.Err)
		require.Equal(t, issued, result.Value)
		// 1 since this should still be the original cached result as we failed to
		// get a new cert.
		require.Equal(t, idx, result.Index)
	}

	// Now we've had two rate limit failures and seen root rotation state work
	// across both the blocking request that observed the rotation and the
	// subsequent one. The next request should wait out the rest of the backoff
	// and then actually fetch a new cert at last!
	getCh = testAsyncGet(t, m, req)
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shouldn't block too long waiting for fetch")
	case result := <-getCh:
		// We should block for _at least_ three jitter periods now.
		require.True(t, time.Since(earliestRootDelivery) > 300*time.Millisecond)
		require.Equal(t, uint64(3), signer.GetSignCallErrorCount())

		require.NoError(t, result.Err)
		require.NotEqual(t, issued, result.Value)
		// 3 since the rootCA change used 2
		require.True(t, result.Index > idx)
	}
}

// This test runs multiple concurrent callers watching different leaf certs and
// tries to ensure that the background root watch activity behaves correctly.
func TestManager_watchRootsDedupingMultipleCallers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	m, signer := testManager(t, nil)

	caRoot := signer.UpdateCA(t, nil)

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
		req := &ConnectCALeafRequest{
			Datacenter: "dc1", Service: fmt.Sprintf("web-%d", i),
			MinQueryIndex: 0, MaxQueryTime: 10 * time.Second,
		}

		// First fetch should return immediately
		getCh := testAsyncGet(t, m, req)
		var idx uint64
		select {
		case <-time.After(100 * time.Millisecond):
			setupDoneCh <- fmt.Errorf("shouldn't block waiting for fetch")
			return
		case result := <-getCh:
			require.NoError(t, result.Err)
			idx = result.Index
		}

		// Second fetch should block with set index
		req.MinQueryIndex = idx
		getCh = testAsyncGet(t, m, req)
		select {
		case result := <-getCh:
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
		case result := <-getCh:
			require.NoError(t, result.Err)
			if req.MinQueryIndex == result.Value.CreateIndex {
				testDoneCh <- fmt.Errorf("index must be different")
				return
			}
		}

		testDoneCh <- nil
	}

	// Sanity check the roots watcher is not running yet
	assertRootsWatchCounts(t, m, 0, 0)

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
	assertRootsWatchCounts(t, m, 1, 0)

	caRootCopy := caRoot.Clone()
	caRootCopy.Active = false

	// Now we deliver the root update
	_ = signer.UpdateCA(t, nil)
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
		assertRootsWatchCounts(r, m, 1, 1)
	})
}

func assertRootsWatchCounts(t require.TestingT, m *Manager, wantStarts, wantStops int) {
	if tt, ok := t.(*testing.T); ok {
		tt.Helper()
	}
	starts := atomic.LoadUint32(&m.rootWatcher.testStartCount)
	stops := atomic.LoadUint32(&m.rootWatcher.testStopCount)
	require.Equal(t, wantStarts, int(starts))
	require.Equal(t, wantStops, int(stops))
}

// Test that after an initial signing, an expiringLeaf will trigger a
// blocking query to resign.
func TestManager_expiringLeaf(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	m, signer := testManager(t, nil)

	caRoot := signer.UpdateCA(t, nil)

	signer.SetSignCallErrors(
		// First call returns expired cert to prime cache with an expired one.
		ReplyWithExpiredCert,
	)

	// We'll reuse the fetch options and request
	req := &ConnectCALeafRequest{
		Datacenter: "dc1", Service: "web",
		MinQueryIndex: 0, MaxQueryTime: 10 * time.Second,
	}

	// First fetch should return immediately
	getCh := testAsyncGet(t, m, req)
	var (
		idx    uint64
		issued *structs.IssuedCert
	)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.NotNil(t, result.Value)
		require.True(t, result.Index > 0)
		idx = result.Index
		issued = result.Value
	}

	// Second fetch should return immediately despite there being
	// no updated CA roots, because we issued an expired cert.
	getCh = testAsyncGet(t, m, req)
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shouldn't block waiting for fetch")
	case result := <-getCh:
		require.NoError(t, result.Err)
		require.NotEqual(t, issued, result.Value)
		require.True(t, result.Index > idx)
		requireLeafValidUnderCA(t, result.Value, caRoot)
		idx = result.Index
	}

	// Third fetch should block since the cert is not expiring and
	// we also didn't update CA certs.
	req.MinQueryIndex = idx
	getCh = testAsyncGet(t, m, req)
	select {
	case result := <-getCh:
		t.Fatalf("should not return: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestManager_DNSSANForService(t *testing.T) {
	t.Parallel()

	m, signer := testManager(t, nil)

	_ = signer.UpdateCA(t, nil)

	req := &ConnectCALeafRequest{
		Datacenter: "dc1",
		Service:    "web",
		DNSSAN:     []string{"test.example.com"},
	}

	_, _, err := m.Get(context.Background(), req)
	require.NoError(t, err)

	caReq := signer.GetCapture(0)
	require.NotNil(t, caReq)

	pemBlock, _ := pem.Decode([]byte(caReq.CSR))
	csr, err := x509.ParseCertificateRequest(pemBlock.Bytes)
	require.NoError(t, err)
	require.Equal(t, csr.DNSNames, []string{"test.example.com"})
}

func TestManager_workflow_good(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const TestOverrideCAChangeInitialDelay = 1 * time.Nanosecond

	m, signer := testManager(t, func(cfg *Config) {
		cfg.TestOverrideCAChangeInitialDelay = TestOverrideCAChangeInitialDelay
	})

	ca1 := signer.UpdateCA(t, nil)

	req := &ConnectCALeafRequest{
		Datacenter:     "dc1",
		Service:        "test",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}

	// List
	issued, meta, err := m.Get(ctx, req)
	require.NoError(t, err)
	require.False(t, meta.Hit)
	require.NotNil(t, issued)

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	require.True(t, issued.ModifyIndex > 0)
	require.Equal(t, issued.ModifyIndex, meta.Index)

	index := meta.Index

	// Fetch it again
	testutil.RunStep(t, "test you get a cache hit on another read", func(t *testing.T) {
		req := &ConnectCALeafRequest{
			Datacenter:     "dc1",
			Service:        "test",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		}
		issued2, _, err := m.Get(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, issued2)
		require.Equal(t, issued, issued2)
	})

	type reply struct {
		cert *structs.IssuedCert
		meta cache.ResultMeta
		err  error
	}

	replyCh := make(chan *reply, 1)
	go func() {
		req := &ConnectCALeafRequest{
			Datacenter:     "dc1",
			Service:        "test",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
			MinQueryIndex:  index,
		}

		issued2, meta2, err := m.Get(ctx, req)

		replyCh <- &reply{issued2, meta2, err}
	}()

	// Set a new CA
	ca2 := signer.UpdateCA(t, nil)

	// Issue a blocking query to ensure that the cert gets updated appropriately
	testutil.RunStep(t, "test blocking queries update leaf cert", func(t *testing.T) {
		var got *reply
		select {
		case got = <-replyCh:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("blocking query did not wake up during rotation")
		}

		issued2, meta2, err := got.cert, got.meta, got.err
		require.NoError(t, err)
		require.NotNil(t, issued2)

		require.NotEqual(t, issued.CertPEM, issued2.CertPEM)
		require.NotEqual(t, issued.PrivateKeyPEM, issued2.PrivateKeyPEM)

		// Verify that the cert is signed by the new CA
		requireLeafValidUnderCA(t, issued2, ca2)

		// Should not be a cache hit! The data was updated in response to the blocking
		// query being made.
		require.False(t, meta2.Hit)
	})

	testutil.RunStep(t, "test non-blocking queries update leaf cert", func(t *testing.T) {
		req := &ConnectCALeafRequest{
			Datacenter:     "dc1",
			Service:        "test",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		}

		issued, _, err := m.Get(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, issued)

		// Verify that the cert is signed by the CA
		requireLeafValidUnderCA(t, issued, ca2)

		// Issue a non blocking query to ensure that the cert gets updated appropriately
		{
			// Set a new CA
			ca3 := signer.UpdateCA(t, nil)

			retry.Run(t, func(r *retry.R) {
				req := &ConnectCALeafRequest{
					Datacenter:     "dc1",
					Service:        "test",
					EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
				}

				issued2, meta2, err := m.Get(ctx, req)
				require.NoError(r, err)
				require.NotNil(r, issued2)

				requireLeafValidUnderCA(r, issued2, ca3)

				// Should not be a cache hit!
				require.False(r, meta2.Hit)

				require.NotEqual(r, issued.CertPEM, issued2.CertPEM)
				require.NotEqual(r, issued.PrivateKeyPEM, issued2.PrivateKeyPEM)

				// Verify that the cert is signed by the new CA
				requireLeafValidUnderCA(r, issued2, ca3)
			})
		}
	})
}

// Test we can request a leaf cert for a service and witness correct caching,
// blocking, and update semantics.
//
// This test originally was a client agent test in
// agent.TestAgentConnectCALeafCert_goodNotLocal and was cloned here to
// increase complex coverage, but the specific naming of the parent test is
// irrelevant here since there's no notion of the catalog at all at this layer.
func TestManager_workflow_goodNotLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const TestOverrideCAChangeInitialDelay = 1 * time.Nanosecond

	m, signer := testManager(t, func(cfg *Config) {
		cfg.TestOverrideCAChangeInitialDelay = TestOverrideCAChangeInitialDelay
	})

	ca1 := signer.UpdateCA(t, nil)

	req := &ConnectCALeafRequest{
		Datacenter:     "dc1",
		Service:        "test",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}

	// List
	issued, meta, err := m.Get(ctx, req)
	require.NoError(t, err)
	require.False(t, meta.Hit)
	require.NotNil(t, issued)

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	require.True(t, issued.ModifyIndex > 0)
	require.Equal(t, issued.ModifyIndex, meta.Index)

	// Fetch it again
	testutil.RunStep(t, "test you get a cache hit on another read", func(t *testing.T) {
		req := &ConnectCALeafRequest{
			Datacenter:     "dc1",
			Service:        "test",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		}
		issued2, _, err := m.Get(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, issued2)
		require.Equal(t, issued, issued2)
	})

	// Test Blocking - see https://github.com/hashicorp/consul/issues/4462
	testutil.RunStep(t, "test blocking issue 4462", func(t *testing.T) {
		// Fetch it again
		req := &ConnectCALeafRequest{
			Datacenter:     "dc1",
			Service:        "test",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
			MinQueryIndex:  issued.ModifyIndex,
			MaxQueryTime:   125 * time.Millisecond,
		}
		var (
			respCh = make(chan *structs.IssuedCert)
			errCh  = make(chan error, 1)
		)
		go func() {
			issued2, _, err := m.Get(ctx, req)
			if err != nil {
				errCh <- err
			} else {
				respCh <- issued2
			}
		}()

		select {
		case <-time.After(500 * time.Millisecond):
			require.FailNow(t, "Shouldn't block for this long - not respecting wait parameter in the query")

		case err := <-errCh:
			require.NoError(t, err)
		case <-respCh:
		}
	})

	testutil.RunStep(t, "test that caching is updated in the background", func(t *testing.T) {
		// Set a new CA
		ca := signer.UpdateCA(t, nil)

		retry.Run(t, func(r *retry.R) {
			// Try and sign again (note no index/wait arg since cache should update in
			// background even if we aren't actively blocking)
			req := &ConnectCALeafRequest{
				Datacenter:     "dc1",
				Service:        "test",
				EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
			}

			issued2, _, err := m.Get(ctx, req)
			require.NoError(r, err)

			if issued.CertPEM == issued2.CertPEM {
				r.Fatalf("leaf has not updated")
			}

			// Got a new leaf. Sanity check it's a whole new key as well as different
			// cert.
			if issued.PrivateKeyPEM == issued2.PrivateKeyPEM {
				r.Fatalf("new leaf has same private key as before")
			}

			// Verify that the cert is signed by the new CA
			requireLeafValidUnderCA(r, issued2, ca)

			require.NotEqual(r, issued, issued2)
		})
	})
}

func TestManager_workflow_nonBlockingQuery_after_blockingQuery_shouldNotBlock(t *testing.T) {
	// see: https://github.com/hashicorp/consul/issues/12048

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	m, signer := testManager(t, nil)

	_ = signer.UpdateCA(t, nil)

	var (
		serialNumber string
		index        uint64
		issued       *structs.IssuedCert
	)
	testutil.RunStep(t, "do initial non-blocking query", func(t *testing.T) {
		req := &ConnectCALeafRequest{
			Datacenter:     "dc1",
			Service:        "test",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		}
		issued1, meta, err := m.Get(ctx, req)
		require.NoError(t, err)

		serialNumber = issued1.SerialNumber

		require.False(t, meta.Hit, "for the leaf cert cache type these are always MISS")
		index = meta.Index
		issued = issued1
	})

	go func() {
		// launch goroutine for blocking query
		req := &ConnectCALeafRequest{
			Datacenter:     "dc1",
			Service:        "test",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
			MinQueryIndex:  index,
		}
		_, _, _ = m.Get(ctx, req)
	}()

	// We just need to ensure that the above blocking query is in-flight before
	// the next step, so do a little sleep.
	time.Sleep(50 * time.Millisecond)

	// The initial non-blocking query populated the leaf cert cache entry
	// implicitly. The agent cache doesn't prune entries very often at all, so
	// in between both of these steps the data should still be there, causing
	// this to be a HIT that completes in less than 10m (the default inner leaf
	// cert blocking query timeout).
	testutil.RunStep(t, "do a non-blocking query that should not block", func(t *testing.T) {
		req := &ConnectCALeafRequest{
			Datacenter:     "dc1",
			Service:        "test",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		}
		issued2, meta2, err := m.Get(ctx, req)
		require.NoError(t, err)

		require.True(t, meta2.Hit)

		// If this is actually returning a cached result, the serial number
		// should be unchanged.
		require.Equal(t, serialNumber, issued2.SerialNumber)

		require.Equal(t, issued, issued2)
	})
}

func requireLeafValidUnderCA(t require.TestingT, issued *structs.IssuedCert, ca *structs.CARoot) {
	require.NotNil(t, issued)
	require.NotNil(t, ca)

	leaf, intermediates, err := connect.ParseLeafCerts(issued.CertPEM)
	require.NoError(t, err)

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(ca.RootCert)))

	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
	})
	require.NoError(t, err)

	// Verify the private key matches. tls.LoadX509Keypair does this for us!
	_, err = tls.X509KeyPair([]byte(issued.CertPEM), []byte(issued.PrivateKeyPEM))
	require.NoError(t, err)
}

// testManager returns a *Manager that is pre-configured to use a mock RPC
// implementation that can sign certs, and an in-memory CA roots reader that
// interacts well with it.
func testManager(t *testing.T, mut func(*Config)) (*Manager, *testSigner) {
	signer := newTestSigner(t, nil, nil)

	deps := Deps{
		Logger:      testutil.Logger(t),
		RootsReader: signer.RootsReader,
		CertSigner:  signer,
		Config: Config{
			// Override the root-change spread so we don't have to wait up to 20 seconds
			// to see root changes work. Can be changed back for specific tests that
			// need to test this, Note it's not 0 since that used default but is
			// effectively the same.
			TestOverrideCAChangeInitialDelay: 1 * time.Microsecond,
		},
	}
	if mut != nil {
		mut(&deps.Config)
	}

	m := NewManager(deps)
	t.Cleanup(m.Stop)

	return m, signer
}

type testRootsReader struct {
	mu      sync.Mutex
	index   uint64
	roots   *structs.IndexedCARoots
	watcher chan struct{}
}

func newTestRootsReader(t *testing.T) *testRootsReader {
	r := &testRootsReader{
		watcher: make(chan struct{}),
	}
	t.Cleanup(func() {
		r.mu.Lock()
		watcher := r.watcher
		r.mu.Unlock()
		close(watcher)
	})
	return r
}

var _ RootsReader = (*testRootsReader)(nil)

func (r *testRootsReader) Set(roots *structs.IndexedCARoots) {
	r.mu.Lock()
	oldWatcher := r.watcher
	r.watcher = make(chan struct{})
	r.roots = roots
	if roots == nil {
		r.index = 1
	} else {
		r.index = roots.Index
	}
	r.mu.Unlock()

	close(oldWatcher)
}

func (r *testRootsReader) Get() (*structs.IndexedCARoots, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.roots, nil
}

func (r *testRootsReader) Notify(ctx context.Context, correlationID string, ch chan<- cache.UpdateEvent) error {
	r.mu.Lock()
	watcher := r.watcher
	r.mu.Unlock()

	go func() {
		<-watcher

		r.mu.Lock()
		defer r.mu.Unlock()

		ch <- cache.UpdateEvent{
			CorrelationID: correlationID,
			Result:        r.roots,
			Meta:          cache.ResultMeta{Index: r.index},
			Err:           nil,
		}
	}()
	return nil
}

type testGetResult struct {
	Index uint64
	Value *structs.IssuedCert
	Err   error
}

// testAsyncGet returns a channel that returns the result of the testGet call.
//
// This is useful for testing timing and concurrency with testGet calls.
func testAsyncGet(t *testing.T, m *Manager, req *ConnectCALeafRequest) <-chan testGetResult {
	ch := make(chan testGetResult)
	go func() {
		index, cert, err := m.testGet(req)
		if err != nil {
			ch <- testGetResult{Err: err}
			return
		}

		ch <- testGetResult{Index: index, Value: cert}
	}()
	return ch
}
