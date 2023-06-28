// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cache

import (
	"context"
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
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/lib/ttlcache"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// Test a basic Get with no indexes (and therefore no blocking queries).
func TestCacheGet_noIndex(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	// Configure the type
	typ.Static(FetchResult{Value: 42}, nil).Times(1)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Get, should not fetch since we already have a satisfying value
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.True(t, meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test a basic Get with no index and a failed fetch.
func TestCacheGet_initError(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	// Configure the type
	fetcherr := fmt.Errorf("error")
	typ.Static(FetchResult{}, fetcherr).Times(2)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.Error(t, err)
	require.Nil(t, result)
	require.False(t, meta.Hit)

	// Get, should fetch again since our last fetch was an error
	result, meta, err = c.Get(context.Background(), "t", req)
	require.Error(t, err)
	require.Nil(t, result)
	require.False(t, meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test a cached error is replaced by a successful result. See
// https://github.com/hashicorp/consul/issues/4480
func TestCacheGet_cachedErrorsDontStick(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
	result, meta, err := c.Get(context.Background(), "t", req)
	require.Error(t, err)
	require.Nil(t, result)
	require.False(t, meta.Hit)

	// Get, should fetch again since our last fetch was an error, but get success
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

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

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	// Configure the type
	typ.Static(FetchResult{Value: 42}, nil).Times(2)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: ""})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Get, should not fetch since we already have a satisfying value
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

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
	c := New(Options{})
	c.RegisterType("t", typ)

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

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
	require.Equal(t, []string{"goodbye", "hello"}, keys)
}

// Test a get with an index set will wait until an index that is higher
// is set in the cache.
func TestCacheGet_blockingIndex(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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

func TestCacheGet_cancellation(t *testing.T) {
	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Times(0).WaitUntil(time.After(1 * time.Millisecond))

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(50*time.Millisecond))
	// this is just to keep the linter happy
	defer cancel()

	result, _, err := c.Get(ctx, "t", TestRequest(t, RequestInfo{
		Key: "hello", MinIndex: 5}))

	require.Nil(t, result)
	require.Error(t, err)
	testutil.RequireErrorContains(t, err, context.DeadlineExceeded.Error())
}

// Test a get with an index set will timeout if the fetch doesn't return
// anything.
func TestCacheGet_blockingIndexTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	stateCh := make(chan int, 1)

	// Configure the type
	typ.Static(FetchResult{Value: 42, State: 31, Index: 1}, nil).Times(1)
	// Return different State, it should NOT be ignored
	typ.Static(FetchResult{Value: nil, State: 32}, nil).Run(func(args mock.Arguments) {
		// We should get back the original state
		opts := args.Get(0).(FetchOptions)
		require.NotNil(t, opts.LastResult)
		stateCh <- opts.LastResult.State.(int)
	})

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Get, should not fetch since we already have a satisfying value
	req = TestRequest(t, RequestInfo{
		Key: "hello", MinIndex: 1, Timeout: 100 * time.Millisecond})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// State delivered to second call should be the result from first call.
	select {
	case state := <-stateCh:
		require.Equal(t, 31, state)
	case <-time.After(20 * time.Millisecond):
		t.Fatal("timed out")
	}

	// Next request should get the SECOND returned state even though the fetch
	// returns nil and so the previous result is used.
	req = TestRequest(t, RequestInfo{
		Key: "hello", MinIndex: 1, Timeout: 100 * time.Millisecond})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)
	select {
	case state := <-stateCh:
		require.Equal(t, 32, state)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		Refresh:      true,
		RefreshTimer: 100 * time.Millisecond,
		QueryTimeout: 5 * time.Minute,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		Refresh:      true,
		RefreshTimer: 0 * time.Millisecond,
		QueryTimeout: 5 * time.Minute,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		Refresh:      true,
		RefreshTimer: 0,
		QueryTimeout: 5 * time.Minute,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		Refresh:      true,
		RefreshTimer: 0,
		QueryTimeout: 5 * time.Minute,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		SupportsBlocking: true,
		Refresh:          true,
		RefreshTimer:     0,
		QueryTimeout:     5 * time.Minute,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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

	typ := &MockType{}
	timeout := 10 * time.Minute
	typ.On("RegisterOptions").Return(RegisterOptions{
		QueryTimeout:     timeout,
		SupportsBlocking: true,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})

	// Register the type with a timeout
	c.RegisterType("t", typ)

	// Configure the type
	var actual time.Duration
	typ.Static(FetchResult{Value: 42}, nil).Times(1).Run(func(args mock.Arguments) {
		opts := args.Get(0).(FetchOptions)
		actual = opts.Timeout
	})

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Test the timeout
	require.Equal(t, timeout, actual)
}

// Test that entries expire
func TestCacheGet_expire(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		LastGetTTL: 400 * time.Millisecond,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})

	// Register the type with a timeout
	c.RegisterType("t", typ)

	// Configure the type
	typ.Static(FetchResult{Value: 42}, nil).Times(2)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Wait for a non-trivial amount of time to sanity check the age increases at
	// least this amount. Note that this is not a fudge for some timing-dependent
	// background work it's just ensuring a non-trivial time elapses between the
	// request above and below serially in this thread so short time is OK.
	time.Sleep(5 * time.Millisecond)

	// Get, should not fetch, verified via the mock assertions above
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.True(t, meta.Hit)
	require.True(t, meta.Age > 5*time.Millisecond)

	// Sleep for the expiry
	time.Sleep(500 * time.Millisecond)

	// Get, should fetch
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen then verify
	// that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test that entries expire for background refresh types that cancel fetch on
// eviction. This is really a special case of the test below where the close
// behavior of the type forces the timing that causes the race but it's worth
// keeping explicitly anyway to make sure this behavior is supported and
// doesn't introduce any different races.
func TestCacheGet_expireBackgroudRefreshCancel(t *testing.T) {
	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		LastGetTTL:       400 * time.Millisecond,
		Refresh:          true,
		RefreshTimer:     0,
		SupportsBlocking: true,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})

	// Register the type with a timeout
	c.RegisterType("t", typ)

	// Create a cache state that is a closer that cancels the context on close
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closer := &testCloser{
		closeFn: func() {
			cancel()
		},
	}

	// Configure the type
	typ.On("Fetch", mock.Anything, mock.Anything).
		Return(func(o FetchOptions, r Request) FetchResult {
			return FetchResult{Value: 8, Index: 4, State: closer}
		}, func(o FetchOptions, r Request) error {
			if o.MinIndex == 4 {
				// Simulate waiting for a new value on second call until the cache type
				// is evicted
				<-ctx.Done()
				return ctx.Err()
			}
			return nil
		})

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 8, result)
	require.Equal(t, uint64(4), meta.Index)
	require.False(t, meta.Hit)

	// Get, should not fetch, verified via the mock assertions above
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 8, result)
	require.Equal(t, uint64(4), meta.Index)
	require.True(t, meta.Hit)

	// Sleep for the expiry
	time.Sleep(500 * time.Millisecond)

	// Get, should fetch
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 8, result)
	require.Equal(t, uint64(4), meta.Index)
	require.False(t, meta.Hit, "the fetch should not have re-populated the cache "+
		"entry after it expired so this get should be a miss")

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test that entries expire for background refresh types that return before any
// watcher re-fetches.
func TestCacheGet_expireBackgroudRefresh(t *testing.T) {
	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		LastGetTTL:       400 * time.Millisecond,
		Refresh:          true,
		RefreshTimer:     0,
		SupportsBlocking: true,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})

	// Register the type with a timeout
	c.RegisterType("t", typ)

	ctrlCh := make(chan struct{})

	// Configure the type
	typ.On("Fetch", mock.Anything, mock.Anything).
		Return(func(o FetchOptions, r Request) FetchResult {
			if o.MinIndex == 4 {
				// Simulate returning from fetch (after a timeout with no value change)
				// at a time controlled by the test to ensure we interleave requests.
				<-ctrlCh
			}
			return FetchResult{Value: 8, Index: 4}
		}, func(o FetchOptions, r Request) error {
			return nil
		})

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 8, result)
	require.Equal(t, uint64(4), meta.Index)
	require.False(t, meta.Hit)

	// Get, should not fetch, verified via the mock assertions above
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 8, result)
	require.Equal(t, uint64(4), meta.Index)
	require.True(t, meta.Hit)

	// Sleep for the expiry
	time.Sleep(500 * time.Millisecond)

	// Now (after expiry) let the fetch call return
	close(ctrlCh)

	// Get, should fetch (it didn't originally because the fetch return would
	// re-insert the value back into the cache and make it live forever).
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 8, result)
	require.Equal(t, uint64(4), meta.Index)
	require.False(t, meta.Hit, "the fetch should not have re-populated the cache "+
		"entry after it expired so this get should be a miss")

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test that entries reset their TTL on Get
func TestCacheGet_expireResetGet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		LastGetTTL: 150 * time.Millisecond,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})

	// Register the type with a timeout
	c.RegisterType("t", typ)

	// Configure the type
	typ.Static(FetchResult{Value: 42}, nil).Times(2)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Fetch multiple times, where the total time is well beyond
	// the TTL. We should not trigger any fetches during this time.
	for i := 0; i < 5; i++ {
		// Sleep a bit
		time.Sleep(50 * time.Millisecond)

		// Get, should not fetch
		req = TestRequest(t, RequestInfo{Key: "hello"})
		result, meta, err = c.Get(context.Background(), "t", req)
		require.NoError(t, err)
		require.Equal(t, 42, result)
		require.True(t, meta.Hit)
	}

	time.Sleep(200 * time.Millisecond)

	// Get, should fetch
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test that entries reset their TTL on Get even when the value isn't changing
func TestCacheGet_expireResetGetNoChange(t *testing.T) {
	t.Parallel()

	// Create a closer so we can tell if the entry gets evicted.
	closer := &testCloser{}

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		LastGetTTL:       150 * time.Millisecond,
		SupportsBlocking: true,
		Refresh:          true,
	})
	typ.On("Fetch", mock.Anything, mock.Anything).
		Return(func(o FetchOptions, r Request) FetchResult {
			if o.MinIndex == 10 {
				// Simulate a very fast timeout from the backend. This must be shorter
				// than the TTL above (as it would be in real life) so that fetch returns
				// a few times with the same value which _should_ cause the blocking watch
				// to go round the Get loop and so keep the cache entry from being
				// evicted.
				time.Sleep(10 * time.Millisecond)
			}
			return FetchResult{Value: 42, Index: 10, State: closer}
		}, func(o FetchOptions, r Request) error {
			return nil
		})
	defer typ.AssertExpectations(t)
	c := New(Options{})

	// Register the type with a timeout
	c.RegisterType("t", typ)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.Equal(t, uint64(10), meta.Index)
	require.False(t, meta.Hit)

	// Do a blocking watch of the value that won't time out until after the TTL.
	start := time.Now()
	req = TestRequest(t, RequestInfo{Key: "hello", MinIndex: 10, Timeout: 300 * time.Millisecond})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.Equal(t, uint64(10), meta.Index)
	require.GreaterOrEqual(t, time.Since(start).Milliseconds(), int64(300))

	// This is the point of this test! Even though we waited for a change for
	// longer than the TTL, we should have been updating the TTL so that the cache
	// entry should not have been evicted. We can't verify that with meta.Hit
	// since that is not set for blocking Get calls but we can assert that the
	// entry was never closed (which assuming the test for eviction closing is
	// also passing is a reliable signal).
	require.False(t, closer.isClosed(), "cache entry should not have been evicted")

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test that entries with state that satisfies io.Closer get cleaned up
func TestCacheGet_expireClose(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	defer typ.AssertExpectations(t)
	c := New(Options{})
	defer c.Close()
	typ.On("RegisterOptions").Return(RegisterOptions{
		SupportsBlocking: true,
		LastGetTTL:       100 * time.Millisecond,
	})

	// Register the type with a timeout
	c.RegisterType("t", typ)

	// Configure the type
	state := &testCloser{}
	typ.Static(FetchResult{Value: 42, State: state}, nil).Times(1)

	ctx := context.Background()
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(ctx, "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)
	require.False(t, state.isClosed())

	// Sleep for the expiry
	time.Sleep(200 * time.Millisecond)

	// state.Close() should have been called
	require.True(t, state.isClosed())
}

type testCloser struct {
	closed  uint32
	closeFn func()
}

func (t *testCloser) Close() error {
	atomic.SwapUint32(&t.closed, 1)
	if t.closeFn != nil {
		t.closeFn()
	}
	return nil
}

func (t *testCloser) isClosed() bool {
	return atomic.LoadUint32(&t.closed) == 1
}

// Test a Get with a request that returns the same cache key across
// two different "types" returns two separate results.
func TestCacheGet_duplicateKeyDifferentType(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	typ2 := TestType(t)
	defer typ2.AssertExpectations(t)

	c := New(Options{})
	c.RegisterType("t", typ)
	c.RegisterType("t2", typ2)

	// Configure the types
	typ.Static(FetchResult{Value: 100}, nil)
	typ2.Static(FetchResult{Value: 200}, nil)

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "foo"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 100, result)
	require.False(t, meta.Hit)

	// Get from t2 with same key, should fetch
	req = TestRequest(t, RequestInfo{Key: "foo"})
	result, meta, err = c.Get(context.Background(), "t2", req)
	require.NoError(t, err)
	require.Equal(t, 200, result)
	require.False(t, meta.Hit)

	// Get from t again with same key, should cache
	req = TestRequest(t, RequestInfo{Key: "foo"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 100, result)
	require.True(t, meta.Hit)

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

	c := New(Options{})
	c.RegisterType("t", &testPartitionType{})

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

	c := New(Options{})
	c.RegisterType("t", &testPartitionType{})

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

func (t *testPartitionType) RegisterOptions() RegisterOptions {
	return RegisterOptions{
		SupportsBlocking: true,
	}
}

// Test that background refreshing reports correct Age in failure and happy
// states.
func TestCacheGet_refreshAge(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	if testing.Short() {
		t.Skip("too slow for -short run")
	}
	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		Refresh:      true,
		RefreshTimer: 0,
		QueryTimeout: 5 * time.Minute,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	// Configure the type
	var index, shouldFail uint64

	typ.On("Fetch", mock.Anything, mock.Anything).
		Return(func(o FetchOptions, r Request) FetchResult {
			idx := atomic.LoadUint64(&index)
			if atomic.LoadUint64(&shouldFail) == 1 {
				return FetchResult{Value: nil, Index: idx}
			}
			if o.MinIndex == idx {
				// Simulate waiting for a new value
				time.Sleep(5 * time.Millisecond)
			}
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
		result, meta, err := c.Get(context.Background(), "t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(t, err)
		require.Equal(t, 8, result)
		require.True(t, meta.Hit)
		// Age should be zero since background refresh was "active"
		require.Equal(t, time.Duration(0), meta.Age)
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
		result, meta, err := c.Get(context.Background(), "t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(t, err)
		require.Equal(t, 8, result)
		require.True(t, meta.Hit)
		// Age should be non-zero since background refresh was "active"
		require.True(t, meta.Age > 0)
		lastAge = meta.Age
	}
	// Wait a bit longer - age should increase by at least this much
	time.Sleep(5 * time.Millisecond)
	{
		result, meta, err := c.Get(context.Background(), "t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(t, err)
		require.Equal(t, 8, result)
		require.True(t, meta.Hit)
		require.True(t, meta.Age > (lastAge+(1*time.Millisecond)))
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
		result, meta, err := c.Get(context.Background(), "t", TestRequest(t, RequestInfo{Key: "hello"}))
		// Should never error even if background is failing as we have cached value
		require.NoError(t, err)
		require.True(t, meta.Hit)
		// Got the new value!
		if result == 10 {
			// Age should be zero since background refresh is "active" again
			t.Logf("Succeeded after %d attempts", attempts)
			require.Equal(t, time.Duration(0), meta.Age)
			timeout = false
			break
		}
	}
	require.False(t, timeout, "failed to observe update after %s", time.Since(t0))
}

func TestCacheGet_nonRefreshAge(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := &MockType{}
	typ.On("RegisterOptions").Return(RegisterOptions{
		Refresh:    false,
		LastGetTTL: 100 * time.Millisecond,
	})
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

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
		result, meta, err := c.Get(context.Background(), "t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(t, err)
		require.Equal(t, 8, result)
		require.True(t, meta.Hit)
		require.True(t, meta.Age > (5*time.Millisecond))
		lastAge = meta.Age
	}

	// Wait for expiry
	time.Sleep(200 * time.Millisecond)

	{
		result, meta, err := c.Get(context.Background(), "t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(t, err)
		require.Equal(t, 8, result)
		require.False(t, meta.Hit)
		// Age should smaller again
		require.True(t, meta.Age < lastAge)
	}

	{
		// Wait for a non-trivial amount of time to sanity check the age increases at
		// least this amount. Note that this is not a fudge for some timing-dependent
		// background work it's just ensuring a non-trivial time elapses between the
		// request above and below serilaly in this thread so short time is OK.
		time.Sleep(5 * time.Millisecond)

		// Fetch again, non-blocking
		result, meta, err := c.Get(context.Background(), "t", TestRequest(t, RequestInfo{Key: "hello"}))
		require.NoError(t, err)
		require.Equal(t, 8, result)
		require.True(t, meta.Hit)
		require.True(t, meta.Age > (5*time.Millisecond))
		lastAge = meta.Age
	}

	// Now verify that setting MaxAge results in cache invalidation
	{
		result, meta, err := c.Get(context.Background(), "t", TestRequest(t, RequestInfo{
			Key:    "hello",
			MaxAge: 1 * time.Millisecond,
		}))
		require.NoError(t, err)
		require.Equal(t, 8, result)
		require.False(t, meta.Hit)
		// Age should smaller again
		require.True(t, meta.Age < lastAge)
	}
}

func TestCacheGet_nonBlockingType(t *testing.T) {
	t.Parallel()

	typ := TestTypeNonBlocking(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	// Configure the type
	typ.Static(FetchResult{Value: 42, Index: 1}, nil).Once()
	typ.Static(FetchResult{Value: 43, Index: 2}, nil).Twice().
		Run(func(args mock.Arguments) {
			opts := args.Get(0).(FetchOptions)
			// MinIndex should never be set for a non-blocking type.
			require.Equal(t, uint64(0), opts.MinIndex)
		})

	// Get, should fetch
	req := TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err := c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.False(t, meta.Hit)

	// Get, should not fetch since we have a cached value
	req = TestRequest(t, RequestInfo{Key: "hello"})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.True(t, meta.Hit)

	// Get, should not attempt to fetch with blocking even if requested. The
	// assertions below about the value being the same combined with the fact the
	// mock will only return that value on first call suffice to show that
	// blocking request is not being attempted.
	req = TestRequest(t, RequestInfo{
		Key:      "hello",
		MinIndex: 1,
		Timeout:  10 * time.Minute,
	})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.True(t, meta.Hit)

	time.Sleep(10 * time.Millisecond)

	// Get with a max age should fetch again
	req = TestRequest(t, RequestInfo{Key: "hello", MaxAge: 5 * time.Millisecond})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 43, result)
	require.False(t, meta.Hit)

	// Get with a must revalidate should fetch again even without a delay.
	req = TestRequest(t, RequestInfo{Key: "hello", MustRevalidate: true})
	result, meta, err = c.Get(context.Background(), "t", req)
	require.NoError(t, err)
	require.Equal(t, 43, result)
	require.False(t, meta.Hit)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
	time.Sleep(20 * time.Millisecond)
	typ.AssertExpectations(t)
}

// Test a get with an index set will wait until an index that is higher
// is set in the cache.
func TestCacheReload(t *testing.T) {
	t.Parallel()

	typ1 := TestType(t)
	defer typ1.AssertExpectations(t)

	c := New(Options{EntryFetchRate: rate.Limit(1), EntryFetchMaxBurst: 1})
	c.RegisterType("t1", typ1)
	typ1.Mock.On("Fetch", mock.Anything, mock.Anything).Return(FetchResult{Value: 42, Index: 42}, nil).Maybe()

	require.False(t, c.ReloadOptions(Options{EntryFetchRate: rate.Limit(1), EntryFetchMaxBurst: 1}), "Value should not be reloaded")

	_, meta, err := c.Get(context.Background(), "t1", TestRequest(t, RequestInfo{Key: "hello1", MinIndex: uint64(1)}))
	require.NoError(t, err)
	require.Equal(t, meta.Index, uint64(42))

	testEntry := func(t *testing.T, doTest func(t *testing.T, entry cacheEntry)) {
		c.entriesLock.Lock()
		tEntry, ok := c.types["t1"]
		require.True(t, ok)
		keyName := makeEntryKey("t1", "", "", "", "hello1")
		ok, entryValid, entry := c.getEntryLocked(tEntry, keyName, RequestInfo{})
		require.True(t, ok)
		require.True(t, entryValid)
		doTest(t, entry)
		c.entriesLock.Unlock()

	}
	testEntry(t, func(t *testing.T, entry cacheEntry) {
		require.Equal(t, entry.FetchRateLimiter.Limit(), rate.Limit(1))
		require.Equal(t, entry.FetchRateLimiter.Burst(), 1)
	})

	// Modify only rateLimit
	require.True(t, c.ReloadOptions(Options{EntryFetchRate: rate.Limit(100), EntryFetchMaxBurst: 1}))
	testEntry(t, func(t *testing.T, entry cacheEntry) {
		require.Equal(t, entry.FetchRateLimiter.Limit(), rate.Limit(100))
		require.Equal(t, entry.FetchRateLimiter.Burst(), 1)
	})

	// Modify only Burst
	require.True(t, c.ReloadOptions(Options{EntryFetchRate: rate.Limit(100), EntryFetchMaxBurst: 5}))
	testEntry(t, func(t *testing.T, entry cacheEntry) {
		require.Equal(t, entry.FetchRateLimiter.Limit(), rate.Limit(100))
		require.Equal(t, entry.FetchRateLimiter.Burst(), 5)
	})

	// Modify only Burst and Limit at the same time
	require.True(t, c.ReloadOptions(Options{EntryFetchRate: rate.Limit(1000), EntryFetchMaxBurst: 42}))

	testEntry(t, func(t *testing.T, entry cacheEntry) {
		require.Equal(t, entry.FetchRateLimiter.Limit(), rate.Limit(1000))
		require.Equal(t, entry.FetchRateLimiter.Burst(), 42)
	})
}

// TestCacheThrottle checks the assumptions for the cache throttling. It sets
// up a cache with Options{EntryFetchRate: 10.0, EntryFetchMaxBurst: 1}, which
// allows for 10req/s, or one request every 100ms.
// It configures two different cache types with each 3 updates. Each type has
// its own rate limiter which starts initially full, and we expect the
// following requests when creating blocking queries against both waiting for
// the third update:
// at ~0ms: typ1 and typ2 receive first value and are blocked until
// ~100ms: typ1 and typ2 receive second value and are blocked until
// ~200ms: typ1 and typ2 receive third value which we check
//
// This test will verify waiting with a blocking query for the last update will
// block for 190ms and only afterwards have the expected result.
// It demonstrates the ratelimiting waits for the expected amount of time and
// also that each type has its own ratelimiter, because results for both types
// are arriving at similar times, which wouldn't be the case if they use a
// shared limiter.
func TestCacheThrottle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ1 := TestType(t)
	typ2 := TestType(t)
	defer typ1.AssertExpectations(t)
	defer typ2.AssertExpectations(t)

	c := New(Options{EntryFetchRate: 10.0, EntryFetchMaxBurst: 1})
	c.RegisterType("t1", typ1)
	c.RegisterType("t2", typ2)

	// Configure the type
	typ1.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ1.Static(FetchResult{Value: 12, Index: 5}, nil).Once()
	typ1.Static(FetchResult{Value: 42, Index: 6}, nil).Once()

	typ2.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ2.Static(FetchResult{Value: 12, Index: 5}, nil).Once()
	typ2.Static(FetchResult{Value: 43, Index: 6}, nil).Once()

	result1Ch := TestCacheGetCh(t, c, "t1", TestRequest(t, RequestInfo{
		Key: "hello1", MinIndex: 5}))

	result2Ch := TestCacheGetCh(t, c, "t2", TestRequest(t, RequestInfo{
		Key: "hello2", MinIndex: 5}))

	select {
	case <-result1Ch:
		t.Fatal("result1Ch should block")
	case <-result2Ch:
		t.Fatal("result2Ch should block")
	case <-time.After(190 * time.Millisecond):
	}

	after := time.After(30 * time.Millisecond)
	var res1, res2 bool
OUT:
	for {
		select {
		case result := <-result1Ch:
			require.Equal(t, 42, result)

			res1 = true
		case result := <-result2Ch:
			require.Equal(t, 43, result)
			res2 = true
		case <-after:
			t.Fatal("shouldn't block that long")
		}
		if res1 && res2 {
			break OUT
		}
	}
}

func TestCache_ExpiryLoop_ExitsWhenStopped(t *testing.T) {
	c := &Cache{
		stopCh:            make(chan struct{}),
		entries:           make(map[string]cacheEntry),
		entriesExpiryHeap: ttlcache.NewExpiryHeap(),
	}
	chStart := make(chan struct{})
	chDone := make(chan struct{})
	go func() {
		close(chStart)
		c.runExpiryLoop()
		close(chDone)
	}()

	<-chStart
	close(c.stopCh)

	select {
	case <-chDone:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected loop to exit when stopped")
	}
}

func TestCache_Prepopulate(t *testing.T) {
	typ := &fakeType{index: 5}
	c := New(Options{})
	c.RegisterType("t", typ)

	c.Prepopulate("t", FetchResult{Value: 17, Index: 1}, "dc1", "", "token", "v1")

	ctx := context.Background()
	req := fakeRequest{
		info: RequestInfo{
			Key:        "v1",
			Token:      "token",
			Datacenter: "dc1",
			MinIndex:   1,
		},
	}
	result, _, err := c.Get(ctx, "t", req)
	require.NoError(t, err)
	require.Equal(t, 17, result)
}

func TestCache_RefreshLifeCycle(t *testing.T) {
	typ := &MockType{}
	t.Cleanup(func() { typ.AssertExpectations(t) })

	typ.On("RegisterOptions").Times(0).Return(RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:          true,
		SupportsBlocking: true,
		RefreshTimer:     0 * time.Second,
		QueryTimeout:     10 * time.Minute,
	})

	makeRequest := func(index uint64) fakeRequest {
		return fakeRequest{
			info: RequestInfo{
				Key:        "v1",
				Token:      "token",
				Datacenter: "dc1",
				MinIndex:   index,
			},
		}
	}

	typ.On("Fetch", mock.Anything, mock.Anything).Once().Return(FetchResult{
		Value: true,
		Index: 2,
	}, nil)

	releaseSecondReq := make(chan time.Time)
	typ.On("Fetch", mock.Anything, mock.Anything).Once().Return(FetchResult{}, acl.PermissionDenied("forced error")).WaitUntil(releaseSecondReq)

	releaseThirdReq := make(chan time.Time)
	typ.On("Fetch", mock.Anything, mock.Anything).Once().Return(FetchResult{}, acl.ErrNotFound).WaitUntil(releaseThirdReq)

	c := New(Options{})
	c.RegisterType("t", typ)

	key := makeEntryKey("t", "dc1", "", "token", "v1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// get the background refresh going
	result, _, err := c.Get(ctx, "t", makeRequest(1))
	require.NoError(t, err)
	require.Equal(t, true, result)

	waitUntilFetching := func(expectValue bool) {
		retry.Run(t, func(t *retry.R) {
			c.entriesLock.Lock()
			defer c.entriesLock.Unlock()
			entry, ok := c.entries[key]
			require.True(t, ok)
			if expectValue {
				require.True(t, entry.Fetching)
			} else {
				require.False(t, entry.Fetching)
			}
		})
	}

	// ensure that the entry is fetching again
	waitUntilFetching(true)

	requestChan := make(chan error)

	getError := func(index uint64) {
		_, _, err := c.Get(ctx, "t", makeRequest(index))
		if ctx.Err() != nil {
			return
		}
		requestChan <- err
	}

	// background a call that will wait for a newer version
	go getError(2)

	// I really dislike the arbitrary sleep here. However we want to test out some of the
	// branching in getWithIndex (called by Get) and that doesn't expose any way for us to
	// know when that go routine has gotten far enough and is waiting on the latest value.
	// Therefore the only thing we can do for now is to sleep long enough to let that
	// go routine progress far enough.
	time.Sleep(100 * time.Millisecond)

	// release the blocking query to simulate an ACL permission denied error
	close(releaseSecondReq)

	// ensure we were woken up and see the permission denied error
	select {
	case err := <-requestChan:
		require.True(t, acl.IsErrPermissionDenied(err))
	case <-time.After(500 * time.Millisecond):
		require.Fail(t, "blocking cache Get never returned")
	}

	// ensure that the entry is fetching again
	waitUntilFetching(true)

	// background a call that will wait for a newer version - will result in an acl not found error
	go getError(5)

	// Same arbitrary sleep as the one after the second request and the same reasoning.
	time.Sleep(100 * time.Millisecond)

	// release the blocking query to simulate an ACL not found error
	close(releaseThirdReq)

	// ensure we were woken up and see the ACL not found error
	select {
	case err := <-requestChan:
		require.True(t, acl.IsErrNotFound(err))
	case <-time.After(500 * time.Millisecond):
		require.Fail(t, "blocking cache Get never returned")
	}

	// ensure that the ACL not found error killed off the background refresh
	// but didn't remove it from the cache
	waitUntilFetching(false)
}

type fakeType struct {
	index uint64
}

func (f fakeType) Fetch(_ FetchOptions, _ Request) (FetchResult, error) {
	idx := atomic.LoadUint64(&f.index)
	return FetchResult{Value: int(idx * 2), Index: idx}, nil
}

func (f fakeType) RegisterOptions() RegisterOptions {
	return RegisterOptions{Refresh: true}
}

var _ Type = (*fakeType)(nil)

type fakeRequest struct {
	info RequestInfo
}

func (f fakeRequest) CacheInfo() RequestInfo {
	return f.info
}

var _ Request = (*fakeRequest)(nil)
