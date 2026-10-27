// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package featuregate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-memdb"
)

func TestStore(t *testing.T) {
	var store Store
	require.False(t, store.Enabled(APIGatewayUpstreamRouting))

	require.True(t, store.Publish(Snapshot{
		StatusIndex: 10,
		PolicyIndex: 8,
		Features: map[string]bool{
			APIGatewayUpstreamRouting.String(): true,
		},
	}))
	require.True(t, store.Enabled(APIGatewayUpstreamRouting))

	// Equal and older generations cannot invalidate committed state.
	require.False(t, store.Publish(Snapshot{StatusIndex: 10}))
	require.False(t, store.Publish(Snapshot{StatusIndex: 9}))
	require.True(t, store.Enabled(APIGatewayUpstreamRouting))

	current := store.Current()
	current.Features[APIGatewayUpstreamRouting.String()] = false
	require.True(t, store.Enabled(APIGatewayUpstreamRouting))

	require.True(t, store.Publish(Snapshot{
		StatusIndex: 11,
		PolicyIndex: 8,
		Features: map[string]bool{
			APIGatewayUpstreamRouting.String(): false,
		},
	}))
	require.False(t, store.Enabled(APIGatewayUpstreamRouting))
}

func TestStore_Reset(t *testing.T) {
	var store Store

	store.Reset(0)
	require.False(t, store.Enabled(APIGatewayUpstreamRouting), "reset on uninitialised store must remain fail-closed")

	require.True(t, store.Publish(Snapshot{
		StatusIndex: 5,
		Features:    map[string]bool{APIGatewayUpstreamRouting.String(): true},
	}))
	require.True(t, store.Enabled(APIGatewayUpstreamRouting))

	watch := store.Watch()

	store.Reset(0)

	require.False(t, store.Enabled(APIGatewayUpstreamRouting), "reset must clear the snapshot and fail-close")

	select {
	case <-watch:
	default:
		t.Fatal("Reset must notify watchers")
	}

	require.True(t, store.Publish(Snapshot{
		StatusIndex: 3,
		Features:    map[string]bool{APIGatewayUpstreamRouting.String(): true},
	}), "Publish after Reset must succeed even with a lower StatusIndex")
	require.True(t, store.Enabled(APIGatewayUpstreamRouting))
}

func TestStore_Watch(t *testing.T) {
	store := &Store{}
	watch := store.Watch()

	require.True(t, store.Publish(Snapshot{StatusIndex: 1}))
	require.Eventually(t, func() bool {
		select {
		case <-watch:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	next := store.Watch()
	require.False(t, store.Publish(Snapshot{StatusIndex: 1}))
	select {
	case <-next:
		t.Fatal("stale publication notified watchers")
	default:
	}
}

func TestStoreObserveForQuery(t *testing.T) {
	feature := Feature{name: "observe-test"}

	t.Run("nil store is disabled and leaves index and watch set untouched", func(t *testing.T) {
		var s *Store
		ws := memdb.NewWatchSet()
		enabled, index := s.ObserveForQuery(ws, feature, 42)
		require.False(t, enabled)
		require.Equal(t, uint64(42), index)
		require.Empty(t, ws)
	})

	t.Run("nil watch set is tolerated", func(t *testing.T) {
		var s Store
		enabled, index := s.ObserveForQuery(nil, feature, 7)
		require.False(t, enabled)
		require.Equal(t, uint64(7), index)
	})

	t.Run("uninitialized store is disabled but still registers a watch", func(t *testing.T) {
		var s Store
		ws := memdb.NewWatchSet()
		enabled, index := s.ObserveForQuery(ws, feature, 10)
		require.False(t, enabled)
		require.Equal(t, uint64(10), index)
		require.Len(t, ws, 1)
	})

	t.Run("published snapshot raises index to StatusIndex", func(t *testing.T) {
		var s Store
		s.Publish(Snapshot{StatusIndex: 50, Features: map[string]bool{"observe-test": true}})

		enabled, index := s.ObserveForQuery(nil, feature, 10)
		require.True(t, enabled)
		require.Equal(t, uint64(50), index)

		enabled, index = s.ObserveForQuery(nil, feature, 99)
		require.True(t, enabled)
		require.Equal(t, uint64(99), index)
	})

	t.Run("watch fires on the next publish", func(t *testing.T) {
		var s Store
		ws := memdb.NewWatchSet()
		s.ObserveForQuery(ws, feature, 1)

		go s.Publish(Snapshot{StatusIndex: 5, Features: map[string]bool{"observe-test": true}})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, ws.WatchCtx(ctx), "watch set must fire when the gate publishes")
	})
}

func TestStoreObserveForQuery_PerFeature(t *testing.T) {
	observed := Feature{name: "observed"}
	fired := func(ws memdb.WatchSet) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		return ws.WatchCtx(ctx) == nil
	}

	var s Store
	s.Publish(Snapshot{StatusIndex: 10, Features: map[string]bool{"observed": true, "other": false}})

	t.Run("unrelated feature change neither wakes nor advances the index", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		_, before := s.ObserveForQuery(ws, observed, 1)
		require.Equal(t, uint64(10), before)

		s.Publish(Snapshot{StatusIndex: 20, Features: map[string]bool{"observed": true, "other": true}})
		require.False(t, fired(ws), "an unrelated feature change must not wake the query")

		enabled, after := s.ObserveForQuery(nil, observed, 1)
		require.True(t, enabled)
		require.Equal(t, before, after, "an unrelated feature change must not advance the index")
	})

	t.Run("republishing an unchanged value does not wake", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		s.ObserveForQuery(ws, observed, 1)
		s.Publish(Snapshot{StatusIndex: 30, Features: map[string]bool{"observed": true, "other": true}})
		require.False(t, fired(ws))
	})

	t.Run("a change of the observed feature wakes and advances the index", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		s.ObserveForQuery(ws, observed, 1)
		s.Publish(Snapshot{StatusIndex: 40, Features: map[string]bool{"observed": false, "other": true}})
		require.True(t, fired(ws))

		enabled, index := s.ObserveForQuery(nil, observed, 1)
		require.False(t, enabled)
		require.Equal(t, uint64(40), index)
	})

	t.Run("a feature removed from the snapshot while enabled counts as a change", func(t *testing.T) {
		s.Publish(Snapshot{StatusIndex: 50, Features: map[string]bool{"observed": true}})
		ws := memdb.NewWatchSet()
		s.ObserveForQuery(ws, observed, 1)
		s.Publish(Snapshot{StatusIndex: 60, Features: map[string]bool{}})
		require.True(t, fired(ws))
		_, index := s.ObserveForQuery(nil, observed, 1)
		require.Equal(t, uint64(60), index)
	})

	t.Run("reset wakes watchers of an enabled feature and advances past indexes already returned", func(t *testing.T) {
		s.Publish(Snapshot{StatusIndex: 70, Features: map[string]bool{"observed": true}})
		ws := memdb.NewWatchSet()
		_, returned := s.ObserveForQuery(ws, observed, 75)
		require.Equal(t, uint64(75), returned)

		// After an FSM replacement the Raft applied index is newer than any
		// index returned to a query, so the disabled result must be delivered.
		s.Reset(80)
		require.True(t, fired(ws))
		enabled, index := s.ObserveForQuery(nil, observed, 75)
		require.False(t, enabled)
		require.Greater(t, index, returned, "a blocking query must see a newer index for the reset")
		require.Equal(t, uint64(80), index)
	})

	t.Run("a publish after reset with an older status index still advances the index", func(t *testing.T) {
		// Reset at 80, then a restored snapshot re-enables the feature with an
		// older StatusIndex (70): the transition must still be newer than 80.
		s.Publish(Snapshot{StatusIndex: 100, Features: map[string]bool{"observed": true}})
		s.Reset(110)
		ws := memdb.NewWatchSet()
		enabled, afterReset := s.ObserveForQuery(ws, observed, 1)
		require.False(t, enabled)
		require.Equal(t, uint64(110), afterReset)

		require.True(t, s.Publish(Snapshot{StatusIndex: 70, Features: map[string]bool{"observed": true}}))
		require.True(t, fired(ws))
		enabled, index := s.ObserveForQuery(nil, observed, afterReset)
		require.True(t, enabled)
		require.Greater(t, index, afterReset, "every effective transition must get a strictly newer index")
	})

	t.Run("reset leaves disabled features untouched", func(t *testing.T) {
		s.Publish(Snapshot{StatusIndex: 90, Features: map[string]bool{"observed": false}})
		ws := memdb.NewWatchSet()
		_, before := s.ObserveForQuery(ws, observed, 1)
		s.Reset(95)
		require.False(t, fired(ws))
		_, after := s.ObserveForQuery(nil, observed, 1)
		require.Equal(t, before, after)
	})
}

func TestStoreReplace(t *testing.T) {
	observed := Feature{name: "observed"}
	fired := func(ws memdb.WatchSet) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		return ws.WatchCtx(ctx) == nil
	}

	var s Store
	s.Publish(Snapshot{StatusIndex: 50, Features: map[string]bool{"observed": true}})
	_, before := s.ObserveForQuery(nil, observed, 0)

	t.Run("an older generation with an unchanged value installs without waking", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		s.ObserveForQuery(ws, observed, 0)
		s.Replace(Snapshot{StatusIndex: 20, Features: map[string]bool{"observed": true}}, 60)
		require.Equal(t, uint64(20), s.Current().StatusIndex, "Replace must install an older generation")
		require.False(t, fired(ws))
		_, index := s.ObserveForQuery(nil, observed, 0)
		require.Equal(t, before, index)
	})

	t.Run("an older generation with a changed value wakes at a newer index", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		s.ObserveForQuery(ws, observed, 0)
		s.Replace(Snapshot{StatusIndex: 10, Features: map[string]bool{"observed": false}}, 70)
		require.True(t, fired(ws))
		enabled, index := s.ObserveForQuery(nil, observed, 0)
		require.False(t, enabled)
		require.Equal(t, uint64(70), index)
	})

	t.Run("later commits publish normally after a replace", func(t *testing.T) {
		require.True(t, s.Publish(Snapshot{StatusIndex: 80, Features: map[string]bool{"observed": true}}))
		enabled, index := s.ObserveForQuery(nil, observed, 0)
		require.True(t, enabled)
		require.Equal(t, uint64(80), index)
	})
}

func TestStoreObserveNamesForQuery(t *testing.T) {
	fired := func(ws memdb.WatchSet) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		return ws.WatchCtx(ctx) == nil
	}

	t.Run("nil store returns the index unchanged", func(t *testing.T) {
		var s *Store
		ws := memdb.NewWatchSet()
		require.Equal(t, uint64(7), s.ObserveNamesForQuery(ws, []string{"a"}, 7))
		require.Empty(t, ws)
	})

	t.Run("returns the newest change index of the observed names only", func(t *testing.T) {
		var s Store
		s.Publish(Snapshot{StatusIndex: 10, Features: map[string]bool{"a": true, "b": false, "c": false}})
		s.Publish(Snapshot{StatusIndex: 20, Features: map[string]bool{"a": true, "b": true, "c": false}})
		s.Publish(Snapshot{StatusIndex: 30, Features: map[string]bool{"a": true, "b": true, "c": true}})

		require.Equal(t, uint64(10), s.ObserveNamesForQuery(nil, []string{"a"}, 0))
		require.Equal(t, uint64(20), s.ObserveNamesForQuery(nil, []string{"a", "b"}, 0))
		require.Equal(t, uint64(30), s.ObserveNamesForQuery(nil, []string{"a", "b", "c"}, 0))
		require.Equal(t, uint64(99), s.ObserveNamesForQuery(nil, []string{"a", "b", "c"}, 99), "a newer table index is kept")
		require.Equal(t, uint64(5), s.ObserveNamesForQuery(nil, nil, 5))
	})

	t.Run("wakes only for the observed names", func(t *testing.T) {
		var s Store
		s.Publish(Snapshot{StatusIndex: 10, Features: map[string]bool{"a": true, "b": true}})

		ws := memdb.NewWatchSet()
		s.ObserveNamesForQuery(ws, []string{"a"}, 0)
		s.Publish(Snapshot{StatusIndex: 20, Features: map[string]bool{"a": true, "b": false}})
		require.False(t, fired(ws), "an unobserved name must not wake the query")

		s.Publish(Snapshot{StatusIndex: 30, Features: map[string]bool{"a": false, "b": false}})
		require.True(t, fired(ws))
	})

	t.Run("a restore to an older generation is delivered at a newer index", func(t *testing.T) {
		var s Store
		s.Publish(Snapshot{StatusIndex: 1000, Features: map[string]bool{"a": true}})
		ws := memdb.NewWatchSet()
		returned := s.ObserveNamesForQuery(ws, []string{"a"}, 0)
		require.Equal(t, uint64(1000), returned)

		s.Replace(Snapshot{StatusIndex: 10, Features: map[string]bool{"a": false}}, 12)
		require.True(t, fired(ws))
		require.Greater(t, s.ObserveNamesForQuery(nil, []string{"a"}, 0), returned)
	})
}
