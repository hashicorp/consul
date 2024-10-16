// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package blockingquery

import (
	"fmt"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestServer_blockingQuery(t *testing.T) {
	t.Parallel()
	getFSM := func(t *testing.T, additionalCfgFunc func(mockFSM *MockFSMServer)) *MockFSMServer {
		fsm := NewMockFSMServer(t)
		testCh := make(chan struct{})
		tombstoneGC, err := state.NewTombstoneGC(time.Second, time.Second)
		require.NoError(t, err)
		store := state.NewStateStore(tombstoneGC)
		fsm.On("GetShutdownChannel").Return(testCh)
		fsm.On("GetState").Return(store)
		fsm.On("SetQueryMeta", mock.Anything, mock.Anything).Return(nil)
		if additionalCfgFunc != nil {
			additionalCfgFunc(fsm)
		}
		return fsm
	}

	getOpts := func(t *testing.T, additionalCfgFunc func(options *MockRequestOptions)) *MockRequestOptions {
		requestOpts := NewMockRequestOptions(t)
		requestOpts.On("GetRequireConsistent").Return(false)
		requestOpts.On("GetToken").Return("fake-token")
		if additionalCfgFunc != nil {
			additionalCfgFunc(requestOpts)
		}
		return requestOpts
	}

	getMeta := func(t *testing.T, additionalCfgFunc func(mockMeta *MockResponseMeta)) *MockResponseMeta {
		meta := NewMockResponseMeta(t)
		if additionalCfgFunc != nil {
			additionalCfgFunc(meta)
		}
		return meta
	}

	// Perform a non-blocking query. Note that it's significant that the meta has
	// a zero index in response - the implied opts.MinQueryIndex is also zero but
	// this should not block still.
	t.Run("non-blocking query", func(t *testing.T) {
		var calls int
		fn := func(_ memdb.WatchSet, _ *state.Store) error {
			calls++
			return nil
		}
		err := Query(getFSM(t, nil), getOpts(t, func(mockOpts *MockRequestOptions) {
			mockOpts.On("GetMinQueryIndex").Return(uint64(0))
		}), getMeta(t, nil), fn)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	// Perform a blocking query that gets woken up and loops around once.
	t.Run("blocking query - single loop", func(t *testing.T) {
		opts := getOpts(t, func(options *MockRequestOptions) {
			options.On("GetMinQueryIndex").Return(uint64(1))
			options.On("GetMaxQueryTime").Return(1*time.Second, nil)
		})

		meta := getMeta(t, func(mockMeta *MockResponseMeta) {
			mockMeta.On("GetIndex").Return(uint64(1))
		})

		fsm := getFSM(t, func(mockFSM *MockFSMServer) {
			mockFSM.On("RPCQueryTimeout", mock.Anything).Return(1 * time.Second)
			mockFSM.On("IncrementBlockingQueries").Return(uint64(1))
			mockFSM.On("DecrementBlockingQueries").Return(uint64(1))
		})

		var calls int
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			if calls == 0 {
				meta.On("GetIndex").Return(uint64(3))

				fakeCh := make(chan struct{})
				close(fakeCh)
				ws.Add(fakeCh)
			} else {
				meta.On("GetIndex").Return(uint64(4))
			}
			calls++
			return nil
		}
		err := Query(fsm, opts, meta, fn)
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	// Perform a blocking query that returns a zero index from blocking func (e.g.
	// no state yet). This should still return an empty response immediately, but
	// with index of 1 and then block on the next attempt. In one sense zero index
	// is not really a valid response from a state method that is not an error but
	// in practice a lot of state store operations do return it unless they
	// explicitly special checks to turn 0 into 1. Often this is not caught or
	// covered by tests but eventually when hit in the wild causes blocking
	// clients to busy loop and burn CPU. This test ensure that blockingQuery
	// systematically does the right thing to prevent future bugs like that.
	t.Run("blocking query with 0 modifyIndex from state func", func(t *testing.T) {
		opts := getOpts(t, func(options *MockRequestOptions) {
			options.On("GetMinQueryIndex").Return(uint64(0))
		})

		meta := getMeta(t, func(mockMeta *MockResponseMeta) {
			mockMeta.On("GetIndex").Return(uint64(1))
		})

		fsm := getFSM(t, func(mockFSM *MockFSMServer) {
			mockFSM.On("RPCQueryTimeout", mock.Anything).Return(1 * time.Second)
			mockFSM.On("IncrementBlockingQueries").Return(uint64(1))
			mockFSM.On("DecrementBlockingQueries").Return(uint64(1))
		})
		var calls int
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			if opts.GetMinQueryIndex() > 0 {
				// If client requested blocking, block forever. This is simulating
				// waiting for the watched resource to be initialized/written to giving
				// it a non-zero index. Note the timeout on the query options is relied
				// on to stop the test taking forever.
				fakeCh := make(chan struct{})
				ws.Add(fakeCh)
			}
			meta.On("GetIndex").Return(uint64(0))
			calls++
			return nil
		}
		err := Query(fsm, opts, meta, fn)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Equal(t, uint64(1), meta.GetIndex(),
			"expect fake index of 1 to force client to block on next update")

		// Simulate client making next request
		opts = getOpts(t, func(options *MockRequestOptions) {
			options.On("GetMinQueryIndex").Return(uint64(1))
			options.On("GetMaxQueryTime").Return(20*time.Millisecond, nil)
		})

		// This time we should block even though the func returns index 0 still
		t0 := time.Now()
		require.NoError(t, Query(fsm, opts, meta, fn))
		t1 := time.Now()
		require.Equal(t, 2, calls)
		require.Equal(t, uint64(1), meta.GetIndex(),
			"expect fake index of 1 to force client to block on next update")
		require.True(t, t1.Sub(t0) > 20*time.Millisecond,
			"should have actually blocked waiting for timeout")

	})

	// Perform a query that blocks and gets interrupted when the state store
	// is abandoned.
	t.Run("blocking query interrupted by abandonCh", func(t *testing.T) {
		opts := getOpts(t, func(options *MockRequestOptions) {
			options.On("GetMinQueryIndex").Return(uint64(3))
			options.On("GetMaxQueryTime").Return(20*time.Millisecond, nil)
		})

		meta := getMeta(t, func(mockMeta *MockResponseMeta) {
			mockMeta.On("GetIndex").Return(uint64(1))
		})

		fsm := getFSM(t, func(mockFSM *MockFSMServer) {
			mockFSM.On("RPCQueryTimeout", mock.Anything).Return(1 * time.Second)
			mockFSM.On("IncrementBlockingQueries").Return(uint64(1))
			mockFSM.On("DecrementBlockingQueries").Return(uint64(1))
		})

		var calls int
		fn := func(_ memdb.WatchSet, _ *state.Store) error {
			if calls == 0 {
				meta.On("GetIndex").Return(uint64(1))

				fsm.GetState().Abandon()
			}
			calls++
			return nil
		}
		err := Query(fsm, opts, meta, fn)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	t.Run("non-blocking query for item that does not exist", func(t *testing.T) {
		opts := getOpts(t, func(options *MockRequestOptions) {
			options.On("GetMinQueryIndex").Return(uint64(3))
			options.On("GetMaxQueryTime").Return(20*time.Millisecond, nil)
		})

		meta := getMeta(t, func(mockMeta *MockResponseMeta) {
			mockMeta.On("GetIndex").Return(uint64(1))
		})

		fsm := getFSM(t, func(mockFSM *MockFSMServer) {
			mockFSM.On("RPCQueryTimeout", mock.Anything).Return(1 * time.Second)
			mockFSM.On("IncrementBlockingQueries").Return(uint64(1))
			mockFSM.On("DecrementBlockingQueries").Return(uint64(1))
		})
		calls := 0
		fn := func(_ memdb.WatchSet, _ *state.Store) error {
			calls++
			return ErrNotFound
		}

		err := Query(fsm, opts, meta, fn)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	t.Run("blocking query for item that does not exist", func(t *testing.T) {
		opts := getOpts(t, func(options *MockRequestOptions) {
			options.On("GetMinQueryIndex").Return(uint64(3))
			options.On("GetMaxQueryTime").Return(100*time.Millisecond, nil)
		})

		meta := getMeta(t, func(mockMeta *MockResponseMeta) {
			mockMeta.On("GetIndex").Return(uint64(1))
		})

		fsm := getFSM(t, func(mockFSM *MockFSMServer) {
			mockFSM.On("RPCQueryTimeout", mock.Anything).Return(1 * time.Second)
			mockFSM.On("IncrementBlockingQueries").Return(uint64(1))
			mockFSM.On("DecrementBlockingQueries").Return(uint64(1))
		})
		calls := 0
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			calls++
			if calls == 1 {
				meta.On("GetIndex").Return(uint64(3))

				ch := make(chan struct{})
				close(ch)
				ws.Add(ch)
				return ErrNotFound
			}
			meta.On("GetIndex").Return(uint64(5))
			return ErrNotFound
		}

		err := Query(fsm, opts, meta, fn)
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("blocking query for item that existed and is removed", func(t *testing.T) {
		opts := getOpts(t, func(options *MockRequestOptions) {
			options.On("GetMinQueryIndex").Return(uint64(3))
			// this query taks 1.002 sceonds locally so setting the timeout to 2 seconds
			options.On("GetMaxQueryTime").Return(2*time.Second, nil)
		})

		meta := getMeta(t, func(mockMeta *MockResponseMeta) {
			mockMeta.On("GetIndex").Return(uint64(3))
		})

		fsm := getFSM(t, func(mockFSM *MockFSMServer) {
			mockFSM.On("RPCQueryTimeout", mock.Anything).Return(1 * time.Second)
			mockFSM.On("IncrementBlockingQueries").Return(uint64(1))
			mockFSM.On("DecrementBlockingQueries").Return(uint64(1))
		})
		calls := 0
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			calls++
			if calls == 1 {

				ch := make(chan struct{})
				close(ch)
				ws.Add(ch)
				return nil
			}
			meta = getMeta(t, func(mockMeta *MockResponseMeta) {
				meta.On("GetIndex").Return(uint64(5))
			})
			return ErrNotFound
		}

		start := time.Now()
		require.NoError(t, Query(fsm, opts, meta, fn))
		queryDuration := time.Since(start)
		maxQueryDuration, err := opts.GetMaxQueryTime()
		require.NoError(t, err)
		require.True(t, queryDuration < maxQueryDuration, fmt.Sprintf("query timed out - queryDuration: %v, maxQueryDuration: %v", queryDuration, maxQueryDuration))
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("blocking query for non-existent item that is created", func(t *testing.T) {
		opts := getOpts(t, func(options *MockRequestOptions) {
			options.On("GetMinQueryIndex").Return(uint64(3))
			// this query taks 1.002 sceonds locally so setting the timeout to 2 seconds
			options.On("GetMaxQueryTime").Return(2*time.Second, nil)
		})

		meta := getMeta(t, func(mockMeta *MockResponseMeta) {
			mockMeta.On("GetIndex").Return(uint64(3))
		})

		fsm := getFSM(t, func(mockFSM *MockFSMServer) {
			mockFSM.On("RPCQueryTimeout", mock.Anything).Return(1 * time.Second)
			mockFSM.On("IncrementBlockingQueries").Return(uint64(1))
			mockFSM.On("DecrementBlockingQueries").Return(uint64(1))
		})
		calls := 0
		fn := func(ws memdb.WatchSet, _ *state.Store) error {
			calls++
			if calls == 1 {
				ch := make(chan struct{})
				close(ch)
				ws.Add(ch)
				return ErrNotFound
			}
			meta = getMeta(t, func(mockMeta *MockResponseMeta) {
				meta.On("GetIndex").Return(uint64(5))
			})
			return nil
		}

		start := time.Now()
		require.NoError(t, Query(fsm, opts, meta, fn))
		queryDuration := time.Since(start)
		maxQueryDuration, err := opts.GetMaxQueryTime()
		require.NoError(t, err)
		require.True(t, queryDuration < maxQueryDuration, fmt.Sprintf("query timed out - queryDuration: %v, maxQueryDuration: %v", queryDuration, maxQueryDuration))
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})
}
