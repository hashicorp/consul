package cache

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Test that a type registered with a periodic refresh can be watched.
func TestCacheNotify(t *testing.T) {
	t.Parallel()

	typ := TestType(t)
	typ.On("RegisterOptions").Return(RegisterOptions{})
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	// Setup triggers to control when "updates" should be delivered
	trigger := make([]chan time.Time, 5)
	for i := range trigger {
		trigger[i] = make(chan time.Time)
	}

	// Send an error to fake a situation where the servers aren't reachable
	// initially.
	typ.Static(FetchResult{Value: nil, Index: 0}, errors.New("no servers available")).Once()

	// Configure the type. The first time we use the fake index of "1" to verify we
	// don't regress on https://github.com/hashicorp/consul/issues/6521 .
	typ.Static(FetchResult{Value: 1, Index: 1}, nil).Once().Run(func(args mock.Arguments) {
		// Assert the right request type - all real Fetch implementations do this so
		// it keeps us honest that Watch doesn't require type mangling which will
		// break in real life (hint: it did on the first attempt)
		_, ok := args.Get(1).(*MockRequest)
		require.True(t, ok)
	}).WaitUntil(trigger[0])
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).Once().WaitUntil(trigger[1])
	typ.Static(FetchResult{Value: 12, Index: 5}, nil).Once().WaitUntil(trigger[2])
	typ.Static(FetchResult{Value: 42, Index: 7}, nil).Once().WaitUntil(trigger[3])
	// It's timing dependent whether the blocking loop manages to make another
	// call before we cancel so don't require it. We need to have a higher index
	// here because if the index is the same then the cache Get will not return
	// until the full 10 min timeout expires. This causes the last fetch to return
	// after cancellation as if it had timed out.
	typ.Static(FetchResult{Value: 42, Index: 8}, nil).WaitUntil(trigger[4])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan UpdateEvent)

	err := c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello"}), "test", ch)
	require.NoError(t, err)

	// Should receive the error with index == 0 first.
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test",
		Result:        nil,
		Meta:          ResultMeta{Hit: false, Index: 0},
		Err:           errors.New("no servers available"),
	})

	// There should be no more updates delivered yet
	require.Len(t, ch, 0)

	// Trigger blocking query to return a "change"
	close(trigger[0])

	// Should receive the first real update next.
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test",
		Result:        1,
		Meta:          ResultMeta{Hit: false, Index: 1},
		Err:           nil,
	})

	// Trigger blocking query to return a "change"
	close(trigger[1])

	// Should receive the next result pretty soon
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test",
		Result:        12,
		// Note these are never cache "hits" because blocking will wait until there
		// is a new value at which point it's not considered a hit.
		Meta: ResultMeta{Hit: false, Index: 5},
		Err:  nil,
	})

	// Register a second observer using same chan and request. Note that this is
	// testing a few things implicitly:
	//  - that multiple watchers on the same cache entity are de-duped in their
	//    requests to the "backend"
	//  - that multiple watchers can distinguish their results using correlationID
	err = c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello"}), "test2", ch)
	require.NoError(t, err)

	// Should get test2 notify immediately, and it should be a cache hit
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test2",
		Result:        12,
		Meta:          ResultMeta{Hit: true, Index: 5},
		Err:           nil,
	})

	// We could wait for a full timeout but we can't directly observe it so
	// simulate the behavior by triggering a response with the same value and
	// index as the last one.
	close(trigger[2])

	// We should NOT be notified about that. Note this is timing dependent but
	// it's only a sanity check, if we somehow _do_ get the change delivered later
	// than 10ms the next value assertion will fail anyway.
	time.Sleep(10 * time.Millisecond)
	require.Len(t, ch, 0)

	// Trigger final update
	close(trigger[3])

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

	// Sanity check closing chan before context is canceled doesn't panic
	//close(ch)

	// Close context
	cancel()

	// It's likely but not certain that at least one of the watchers was blocked
	// on the next cache Get so trigger that to timeout so we can observe the
	// watch goroutines being cleaned up. This is necessary since currently we
	// have no way to interrupt a blocking query. In practice it's fine to know
	// that after 10 mins max the blocking query will return and the resources
	// will be cleaned.
	close(trigger[4])

	// I want to test that canceling the context cleans up goroutines (which it
	// does from manual verification with debugger etc). I had a check based on a
	// similar approach to https://golang.org/src/net/http/main_test.go#L60 but it
	// was just too flaky because it relies on the timing of the error backoff
	// timer goroutines and similar so I've given up for now as I have more
	// important things to get working.
}

func TestCacheNotifyPolling(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := TestTypeNonBlocking(t)
	defer typ.AssertExpectations(t)
	c := New(Options{})
	c.RegisterType("t", typ)

	// Configure the type
	typ.Static(FetchResult{Value: 1, Index: 1}, nil).Once().Run(func(args mock.Arguments) {
		// Assert the right request type - all real Fetch implementations do this so
		// it keeps us honest that Watch doesn't require type mangling which will
		// break in real life (hint: it did on the first attempt)
		_, ok := args.Get(1).(*MockRequest)
		require.True(t, ok)
	})
	typ.Static(FetchResult{Value: 12, Index: 1}, nil).Once()
	typ.Static(FetchResult{Value: 42, Index: 1}, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan UpdateEvent)

	err := c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello", MaxAge: 100 * time.Millisecond}), "test", ch)
	require.NoError(t, err)

	// Should receive the first result pretty soon
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test",
		Result:        1,
		Meta:          ResultMeta{Hit: false, Index: 1},
		Err:           nil,
	})

	// There should be no more updates delivered yet
	require.Len(t, ch, 0)

	// make sure the updates do not come too quickly
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ch:
		require.Fail(t, "Received update too early")
	}

	// make sure we get the update not too far out.
	select {
	case <-time.After(100 * time.Millisecond):
		require.Fail(t, "Didn't receive the notification")
	case result := <-ch:
		require.Equal(t, result.Result, 12)
		require.Equal(t, result.CorrelationID, "test")
		require.Equal(t, result.Meta.Hit, false)
		require.Equal(t, result.Meta.Index, uint64(1))
		// pretty conservative check it should be even newer because without a second
		// notifier each value returned will have been executed just then and not served
		// from the cache.
		require.True(t, result.Meta.Age < 50*time.Millisecond)
		require.NoError(t, result.Err)
	}

	require.Len(t, ch, 0)

	// Register a second observer using same chan and request. Note that this is
	// testing a few things implicitly:
	//  - that multiple watchers on the same cache entity are de-duped in their
	//    requests to the "backend"
	//  - that multiple watchers can distinguish their results using correlationID
	err = c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello", MaxAge: 100 * time.Millisecond}), "test2", ch)
	require.NoError(t, err)

	// Should get test2 notify immediately, and it should be a cache hit
	TestCacheNotifyChResult(t, ch, UpdateEvent{
		CorrelationID: "test2",
		Result:        12,
		Meta:          ResultMeta{Hit: true, Index: 1},
		Err:           nil,
	})

	require.Len(t, ch, 0)

	// wait for the next batch of responses
	events := make([]UpdateEvent, 0)
	// At least 110ms is needed to allow for the jitter
	timeout := time.After(220 * time.Millisecond)

	for i := 0; i < 2; i++ {
		select {
		case <-timeout:
			require.Fail(t, "UpdateEvent not received in time")
		case eve := <-ch:
			events = append(events, eve)
		}
	}

	require.Equal(t, events[0].Result, 42)
	require.Equal(t, events[0].Meta.Hit && events[1].Meta.Hit, false)
	require.Equal(t, events[0].Meta.Index, uint64(1))
	require.True(t, events[0].Meta.Age < 50*time.Millisecond)
	require.NoError(t, events[0].Err)
	require.Equal(t, events[1].Result, 42)
	// Sometimes this would be a hit and others not. It all depends on when the various getWithIndex calls got fired.
	// If both are done concurrently then it will not be a cache hit but the request gets single flighted and both
	// get notified at the same time.
	// require.Equal(t,events[1].Meta.Hit, true)
	require.Equal(t, events[1].Meta.Index, uint64(1))
	require.True(t, events[1].Meta.Age < 100*time.Millisecond)
	require.NoError(t, events[1].Err)
}

// Test that a refresh performs a backoff.
func TestCacheWatch_ErrorBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := TestType(t)
	typ.On("RegisterOptions").Return(RegisterOptions{})
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan UpdateEvent)

	err := c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello"}), "test", ch)
	require.NoError(t, err)

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
	// backoff actually taking effect. We can validate that by ensuring this test
	// fails without the backoff code reliably.
	timeoutC := time.After(500 * time.Millisecond)
OUT:
	for {
		select {
		case <-timeoutC:
			break OUT
		case u := <-ch:
			numErrors++
			require.Error(t, u.Err)
		}
	}
	// Must be fewer than 10 failures in that time
	require.True(t, numErrors < 10, fmt.Sprintf("numErrors: %d", numErrors))

	// Check the number of RPCs as a sanity check too
	actual := atomic.LoadUint32(&retries)
	require.True(t, actual < 10, fmt.Sprintf("actual: %d", actual))
}

// Test that a refresh performs a backoff.
func TestCacheWatch_ErrorBackoffNonBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	typ := TestTypeNonBlocking(t)
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan UpdateEvent)

	err := c.Notify(ctx, "t", TestRequest(t, RequestInfo{Key: "hello", MaxAge: 100 * time.Millisecond}), "test", ch)
	require.NoError(t, err)

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
	// backoff actually taking effect. We can validate that by ensuring this test
	// fails without the backoff code reliably.
	//
	// 100 + 500 milliseconds. 100 because the first retry will not happen until
	// the 100 + jitter milliseconds have elapsed.
	timeoutC := time.After(600 * time.Millisecond)
OUT:
	for {
		select {
		case <-timeoutC:
			break OUT
		case u := <-ch:
			numErrors++
			require.Error(t, u.Err)
		}
	}
	// Must be fewer than 10 failures in that time
	require.True(t, numErrors < 10, fmt.Sprintf("numErrors: %d", numErrors))

	// Check the number of RPCs as a sanity check too
	actual := atomic.LoadUint32(&retries)
	require.True(t, actual < 10, fmt.Sprintf("actual: %d", actual))
}

func Test_isEqual(t *testing.T) {
	s1 := time.Now()
	s2 := s1

	var pb1 proto.Message = timestamppb.New(s1)
	var pb2 proto.Message = &timestamppb.Timestamp{}

	// Marshal and unmarshal the proto object
	b, err := proto.Marshal(pb1)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(b, pb2))

	// Sanity-check that protobuf comparisons fail reflect.DeepEqual
	require.False(t, reflect.DeepEqual(pb1, pb2))

	tests := []struct {
		equal bool
		v1    interface{}
		v2    interface{}
	}{
		// Test protobuf message comparisons work.
		{true, nil, nil},
		{true, pb1, pb2},
		{false, nil, pb2},
		{false, pb1, nil},
		{false, pb1, timestamppb.New(s1.Add(time.Second))},
		// Test normal struct comparisons
		{true, s1, s2},
		{true, &s1, &s2},
		{false, s1, nil},
		{false, &s1, nil},
		{false, nil, s2},
		{false, nil, &s2},
		{false, s1, s1.Add(time.Second)},
	}

	for _, tc := range tests {
		require.Equal(t, tc.equal, isEqual(tc.v1, tc.v2))
	}
}
