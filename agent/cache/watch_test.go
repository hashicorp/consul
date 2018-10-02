package cache

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test that a type registered with a periodic refresh can be watched.
func TestCacheNotify(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh: false,
	})

	// Setup triggers to control when "updates" should be delivered
	trigger := make([]chan time.Time, 4)
	for i := range trigger {
		trigger[i] = make(chan time.Time)
	}

	// Configure the type
	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Once().Run(func(args mock.Arguments) {
		// Assert the right request type - all real Fetch implementations do this so
		// it keeps us honest that Watch doesn't require type mangling which will
		// break in real life (hint: it did on the first attempt)
		_, ok := args.Get(1).(*MockRequest)
		require.True(t, ok)
	})
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).Once().WaitUntil(trigger[0])
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).Once().WaitUntil(trigger[1])
	typ.Static(FetchResult{Value: 42, Index: 7}, nil).Once().WaitUntil(trigger[2])
	// It's timing dependent whether the blocking loop manages to make another
	// call before we cancel so don't require it. We need to have a higher index
	// here because if the index is the same then the cache Get will not return
	// until the full 10 min timeout expires. This causes the last fetch to return
	// after cancellation as if it had timed out.
	typ.Static(FetchResult{Value: 42, Index: 8}, nil).WaitUntil(trigger[3])

	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan UpdateEvent)

	err := c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello"}), "test", ch)
	require.NoError(err)

	// Should receive the first result pretty soon
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test",
		Result:        1,
		Meta:          ResultMeta{Hit: false, Index: 4},
		Err:           nil,
	})

	// There should be no more updates delivered yet
	require.Len(ch, 0)

	// Trigger blocking query to return a "change"
	close(trigger[0])

	// Should receive the next result pretty soon
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test",
		Result:        12,
		// Note these are never cache "hits" because blocking will wait until there
		// is a new value at which point it's not considered a hit.
		Meta: ResultMeta{Hit: false, Index: 5},
		Err:  nil,
	})

	// Registere a second observer using same chan and request. Note that this is
	// testing a few things implicitly:
	//  - that multiple watchers on the same cache entity are de-duped in their
	//    requests to the "backend"
	//  - that multiple watchers can distinguish their results using correlationID
	err = c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello"}), "test2", ch)
	require.NoError(err)

	// Should get test2 notify immediately, and it should be a cache hit
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test2",
		Result:        12,
		Meta:          ResultMeta{Hit: true, Index: 5},
		Err:           nil,
	})

	// We could wait for a full timeout but we can't directly observe it so
	// simulate the behaviour by triggering a response with the same value and
	// index as the last one.
	close(trigger[1])

	// We should NOT be notified about that. Note this is timing dependent but
	// it's only a sanity check, if we somehow _do_ get the change delivered later
	// than 10ms the next value assertion will fail anyway.
	time.Sleep(10 * time.Millisecond)
	require.Len(ch, 0)

	// Trigger final update
	close(trigger[2])

	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test",
		Result:        42,
		Meta:          ResultMeta{Hit: false, Index: 7},
		Err:           nil,
	}, UpdateEvent{
		CorrelationID: "test2",
		Result:        42,
		Meta:          ResultMeta{Hit: false, Index: 7},
		Err:           nil,
	})

	// Sanity check closing chan before context is cancelled doesn't panic
	//close(ch)

	// Close context
	cancel()

	// It's likely but not certain that at least one of the watchers was blocked
	// on the next cache Get so trigger that to timeout so we can observe the
	// watch goroutines being cleaned up. This is necessary since currently we
	// have no way to interrupt a blocking query. In practice it's fine to know
	// that after 10 mins max the blocking query will return and the resources
	// will be cleaned.
	close(trigger[3])

	// I want to test that cancelling the context cleans up goroutines (which it
	// does from manual verification with debugger etc). I had a check based on a
	// similar approach to https://golang.org/src/net/http/main_test.go#L60 but it
	// was just too flaky because it relies on the timing of the error backoff
	// timer goroutines and similar so I've given up for now as I have more
	// important things to get working.
}

// Test that a refresh performs a backoff.
func TestCacheWatch_ErrorBackoff(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	defer typ.AssertExpectations(t)
	c := TestCache(t)
	c.RegisterType("t", typ, &RegisterOptions{
		Refresh: false,
	})

	// Configure the type
	var retries uint32
	fetchErr := fmt.Errorf("test fetch error")
	typ.Static(FetchResult{Value: 1, Index: 4}, nil).Once()
	typ.Static(FetchResult{Value: nil, Index: 5}, fetchErr).Run(func(args mock.Arguments) {
		atomic.AddUint32(&retries, 1)
	})

	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan UpdateEvent)

	err := c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello"}), "test", ch)
	require.NoError(err)

	// Should receive the first result pretty soon
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test",
		Result:        1,
		Meta:          ResultMeta{Hit: false, Index: 4},
		Err:           nil,
	})

	numErrors := 0
	// Loop for a little while and count how many errors we see reported. If this
	// was running as fast as it could go we'd expect this to be huge. We have to
	// be a little careful here because the watch chan ch doesn't have a large
	// buffer so we could be artificially slowing down the loop without the
	// backoff actualy taking affect. We can validate that by ensuring this test
	// fails without the backoff code reliably.
	timeoutC := time.After(500 * time.Millisecond)
OUT:
	for {
		select {
		case <-timeoutC:
			break OUT
		case u := <-ch:
			numErrors++
			require.Error(u.Err)
		}
	}
	// Must be fewer than 10 failures in that time
	require.True(numErrors < 10, fmt.Sprintf("numErrors: %d", numErrors))

	// Check the number of RPCs as a sanity check too
	actual := atomic.LoadUint32(&retries)
	require.True(actual < 10, fmt.Sprintf("actual: %d", actual))
}
