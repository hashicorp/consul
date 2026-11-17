// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package kv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

type blockingResult struct {
	pair    *api.KVPair
	pairs   api.KVPairs
	index   uint64
	elapsed time.Duration
	err     error
}

// blockingGet starts a blocking read of key that returns when the key changes
// after index or when wait elapses.
func blockingGet(kv *api.KV, key string, index uint64, wait time.Duration) <-chan blockingResult {
	out := make(chan blockingResult, 1)
	go func() {
		start := time.Now()
		pair, meta, err := kv.Get(key, &api.QueryOptions{WaitIndex: index, WaitTime: wait})
		res := blockingResult{pair: pair, elapsed: time.Since(start), err: err}
		if meta != nil {
			res.index = meta.LastIndex
		}
		out <- res
	}()
	return out
}

func blockingList(kv *api.KV, prefix string, index uint64, wait time.Duration) <-chan blockingResult {
	out := make(chan blockingResult, 1)
	go func() {
		start := time.Now()
		pairs, meta, err := kv.List(prefix, &api.QueryOptions{WaitIndex: index, WaitTime: wait})
		res := blockingResult{pairs: pairs, elapsed: time.Since(start), err: err}
		if meta != nil {
			res.index = meta.LastIndex
		}
		out <- res
	}()
	return out
}

// await returns the result of a blocking query, failing the test if it does
// not return in time.
func await(t *testing.T, ch <-chan blockingResult, within time.Duration) blockingResult {
	t.Helper()
	select {
	case res := <-ch:
		require.NoError(t, res.err)
		return res
	case <-time.After(within):
		t.Fatalf("blocking query did not return within %s", within)
		return blockingResult{}
	}
}

// TestKV_BlockingQueries verifies the long-poll contract that watches, config
// templating and the agent cache are built on: a read with an index waits until
// the data changes or the wait time passes.
func TestKV_BlockingQueries(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	kv := e.client.KV()

	put(t, kv, "watch/key", "v1")
	start := mustGet(t, kv, "watch/key")

	t.Run("an unchanged key returns when the wait time passes", func(t *testing.T) {
		res := await(t, blockingGet(kv, "watch/key", start.ModifyIndex, 2*time.Second), 30*time.Second)

		require.GreaterOrEqual(t, res.elapsed, 1500*time.Millisecond, "the query must block while nothing changes")
		require.Equal(t, start.ModifyIndex, res.index, "the index is unchanged")
		require.Equal(t, "v1", string(res.pair.Value))
	})

	t.Run("a write to the key wakes the query with the new value", func(t *testing.T) {
		ch := blockingGet(kv, "watch/key", start.ModifyIndex, 60*time.Second)
		time.Sleep(time.Second)
		put(t, kv, "watch/key", "v2")

		res := await(t, ch, 30*time.Second)
		require.Less(t, res.elapsed, 20*time.Second, "the query returns on the change, not at the wait time")
		require.Greater(t, res.index, start.ModifyIndex)
		require.Equal(t, "v2", string(res.pair.Value))
	})

	t.Run("a write to an unrelated key does not wake it", func(t *testing.T) {
		current := mustGet(t, kv, "watch/key")
		ch := blockingGet(kv, "watch/key", current.ModifyIndex, 4*time.Second)
		time.Sleep(time.Second)
		put(t, kv, "watch/unrelated", "x")

		res := await(t, ch, 30*time.Second)
		require.GreaterOrEqual(t, res.elapsed, 3500*time.Millisecond, "it only returns at the wait time")
		require.Equal(t, current.ModifyIndex, res.index)
	})

	t.Run("a recursive list wakes when a key under the prefix is added", func(t *testing.T) {
		put(t, kv, "prefix/a", "1")
		before, meta, err := kv.List("prefix/", nil)
		require.NoError(t, err)
		require.Len(t, before, 1)

		ch := blockingList(kv, "prefix/", meta.LastIndex, 60*time.Second)
		time.Sleep(time.Second)
		put(t, kv, "prefix/b", "2")

		res := await(t, ch, 30*time.Second)
		require.Equal(t, []string{"prefix/a", "prefix/b"}, keysOf(res.pairs))
		require.Greater(t, res.index, meta.LastIndex)
	})

	t.Run("a missing key can be waited on until it is created", func(t *testing.T) {
		pair, meta, err := kv.Get("watch/future", nil)
		require.NoError(t, err)
		require.Nil(t, pair)

		ch := blockingGet(kv, "watch/future", meta.LastIndex, 60*time.Second)
		time.Sleep(time.Second)
		put(t, kv, "watch/future", "now")

		res := await(t, ch, 30*time.Second)
		require.NotNil(t, res.pair)
		require.Equal(t, "now", string(res.pair.Value))
	})

	t.Run("deleting a key wakes the query with no value", func(t *testing.T) {
		put(t, kv, "watch/doomed", "x")
		current := mustGet(t, kv, "watch/doomed")

		ch := blockingGet(kv, "watch/doomed", current.ModifyIndex, 60*time.Second)
		time.Sleep(time.Second)
		_, err := kv.Delete("watch/doomed", nil)
		require.NoError(t, err)

		res := await(t, ch, 30*time.Second)
		require.Nil(t, res.pair)
		require.Greater(t, res.index, current.ModifyIndex)
	})
}
