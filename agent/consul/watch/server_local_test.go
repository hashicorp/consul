// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"context"
	"fmt"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/lib/retry"
)

type mockStoreProvider struct {
	mock.Mock
}

func newMockStoreProvider(t *testing.T) *mockStoreProvider {
	t.Helper()
	provider := &mockStoreProvider{}
	t.Cleanup(func() {
		provider.AssertExpectations(t)
	})
	return provider
}

func (m *mockStoreProvider) getStore() *MockStateStore {
	return m.Called().Get(0).(*MockStateStore)
}

type testResult struct {
	value string
}

func (m *mockStoreProvider) query(ws memdb.WatchSet, store *MockStateStore) (uint64, *testResult, error) {
	ret := m.Called(ws, store)

	index := ret.Get(0).(uint64)
	result := ret.Get(1).(*testResult)
	err := ret.Error(2)

	return index, result, err
}

func (m *mockStoreProvider) notify(ctx context.Context, correlationID string, result *testResult, err error) {
	m.Called(ctx, correlationID, result, err)
}

func TestServerLocalBlockingQuery_getStoreNotProvided(t *testing.T) {
	_, _, err := ServerLocalBlockingQuery(
		context.Background(),
		nil,
		0,
		true,
		func(memdb.WatchSet, *MockStateStore) (uint64, struct{}, error) {
			return 0, struct{}{}, nil
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "no getStore function was provided")
}

func TestServerLocalBlockingQuery_queryNotProvided(t *testing.T) {
	var query func(memdb.WatchSet, *MockStateStore) (uint64, struct{}, error)
	_, _, err := ServerLocalBlockingQuery(
		context.Background(),
		func() *MockStateStore { return nil },
		0,
		true,
		query,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "no query function was provided")
}

func TestServerLocalBlockingQuery_NonBlocking(t *testing.T) {
	abandonCh := make(chan struct{})
	t.Cleanup(func() { close(abandonCh) })

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Once()

	provider := newMockStoreProvider(t)
	provider.On("getStore").Return(store).Once()
	provider.On("query", mock.Anything, store).
		Return(uint64(1), &testResult{value: "foo"}, nil).
		Once()

	idx, result, err := ServerLocalBlockingQuery(
		context.Background(),
		provider.getStore,
		0,
		true,
		provider.query,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, idx)
	require.Equal(t, &testResult{value: "foo"}, result)
}

func TestServerLocalBlockingQuery_Index0(t *testing.T) {
	abandonCh := make(chan struct{})
	t.Cleanup(func() { close(abandonCh) })

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Once()

	provider := newMockStoreProvider(t)
	provider.On("getStore").Return(store).Once()
	provider.On("query", mock.Anything, store).
		// the index 0 returned here should get translated to 1 by ServerLocalBlockingQuery
		Return(uint64(0), &testResult{value: "foo"}, nil).
		Once()

	idx, result, err := ServerLocalBlockingQuery(
		context.Background(),
		provider.getStore,
		0,
		true,
		provider.query,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, idx)
	require.Equal(t, &testResult{value: "foo"}, result)
}

func TestServerLocalBlockingQuery_NotFound(t *testing.T) {
	abandonCh := make(chan struct{})
	t.Cleanup(func() { close(abandonCh) })

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Once()

	provider := newMockStoreProvider(t)
	provider.On("getStore").
		Return(store).
		Once()

	var nilResult *testResult
	provider.On("query", mock.Anything, store).
		Return(uint64(1), nilResult, ErrorNotFound).
		Once()

	idx, result, err := ServerLocalBlockingQuery(
		context.Background(),
		provider.getStore,
		0,
		true,
		provider.query,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, idx)
	require.Nil(t, result)
}

func TestServerLocalBlockingQuery_NotFoundBlocks(t *testing.T) {
	abandonCh := make(chan struct{})
	t.Cleanup(func() { close(abandonCh) })

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Times(5)

	provider := newMockStoreProvider(t)
	provider.On("getStore").
		Return(store).
		Times(3)

	var nilResult *testResult
	// Initial data returned is not found and has an index less than the original
	// blocking index. This should not return data to the caller.
	provider.On("query", mock.Anything, store).
		Return(uint64(4), nilResult, ErrorNotFound).
		Run(addReadyWatchSet).
		Once()
	// There is an update to the data but the value still doesn't exist. Therefore
	// we should not return data to the caller.
	provider.On("query", mock.Anything, store).
		Return(uint64(6), nilResult, ErrorNotFound).
		Run(addReadyWatchSet).
		Once()
	// Finally we have some real data and can return it to the caller.
	provider.On("query", mock.Anything, store).
		Return(uint64(7), &testResult{value: "foo"}, nil).
		Once()

	idx, result, err := ServerLocalBlockingQuery(
		context.Background(),
		provider.getStore,
		5,
		true,
		provider.query,
	)
	require.NoError(t, err)
	require.EqualValues(t, 7, idx)
	require.Equal(t, &testResult{value: "foo"}, result)
}

func TestServerLocalBlockingQuery_Error(t *testing.T) {
	abandonCh := make(chan struct{})
	t.Cleanup(func() { close(abandonCh) })

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Once()

	provider := newMockStoreProvider(t)
	provider.On("getStore").
		Return(store).
		Once()

	var nilResult *testResult
	provider.On("query", mock.Anything, store).
		Return(uint64(10), nilResult, fmt.Errorf("synthetic error")).
		Once()

	idx, result, err := ServerLocalBlockingQuery(
		context.Background(),
		provider.getStore,
		4,
		true,
		provider.query,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "synthetic error")
	require.EqualValues(t, 10, idx)
	require.Nil(t, result)
}

func TestServerLocalBlockingQuery_ContextCancellation(t *testing.T) {
	abandonCh := make(chan struct{})
	t.Cleanup(func() { close(abandonCh) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Once()

	provider := newMockStoreProvider(t)
	provider.On("getStore").
		Return(store).
		Once()
	provider.On("query", mock.Anything, store).
		// Return an index that should not cause the blocking query to return.
		Return(uint64(4), &testResult{value: "foo"}, nil).
		Once().
		Run(func(_ mock.Arguments) {
			// Cancel the context so that the memdb WatchCtx call will error.
			cancel()
		})

	idx, result, err := ServerLocalBlockingQuery(
		ctx,
		provider.getStore,
		8,
		true,
		provider.query,
	)
	// The internal cancellation error should not be propagated.
	require.NoError(t, err)
	require.EqualValues(t, 4, idx)
	require.Equal(t, &testResult{value: "foo"}, result)
}

func TestServerLocalBlockingQuery_StateAbandoned(t *testing.T) {
	abandonCh := make(chan struct{})

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Twice()

	provider := newMockStoreProvider(t)
	provider.On("getStore").
		Return(store).
		Once()
	provider.On("query", mock.Anything, store).
		// Return an index that should not cause the blocking query to return.
		Return(uint64(4), &testResult{value: "foo"}, nil).
		Once().
		Run(func(_ mock.Arguments) {
			// Cancel the context so that the memdb WatchCtx call will error.
			close(abandonCh)
		})

	idx, result, err := ServerLocalBlockingQuery(
		context.Background(),
		provider.getStore,
		8,
		true,
		provider.query,
	)
	// The internal cancellation error should not be propagated.
	require.NoError(t, err)
	require.EqualValues(t, 4, idx)
	require.Equal(t, &testResult{value: "foo"}, result)
}

func TestServerLocalNotify_Validations(t *testing.T) {
	provider := newMockStoreProvider(t)

	type testCase struct {
		ctx      context.Context
		getStore func() *MockStateStore
		query    func(memdb.WatchSet, *MockStateStore) (uint64, *testResult, error)
		notify   func(context.Context, string, *testResult, error)
		err      error
	}

	cases := map[string]testCase{
		"nil-context": {
			getStore: provider.getStore,
			query:    provider.query,
			notify:   provider.notify,
			err:      errNilContext,
		},
		"nil-getStore": {
			ctx:    context.Background(),
			query:  provider.query,
			notify: provider.notify,
			err:    errNilGetStore,
		},
		"nil-query": {
			ctx:      context.Background(),
			getStore: provider.getStore,
			notify:   provider.notify,
			err:      errNilQuery,
		},
		"nil-notify": {
			ctx:      context.Background(),
			getStore: provider.getStore,
			query:    provider.query,
			err:      errNilNotify,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := ServerLocalNotify(tcase.ctx, "test", tcase.getStore, tcase.query, tcase.notify)
			require.ErrorIs(t, err, tcase.err)
		})
	}
}

func TestServerLocalNotify(t *testing.T) {
	notifyCtx, notifyCancel := context.WithCancel(context.Background())
	t.Cleanup(notifyCancel)

	abandonCh := make(chan struct{})

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Times(3)

	provider := newMockStoreProvider(t)
	provider.On("getStore").
		Return(store).
		Times(3)
	provider.On("query", mock.Anything, store).
		Return(uint64(4), &testResult{value: "foo"}, nil).
		Once()
	provider.On("notify", notifyCtx, t.Name(), &testResult{value: "foo"}, nil).Once()
	provider.On("query", mock.Anything, store).
		Return(uint64(6), &testResult{value: "bar"}, nil).
		Once()
	provider.On("notify", notifyCtx, t.Name(), &testResult{value: "bar"}, nil).Once()
	provider.On("query", mock.Anything, store).
		Return(uint64(7), &testResult{value: "baz"}, context.Canceled).
		Run(func(mock.Arguments) {
			notifyCancel()
		})

	doneCtx, routineDone := context.WithCancel(context.Background())
	err := serverLocalNotify(notifyCtx, t.Name(), provider.getStore, provider.query, provider.notify, routineDone, defaultWaiter())
	require.NoError(t, err)

	// Wait for the context cancellation which will happen when the "query" func is run the third time. The doneCtx gets "cancelled"
	// by the backgrounded go routine when it is actually finished. We need to wait for this to ensure that all mocked calls have been
	// made and that no extra calls get made.
	<-doneCtx.Done()
}

func TestServerLocalNotify_internal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	abandonCh := make(chan struct{})

	store := NewMockStateStore(t)
	store.On("AbandonCh").
		Return(closeChan(abandonCh)).
		Times(4)

	var nilResult *testResult

	provider := newMockStoreProvider(t)
	provider.On("getStore").
		Return(store).
		Times(4)
	provider.On("query", mock.Anything, store).
		Return(uint64(0), nilResult, fmt.Errorf("injected error")).
		Times(3)
	// we should only notify the first time as the index of 1 wont exceed the min index
	// after the second two queries.
	provider.On("notify", ctx, "test", nilResult, fmt.Errorf("injected error")).
		Once()
	provider.On("query", mock.Anything, store).
		Return(uint64(7), &testResult{value: "foo"}, nil).
		Once()
	provider.On("notify", ctx, "test", &testResult{value: "foo"}, nil).
		Once().
		Run(func(mock.Arguments) {
			cancel()
		})
	waiter := retry.Waiter{
		MinFailures: 1,
		MinWait:     time.Millisecond,
		MaxWait:     50 * time.Millisecond,
		Jitter:      retry.NewJitter(100),
		Factor:      2 * time.Millisecond,
	}

	// all the mock expectations should ensure things are working properly
	serverLocalNotifyRoutine(ctx, "test", provider.getStore, provider.query, provider.notify, noopDone, &waiter)
}

func addReadyWatchSet(args mock.Arguments) {
	ws := args.Get(0).(memdb.WatchSet)
	ch := make(chan struct{})
	ws.Add(ch)
	close(ch)
}

// small convenience to make this more readable. The alternative in a few
// cases would be to do something like (<-chan struct{})(ch). I find that
// syntax very difficult to read.
func closeChan(ch chan struct{}) <-chan struct{} {
	return ch
}
