package cache

import (
	"sort"
	"sync"
	"testing"
	"time"

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
	result, err := c.Get("t", req)
	require.Nil(err)
	require.Equal(42, result)

	// Get, should not fetch since we already have a satisfying value
	result, err = c.Get("t", req)
	require.Nil(err)
	require.Equal(42, result)

	// Sleep a tiny bit just to let maybe some background calls happen
	// then verify that we still only got the one call
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
	result, err := c.Get("t", req)
	require.Nil(err)
	require.Equal(42, result)

	// Get, should not fetch since we already have a satisfying value
	result, err = c.Get("t", req)
	require.Nil(err)
	require.Equal(42, result)

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
