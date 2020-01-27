package cache

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test a basic Get with no indexes (and therefore no blocking queries).
func TestCacheGet_noIndex(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	typ.Static(FetchResult{Value: 42}, nil).Times(1)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Get, should not fetch since we already have a satisfying value
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.True(meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test a basic Get with no index and a failed fetch.
func TestCacheGet_initError(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	fetcherr := fmt.Errorf("error")
	typ.Static(FetchResult{}, fetcherr).Times(2)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get("t", req)
	require.Error(err)
	require.Nil(result)
	require.False(meta.Hit)

	// Get, should fetch again since our last fetch was an error
	result, meta, err = c.Get("t", req)
	require.Error(err)
	require.Nil(result)
	require.False(meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test a cached error is replaced by a successful result. See
// https://github.com/hashicorp/consul/issues/4480
func TestCacheGet_cachedErrorsDontStick(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	fetcherr := fmt.Errorf("initial error")
	// First fetch errors, subsequent fetches are successful and then block
	typ.Static(FetchResult{}, fetcherr).Times(1)
	typ.Static(FetchResult{Value: 42, Index: 123}, nil).Times(1)
	// We trigger this to return same value to simulate a timeout.
	triggerCh := make(chan time.Time)
	typ.Static(FetchResult{Value: 42, Index: 123}, nil).WaitUntil(triggerCh)

	// Get, should fetch and get error
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get("t", req)
	require.Error(err)
	require.Nil(result)
	require.False(meta.Hit)

	// Get, should fetch again since our last fetch was an error, but get success
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Now get should block until timeout and then get the same response NOT the
	// cached error.
	getCh1 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{
		Key:      "hello",
		MinIndex: 123,
		// We _don't_ set a timeout here since that doesn't trigger the bug - the
		// bug occurs when the Fetch call times out and returns the same value when
		// an error is set. If it returns a new value the blocking loop works too.
	}))
	time.AfterFunc(50*time.Millisecond, func() {
		// "Timeout" the Fetch after a short time.
		close(triggerCh)
	})
	select {
	case result := <-getCh1:
		t.Fatalf("result or error returned before an update happened. "+
			"If this is nil look above for the error log: %v", result)
	case <-time.After(100 * time.Millisecond):
		// It _should_ keep blocking for a new value here
	}

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify the calls.
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test a Get with a request that returns a blank cache key. This should
// force a backend request and skip the cache entirely.
func TestCacheGet_blankCacheKey(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	typ.Static(FetchResult{Value: 42}, nil).Times(2)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: ""})
	result, meta, err := c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Get, should not fetch since we already have a satisfying value
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test that Get blocks on the initial value
func TestCacheGet_blockingInitSameKey(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	triggerCh := make(chan time.Time)
	typ.Static(FetchResult{Value: 42}, nil).WaitUntil(triggerCh).Times(1)

	// Perform multiple gets
	getCh1 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	getCh2 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))

	// They should block
	select {
	case <-getCh1:
		t.Fatal("should block (ch1)")
	case <-getCh2:
		t.Fatal("should block (ch2)")
	case <-time.After(50 * time.Millisecond):
	}

	// Trigger it
	close(triggerCh)

	// Should return
	TestCacheGetChResult(t, getCh1, 42)
	TestCacheGetChResult(t, getCh2, 42)
}

// Test that Get with different cache keys both block on initial value
// but that the fetches were both properly called.
func TestCacheGet_blockingInitDiffKeys(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Keep track of the keys
	var keysLock sync.Mutex
	var keys []string

	// Configure the type
	triggerCh := make(chan time.Time)
	typ.Static(FetchResult{Value: 42}, nil).
		WaitUntil(triggerCh).
		Times(2).
		Run(func(args mock.Arguments) {
			keysLock.Lock()
			defer keysLock.Unlock()
			keys = append(keys, args.Get(1).(Request).CacheInfo().Key)
		})

	// Perform multiple gets
	getCh1 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	getCh2 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "goodbye"}))

	// They should block
	select {
	case <-getCh1:
		t.Fatal("should block (ch1)")
	case <-getCh2:
		t.Fatal("should block (ch2)")
	case <-time.After(50 * time.Millisecond):
	}

	// Trigger it
	close(triggerCh)

	// Should return both!
	TestCacheGetChResult(t, getCh1, 42)
	TestCacheGetChResult(t, getCh2, 42)

	// Verify proper keys
	sort.Strings(keys)
	require.Equal([]string{"goodbye", "hello"}, keys)
}

// Test a get with an index set will wait until an index that is higher
// is set in the cache.
func TestCacheGet_blockingIndex(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	triggerCh := make(chan time.Time)
	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).Once()
	typ.Static(FetchResult{Value: 42, Index: 6}, nil).WaitUntil(triggerCh)

	// Fetch should block
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{
		Key: "hello", MinIndex: 5}))

	// Should block
	select {
	case <-resultCh:
		t.Fatal("should block")
	case <-time.After(50 * time.Millisecond):
	}

	// Wait a bit
	close(triggerCh)

	// Should return
	TestCacheGetChResult(t, resultCh, 42)
}

// Test a get with an index set will timeout if the fetch doesn't return
// anything.
func TestCacheGet_blockingIndexTimeout(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	triggerCh := make(chan time.Time)
	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).Once()
	typ.Static(FetchResult{Value: 42, Index: 6}, nil).WaitUntil(triggerCh)

	// Fetch should block
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{
		Key: "hello", MinIndex: 5, Timeout: 200 * time.Millisecond}))

	// Should block
	select {
	case <-resultCh:
		t.Fatal("should block")
	case <-time.After(50 * time.Millisecond):
	}

	// Should return after more of the timeout
	select {
	case result := <-resultCh:
		require.Equal(t, 12, result)
	case <-time.After(300 * time.Millisecond):
		t.Fatal("should've returned")
	}
}

// Test a get with an index set with requests returning an error
// will return that error.
func TestCacheGet_blockingIndexError(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	var retries uint32
	fetchErr := fmt.Errorf("test fetch error")
	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ.Static(FetchResult{Value: nil, Index: 5}, fetchErr).Run(func(args mock.Arguments) {
		atomic.AddUint32(&retries, 1)
	})

	// First good fetch to populate catch
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 1)

	// Fetch should not block and should return error
	resultCh = TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{
		Key: "hello", MinIndex: 7, Timeout: 1 * time.Minute}))
	TestCacheGetChResult(t, resultCh, nil)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Check the number
	actual := atomic.LoadUint32(&retries)
	require.True(t, actual < 10, fmt.Sprintf("actual: %d", actual))
}

// Test that if a Type returns an empty value on Fetch that the previous
// value is preserved.
func TestCacheGet_emptyFetchResult(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	stateCh := make(chan int, 1)

	// Configure the type
	typ.Static(FetchResult{Value: 42, State: 31, Index: 1}, nil).Times(1)
	// Return different State, it should NOT be ignored
	typ.Static(FetchResult{Value: nil, State: 32}, nil).Run(func(args mock.Arguments) {
		// We should get back the original state
		opts := args.Get(0).(FetchOptions)
		require.NotNil(opts.LastResult)
		stateCh <- opts.LastResult.State.(int)
	})

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Get, should not fetch since we already have a satisfying value
	req = TestRequest(t, RequestInfo{
		Key: "hello", MinIndex: 1, Timeout: 100 * time.Millisecond})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// State delivered to second call should be the result from first call.
	select {
	case state := <-stateCh:
		require.Equal(31, state)
	case <-time.After(20 * time.Millisecond):
		t.Fatal("timed out")
	}

	// Next request should get the SECOND returned state even though the fetch
	// returns nil and so the previous result is used.
	req = TestRequest(t, RequestInfo{
		Key: "hello", MinIndex: 1, Timeout: 100 * time.Millisecond})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)
	select {
	case state := <-stateCh:
		require.Equal(32, state)
	case <-time.After(20 * time.Millisecond):
		t.Fatal("timed out")
	}

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test that a type registered with a periodic refresh will perform
// that refresh after the timer is up.
func TestCacheGet_periodicRefresh(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh:        true,
		RefreshTimer:   100 * time.Millisecond,
		RefreshTimeout: 5 * time.Minute,
	})

	// This is a bit weird, but we do this to ensure that the final
	// call to the Fetch (if it happens, depends on timing) just blocks.
	triggerCh := make(chan time.Time)
	defer close(triggerCh)

	// Configure the type
	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).Once()
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).WaitUntil(triggerCh)

	// Fetch should block
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 1)

	// Fetch again almost immediately should return old result
	time.Sleep(5 * time.Millisecond)
	resultCh = TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 1)

	// Wait for the timer
	time.Sleep(200 * time.Millisecond)
	resultCh = TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 12)
}

// Test that a type registered with a periodic refresh will perform
// that refresh after the timer is up.
func TestCacheGet_periodicRefreshMultiple(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0 * time.Millisecond,
		RefreshTimeout: 5 * time.Minute,
	})

	// This is a bit weird, but we do this to ensure that the final
	// call to the Fetch (if it happens, depends on timing) just blocks.
	trigger := make([]chan time.Time, 3)
	for i := range trigger {
		trigger[i] = make(chan time.Time)
	}

	// Configure the type
	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).Once().WaitUntil(trigger[0])
	typ.Static(FetchResult{Value: 24, Index: 6}, nil).Once().WaitUntil(trigger[1])
	typ.Static(FetchResult{Value: 42, Index: 7}, nil).WaitUntil(trigger[2])

	// Fetch should block
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 1)

	// Fetch again almost immediately should return old result
	time.Sleep(5 * time.Millisecond)
	resultCh = TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 1)

	// Trigger the next, sleep a bit, and verify we get the next result
	close(trigger[0])
	time.Sleep(100 * time.Millisecond)
	resultCh = TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 12)

	// Trigger the next, sleep a bit, and verify we get the next result
	close(trigger[1])
	time.Sleep(100 * time.Millisecond)
	resultCh = TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 24)
}

// Test that a refresh performs a backoff.
func TestCacheGet_periodicRefreshErrorBackoff(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 5 * time.Minute,
	})

	// Configure the type
	var retries uint32
	fetchErr := fmt.Errorf("test fetch error")
	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ.Static(FetchResult{Value: nil, Index: 5}, fetchErr).Run(func(args mock.Arguments) {
		atomic.AddUint32(&retries, 1)
	})

	// Fetch
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 1)

	// Sleep a bit. The refresh will quietly fail in the background. What we
	// want to verify is that it doesn't retry too much. "Too much" is hard
	// to measure since its CPU dependent if this test is failing. But due
	// to the short sleep below, we can calculate about what we'd expect if
	// backoff IS working.
	time.Sleep(500 * time.Millisecond)

	// Fetch should work, we should get a 1 still. Errors are ignored.
	resultCh = TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 1)

	// Check the number
	actual := atomic.LoadUint32(&retries)
	require.True(t, actual < 10, fmt.Sprintf("actual: %d", actual))
}

// Test that a badly behaved RPC that returns 0 index will perform a backoff.
func TestCacheGet_periodicRefreshBadRPCZeroIndexErrorBackoff(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 5 * time.Minute,
	})

	// Configure the type
	var retries uint32
	typ.Static(FetchResult{Value: 0, Index: 0}, nil).Run(func(args mock.Arguments) {
		atomic.AddUint32(&retries, 1)
	})

	// Fetch
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 0)

	// Sleep a bit. The refresh will quietly fail in the background. What we
	// want to verify is that it doesn't retry too much. "Too much" is hard
	// to measure since its CPU dependent if this test is failing. But due
	// to the short sleep below, we can calculate about what we'd expect if
	// backoff IS working.
	time.Sleep(500 * time.Millisecond)

	// Fetch should work, we should get a 0 still. Errors are ignored.
	resultCh = TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 0)

	// Check the number
	actual := atomic.LoadUint32(&retries)
	require.True(t, actual < 10, fmt.Sprintf("%d retries, should be < 10", actual))
}

// Test that fetching with no index makes an initial request with no index, but
// then ensures all background refreshes have > 0. This ensures we don't end up
// with any index 0 loops from background refreshed while also returning
// immediately on the initial request if there is no data written to that table
// yet.
func TestCacheGet_noIndexSetsOne(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 5 * time.Minute,
	})

	// Simulate "well behaved" RPC with no data yet but returning 1
	{
		first := int32(1)

		typ.Static(FetchResult{Value: 0, Index: 1}, nil).Run(func(args mock.Arguments) {
			opts := args.Get(0).(FetchOptions)
			isFirst := atomic.SwapInt32(&first, 0)
			if isFirst == 1 {
				assert.Equal(t, uint64(0), opts.MinIndex)
			} else {
				assert.True(t, opts.MinIndex > 0, "minIndex > 0")
			}
		})

		// Fetch
		resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
		TestCacheGetChResult(t, resultCh, 0)

		// Sleep a bit so background refresh happens
		time.Sleep(100 * time.Millisecond)
	}

	// Same for "badly behaved" RPC that returns 0 index and no data
	{
		first := int32(1)

		typ.Static(FetchResult{Value: 0, Index: 0}, nil).Run(func(args mock.Arguments) {
			opts := args.Get(0).(FetchOptions)
			isFirst := atomic.SwapInt32(&first, 0)
			if isFirst == 1 {
				assert.Equal(t, uint64(0), opts.MinIndex)
			} else {
				assert.True(t, opts.MinIndex > 0, "minIndex > 0")
			}
		})

		// Fetch
		resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
		TestCacheGetChResult(t, resultCh, 0)

		// Sleep a bit so background refresh happens
		time.Sleep(100 * time.Millisecond)
	}
}

// Test that the backend fetch sets the proper timeout.
func TestCacheGet_fetchTimeout(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)

	// Register the type with a timeout
	timeout := 10 * time.Minute
	c.RegisterType("t", typ, &RegisterOptions{
		RefreshTimeout: timeout,
	})

	// Configure the type
	var actual time.Duration
	typ.Static(FetchResult{Value: 42}, nil).Times(1).Run(func(args mock.Arguments) {
		opts := args.Get(0).(FetchOptions)
		actual = opts.Timeout
	})

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Test the timeout
	require.Equal(timeout, actual)
}

// Test that entries expire
func TestCacheGet_expire(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)

	// Register the type with a timeout
	c.RegisterType("t", typ, &RegisterOptions{
		LastGetTTL: 400 * time.Millisecond,
	})

	// Configure the type
	typ.Static(FetchResult{Value: 42}, nil).Times(2)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Wait for a non-trivial amount of time to sanity check the age increases at
	// least this amount. Note that this is not a fudge for some timing-dependent
	// background work it's just ensuring a non-trivial time elapses between the
	// request above and below serilaly in this thread so short time is OK.
	time.Sleep(5 * time.Millisecond)

	// Get, should not fetch, verified via the mock assertions above
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.True(meta.Hit)
	require.True(meta.Age > 5*time.Millisecond)

	// Sleep for the expiry
	time.Sleep(500 * time.Millisecond)

	// Get, should fetch
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test that entries reset their TTL on Get
func TestCacheGet_expireResetGet(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)

	// Register the type with a timeout
	c.RegisterType("t", typ, &RegisterOptions{
		LastGetTTL: 150 * time.Millisecond,
	})

	// Configure the type
	typ.Static(FetchResult{Value: 42}, nil).Times(2)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Fetch multiple times, where the total time is well beyond
	// the TTL. We should not trigger any fetches during this time.
	for i := 0; i < 5; i++ {
		// Sleep a bit
		time.Sleep(50 * time.Millisecond)

		// Get, should not fetch
		req = TestRequest(t, RequestInfo{Key: "hello"})
		result, meta, err = c.Get("t", req)
		require.NoError(err)
		require.Equal(42, result)
		require.True(meta.Hit)
	}

	time.Sleep(200 * time.Millisecond)

	// Get, should fetch
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test a Get with a request that returns the same cache key across
// two different "types" returns two separate results.
func TestCacheGet_duplicateKeyDifferentType(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	typ2 := TestType(t)
	defer typ2.AssertExpectations(t)

	c := TestCache(t)
	c.RegisterType("t", typ, nil)
	c.RegisterType("t2", typ2, nil)

	// Configure the types
	typ.Static(FetchResult{Value: 100}, nil)
	typ2.Static(FetchResult{Value: 200}, nil)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "foo"})
	result, meta, err := c.Get("t", req)
	require.NoError(err)
	require.Equal(100, result)
	require.False(meta.Hit)

	// Get from t2 with same key, should fetch
	req = TestRequest(t, RequestInfo{Key: "foo"})
	result, meta, err = c.Get("t2", req)
	require.NoError(err)
	require.Equal(200, result)
	require.False(meta.Hit)

	// Get from t again with same key, should cache
	req = TestRequest(t, RequestInfo{Key: "foo"})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(100, result)
	require.True(meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
	typ2.AssertExpectations(t)
}

// Test that Get partitions the caches based on DC so two equivalent requests
// to different datacenters are automatically cached even if their keys are
// the same.
func TestCacheGet_partitionDC(t *testing.T) {
	t.Parallel()

	c := TestCache(t)
	c.RegisterType("t", &testPartitionType{}, nil)

	// Perform multiple gets
	getCh1 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{
		Datacenter: "dc1", Key: "hello"}))
	getCh2 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{
		Datacenter: "dc9", Key: "hello"}))

	// Should return both!
	TestCacheGetChResult(t, getCh1, "dc1")
	TestCacheGetChResult(t, getCh2, "dc9")
}

// Test that Get partitions the caches based on token so two equivalent requests
// with different ACL tokens do not return the same result.
func TestCacheGet_partitionToken(t *testing.T) {
	t.Parallel()

	c := TestCache(t)
	c.RegisterType("t", &testPartitionType{}, nil)

	// Perform multiple gets
	getCh1 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{
		Token: "", Key: "hello"}))
	getCh2 := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{
		Token: "foo", Key: "hello"}))

	// Should return both!
	TestCacheGetChResult(t, getCh1, "")
	TestCacheGetChResult(t, getCh2, "foo")
}

// testPartitionType implements Type for testing that simply returns a value
// comprised of the request DC and ACL token, used for testing cache
// partitioning.
type testPartitionType struct{}

func (t *testPartitionType) Fetch(opts FetchOptions, r Request) (FetchResult, error) {
	info := r.CacheInfo()
	return FetchResult{
		Value: fmt.Sprintf("%s%s", info.Datacenter, info.Token),
	}, nil
}

func (t *testPartitionType) SupportsBlocking() bool {
	return true
}

// Test that background refreshing reports correct Age in failure and happy
// states.
func TestCacheGet_refreshAge(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0,
		RefreshTimeout: 5 * time.Minute,
	})

	// Configure the type
	var index, shouldFail uint64

	typ.On("Fetch", mock.Anything, mock.Anything).
		Return(func(o FetchOptions, r Request) FetchResult {
			idx := atomic.LoadUint64(&index)
			if atomic.LoadUint64(&shouldFail) == 1 {
				t.Logf("Failing Fetch at index %d", idx)
				return FetchResult{Value: nil, Index: idx}
			}
			if o.MinIndex == idx {
				t.Logf("Sleeping Fetch at index %d", idx)
				// Simulate waiting for a new value
				time.Sleep(5 * time.Millisecond)
			}
			t.Logf("Returning Fetch at index %d", idx)
			return FetchResult{Value: int(idx * 2), Index: idx}
		}, func(o FetchOptions, r Request) error {
			if atomic.LoadUint64(&shouldFail) == 1 {
				return errors.New("test error")
			}
			return nil
		})

	// Set initial index/value
	atomic.StoreUint64(&index, 4)

	// Fetch
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 8)

	{
		// Wait a few milliseconds after initial fetch to check age is not reporting
		// actual age.
		time.Sleep(2 * time.Millisecond)

		// Fetch again, non-blocking
		result, meta, err := c.Get("t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(err)
		require.Equal(8, result)
		require.True(meta.Hit)
		// Age should be zero since background refresh was "active"
		require.Equal(time.Duration(0), meta.Age)
	}

	// Now fail the next background sync
	atomic.StoreUint64(&shouldFail, 1)

	// Wait until the current request times out and starts failing. The request
	// should take a maximum of 5ms to return but give it some headroom to allow
	// it to finish 5ms sleep, unblock and next background request to be attemoted
	// and fail and state updated in noisy CI... We might want to retry if this is
	// still flaky but see if a longer wait is sufficient for now.
	time.Sleep(50 * time.Millisecond)

	var lastAge time.Duration
	{
		result, meta, err := c.Get("t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(err)
		require.Equal(8, result)
		require.True(meta.Hit)
		// Age should be non-zero since background refresh was "active"
		require.True(meta.Age > 0)
		lastAge = meta.Age
	}
	// Wait a bit longer - age should increase by at least this much
	time.Sleep(5 * time.Millisecond)
	{
		result, meta, err := c.Get("t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(err)
		require.Equal(8, result)
		require.True(meta.Hit)
		require.True(meta.Age > (lastAge + (1 * time.Millisecond)))
	}

	// Now unfail the background refresh
	atomic.StoreUint64(&shouldFail, 0)

	// And update the data so we can see when the background task is working again
	// (won't be immediate due to backoff on the errors).
	atomic.AddUint64(&index, 1)

	t0 := time.Now()

	timeout := true
	// Allow up to 5 seconds since the error backoff is likely to have kicked in
	// and causes this to take different amounts of time depending on how quickly
	// the test thread got down here relative to the failures.
	for attempts := 0; attempts < 50; attempts++ {
		time.Sleep(100 * time.Millisecond)
		result, meta, err := c.Get("t", TestRequest(t, RequestInfo{Key: "hello"}))
		// Should never error even if background is failing as we have cached value
		require.NoError(err)
		require.True(meta.Hit)
		// Got the new value!
		if result == 10 {
			// Age should be zero since background refresh is "active" again
			t.Logf("Succeeded after %d attempts", attempts)
			require.Equal(time.Duration(0), meta.Age)
			timeout = false
			break
		}
	}
	require.False(timeout, "failed to observe update after %s", time.Since(t0))
}

func TestCacheGet_nonRefreshAge(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh:    false,
		LastGetTTL: 100 * time.Millisecond,
	})

	// Configure the type
	var index uint64

	typ.On("Fetch", mock.Anything, mock.Anything).
		Return(func(o FetchOptions, r Request) FetchResult {
			idx := atomic.LoadUint64(&index)
			return FetchResult{Value: int(idx * 2), Index: idx}
		}, nil)

	// Set initial index/value
	atomic.StoreUint64(&index, 4)

	// Fetch
	resultCh := TestCacheGetCh(t, c, "t", TestRequest(t, RequestInfo{Key: "hello"}))
	TestCacheGetChResult(t, resultCh, 8)

	var lastAge time.Duration
	{
		// Wait a few milliseconds after initial fetch to check age IS reporting
		// actual age.
		time.Sleep(5 * time.Millisecond)

		// Fetch again, non-blocking
		result, meta, err := c.Get("t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(err)
		require.Equal(8, result)
		require.True(meta.Hit)
		require.True(meta.Age > (5 * time.Millisecond))
		lastAge = meta.Age
	}

	// Wait for expiry
	time.Sleep(200 * time.Millisecond)

	{
		result, meta, err := c.Get("t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(err)
		require.Equal(8, result)
		require.False(meta.Hit)
		// Age should smaller again
		require.True(meta.Age < lastAge)
	}

	{
		// Wait for a non-trivial amount of time to sanity check the age increases at
		// least this amount. Note that this is not a fudge for some timing-dependent
		// background work it's just ensuring a non-trivial time elapses between the
		// request above and below serilaly in this thread so short time is OK.
		time.Sleep(5 * time.Millisecond)

		// Fetch again, non-blocking
		result, meta, err := c.Get("t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(err)
		require.Equal(8, result)
		require.True(meta.Hit)
		require.True(meta.Age > (5 * time.Millisecond))
		lastAge = meta.Age
	}

	// Now verify that setting MaxAge results in cache invalidation
	{
		result, meta, err := c.Get("t", TestRequest(t, RequestInfo{
			Key:    "hello",
			MaxAge: 1 * time.Millisecond,
		}))
		require.NoError(err)
		require.Equal(8, result)
		require.False(meta.Hit)
		// Age should smaller again
		require.True(meta.Age < lastAge)
	}
}

func TestCacheGet_nonBlockingType(t *testing.T) {
	t.Parallel()

	typ := TestTypeNonBlocking(t)
	c := TestCache(t)
	c.RegisterType("t", typ, nil)

	// Configure the type
	typ.Static(FetchResult{Value: 42, Index: 1}, nil).Once()
	typ.Static(FetchResult{Value: 43, Index: 2}, nil).Twice().
		Run(func(args mock.Arguments) {
			opts := args.Get(0).(FetchOptions)
			// MinIndex should never be set for a non-blocking type.
			require.Equal(t, uint64(0), opts.MinIndex)
		})

	require := require.New(t)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.False(meta.Hit)

	// Get, should not fetch since we have a cached value
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.True(meta.Hit)

	// Get, should not attempt to fetch with blocking even if requested. The
	// assertions below about the value being the same combined with the fact the
	// mock will only return that value on first call suffice to show that
	// blocking request is not being attempted.
	req = TestRequest(t, RequestInfo{
		Key:      "hello",
		MinIndex: 1,
		Timeout:  10 * time.Minute,
	})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(42, result)
	require.True(meta.Hit)

	time.Sleep(10 * time.Millisecond)

	// Get with a max age should fetch again
	req = TestRequest(t, RequestInfo{Key: "hello", MaxAge: 5 * time.Millisecond})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(43, result)
	require.False(meta.Hit)

	// Get with a must revalidate should fetch again even without a delay.
	req = TestRequest(t, RequestInfo{Key: "hello", MustRevalidate: true})
	result, meta, err = c.Get("t", req)
	require.NoError(err)
	require.Equal(43, result)
	require.False(meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}
