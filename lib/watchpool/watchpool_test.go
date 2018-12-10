package watchpool

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil/retry"

	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
)

func TestWatchPoolWatch_Timeout(t *testing.T) {
	ws := memdb.NewWatchSet()

	wp := &WatchPool{}

	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	ch3 := make(chan struct{})

	ws.Add(ch1)
	ws.Add(ch2)
	ws.Add(ch3)

	// Watch with context deadline should timeout. I test this first as it's worth
	// relying on in other tests.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	require := require.New(t)
	err := wp.Watch(ctx, ws)
	require.Error(err)
	require.Equal(context.DeadlineExceeded, err)
}

func TestWatchPoolWatch_SingleFires(t *testing.T) {
	ws := memdb.NewWatchSet()

	wp := &WatchPool{}

	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	ch3 := make(chan struct{})

	ws.Add(ch1)
	ws.Add(ch2)
	ws.Add(ch3)

	// Close a chan before watch
	close(ch2)

	// Watch with context deadline should timeout. I test this first as it's worth
	// relying on in other tests.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	require := require.New(t)
	err := wp.Watch(ctx, ws)
	require.NoError(err)
}

func TestWatchPoolWatch_SingleFiresAfterDelay(t *testing.T) {
	ws := memdb.NewWatchSet()

	wp := &WatchPool{}

	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	ch3 := make(chan struct{})

	ws.Add(ch1)
	ws.Add(ch2)
	ws.Add(ch3)

	// Close a chan in a bit
	time.AfterFunc(20*time.Millisecond, func() {
		close(ch3)
	})

	// Deadline limits test length
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	require := require.New(t)
	start := time.Now()
	err := wp.Watch(ctx, ws)
	require.NoError(err)
	require.True(time.Since(start) > 20*time.Millisecond)
}

func testMakeChans(n int) []chan struct{} {
	chans := make([]chan struct{}, n)
	for i := range chans {
		chans[i] = make(chan struct{})
	}
	return chans
}

func testRunWatcher(ctx context.Context, wp *WatchPool, chans []chan struct{}, errCh chan error) {
	ws := memdb.NewWatchSet()
	for _, ch := range chans {
		ws.Add(ch)
	}
	errCh <- wp.Watch(ctx, ws)
}

func testRunNWatchers(wp *WatchPool, chans []chan struct{}, n int, to time.Duration) chan error {
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), to)
			defer cancel()
			testRunWatcher(ctx, wp, chans, errCh)
		}()
	}
	return errCh
}

func TestWatchPoolWatch_MultipleAreDeduped(t *testing.T) {
	wp := &WatchPool{}

	// Create 100 chans to watch (just in case lots makes a difference)
	chans := testMakeChans(100)

	// Run the initial watcher
	leaderCtx, leaderCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer leaderCancel()
	leaderErrCh := make(chan error)
	go testRunWatcher(leaderCtx, wp, chans, leaderErrCh)

	// Wait for it to be running. Reaching into private state is unpleasant but
	// it beats relying on timing.
	retry.Run(t, func(r *retry.R) {
		wp.Lock()
		defer wp.Unlock()
		require.Len(r, wp.watchSets, 1)
	})

	// Now run a few other watchers on the same set of chans (different watch set
	// objects).
	n := 3
	errCh := testRunNWatchers(wp, chans, n, 100*time.Millisecond)

	// Wait a bit
	time.Sleep(20 * time.Millisecond)

	req := require.New(t)

	// Sanity check only one ws in state
	wp.Lock()
	req.Len(wp.watchSets, 1)
	wp.Unlock()

	// Cancel the leader
	leaderCancel()

	// Should get Cancelled error from leader
	select {
	case err := <-leaderErrCh:
		req.Error(err)
		req.Equal(context.Canceled, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatal("timedout")
	}

	// Other watchers should still be running though
	select {
	case err := <-errCh:
		t.Fatalf("watchers should not have fired yet got %s (len %d)", err, len(errCh))
	default:
	}

	// And pool should still have state
	wp.Lock()
	req.Len(wp.watchSets, 1)
	wp.Unlock()

	// Now close a chan being watched
	close(chans[0])

	// The watchers should all return no error since one of them should have taken
	// over leadership
	for i := 0; i < n; i++ {
		select {
		case err := <-errCh:
			req.NoError(err)
		case <-time.After(50 * time.Millisecond):
			t.Fatal("timedout")
		}
	}

	// And pool should have had it's state cleaned up
	wp.Lock()
	req.Len(wp.watchSets, 0)
	wp.Unlock()
}

func TestWatchPoolWatch_MultipleAllTimeout(t *testing.T) {
	wp := &WatchPool{}

	// Create 100 chans to watch (just in case lots makes a difference)
	chans := testMakeChans(100)

	// Now run a few other watchers on the same set of chans (different watch set
	// objects).
	n := 3
	start := time.Now()
	errCh := testRunNWatchers(wp, chans, n, 20*time.Millisecond)

	// Wait for the timeout
	for i := 0; i < n; i++ {
		select {
		case err := <-errCh:
			require.Error(t, err)
			require.Equal(t, context.DeadlineExceeded, err)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("took too long to all fail")
		}
	}
	// Sanity check they all actually blocked until timeout
	require.True(t, time.Since(start) > 20*time.Millisecond)

	// Sanity check pool was left in a clean state
	wp.Lock()
	require.Len(t, wp.watchSets, 0)
	wp.Unlock()
}

func TestWatchPoolWatch_KeyHash(t *testing.T) {

	ch1 := make(chan struct{})
	ch2 := make(chan struct{})

	tests := []struct {
		name     string
		a, b     memdb.WatchSet
		wantSame bool
	}{
		{
			name:     "sets with same chan match",
			a:        memdb.WatchSet{ch1: struct{}{}},
			b:        memdb.WatchSet{ch1: struct{}{}},
			wantSame: true,
		},
		{
			name:     "sets with diff chans don't match",
			a:        memdb.WatchSet{ch1: struct{}{}},
			b:        memdb.WatchSet{ch2: struct{}{}},
			wantSame: false,
		},
		{
			name:     "sets with same chans in diff order match",
			a:        memdb.WatchSet{ch1: struct{}{}, ch2: struct{}{}},
			b:        memdb.WatchSet{ch2: struct{}{}, ch1: struct{}{}},
			wantSame: true,
		},
		{
			name:     "sets with subset of chans don't match",
			a:        memdb.WatchSet{ch1: struct{}{}, ch2: struct{}{}},
			b:        memdb.WatchSet{ch2: struct{}{}},
			wantSame: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wp := &WatchPool{}
			hA, err := wp.wsKey(tc.a)
			require.NoError(t, err)
			hB, err := wp.wsKey(tc.b)
			require.NoError(t, err)

			if tc.wantSame {
				require.Equal(t, hA, hB)
			} else {
				require.NotEqual(t, hA, hB)
			}
		})
	}
}
