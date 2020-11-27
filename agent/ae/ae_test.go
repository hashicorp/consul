package ae

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/lib"
)

func TestScaleFactor(t *testing.T) {
	tests := []struct {
		nodes int
		scale int
	}{
		{100, 1},
		{200, 2},
		{1000, 4},
		{10000, 8},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d nodes", tc.nodes), func(t *testing.T) {
			require.Equal(t, tc.scale, scaleFactor(tc.nodes))
		})
	}
}

func TestStateSyncer_Pause_nestedPauseResume(t *testing.T) {
	sync := NewStateSyncer(nil, 0, nil, nil)

	require.False(t, sync.isPaused(), "syncer should be unPaused after init")
	require.Nil(t, sync.WaitResume())

	sync.Pause()
	require.True(t, sync.isPaused(), "syncer should be Paused after first call to Pause()")
	isBlocked(t, sync.WaitResume())

	sync.Pause()
	require.True(t, sync.isPaused(), "syncer should STILL be Paused after second call to Pause()")
	isBlocked(t, sync.WaitResume())

	resumed := sync.Resume()
	require.False(t, resumed)
	require.True(t, sync.isPaused(), "syncer should STILL be Paused after FIRST call to Resume()")
	chWaitResume := sync.WaitResume()
	isBlocked(t, chWaitResume)

	resumed = sync.Resume()
	require.True(t, resumed)
	require.False(t, sync.isPaused(), "syncer should NOT be Paused after SECOND call to Resume()")
	require.Nil(t, sync.WaitResume())

	select {
	case <-chWaitResume:
	case <-time.After(20 * time.Millisecond):
		t.Fatal("expected WaitResume to unblock when sync is resumed")
	}

	require.Panics(t, func() { sync.Resume() }, "unbalanced Resume() should panic")
}

func isBlocked(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("expected channel to be blocked")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStateSyncer_Resume_TriggersSyncChanges(t *testing.T) {
	l := NewStateSyncer(nil, 0, nil, nil)
	l.Pause()
	l.Resume()
	select {
	case <-l.SyncChanges.wait():
		// expected
	case <-l.SyncFull.wait():
		t.Fatal("resume triggered SyncFull instead of SyncChanges")
	default:
		t.Fatal("resume did not trigger SyncFull")
	}
}

func TestDelayer_Jitter(t *testing.T) {
	libRandomStagger = func(d time.Duration) time.Duration { return d }
	defer func() { libRandomStagger = lib.RandomStagger }()

	t.Run("cluster size 1", func(t *testing.T) {

		delayer := NewClusterSizeDelayer(func() int { return 1 })
		actual := delayer.Jitter(10 * time.Millisecond)
		require.Equal(t, 10*time.Millisecond, actual)
	})

	t.Run("cluster size 256", func(t *testing.T) {
		delayer := NewClusterSizeDelayer(func() int { return 256 })
		actual := delayer.Jitter(10 * time.Millisecond)
		require.Equal(t, 20*time.Millisecond, actual)
	})
}

func TestStateSyncer_Run_SyncFullBeforeChanges(t *testing.T) {
	shutdownCh := make(chan struct{})
	state := &fakeState{
		syncChanges: func() error {
			close(shutdownCh)
			return nil
		},
	}

	logger := hclog.New(nil)

	l := NewStateSyncer(state, time.Millisecond, shutdownCh, logger)
	l.Delayer = constDelayer{}

	l.SyncChanges.Trigger()
	l.Run()

	require.Equal(t, []string{"full", "changes"}, state.calls)
}

func TestStateSyncer_Run_SyncFullTrigger(t *testing.T) {
	shutdownCh := make(chan struct{})
	var counter int
	state := &fakeState{
		syncFull: func() error {
			counter++
			if counter == 2 {
				close(shutdownCh)
			}
			return nil
		},
	}

	logger := hclog.New(nil)
	l := NewStateSyncer(state, time.Millisecond, shutdownCh, logger)
	l.Delayer = constDelayer{}
	l.serverUpInterval = time.Millisecond

	l.SyncFull.Trigger()
	l.Run()

	require.Equal(t, []string{"full", "full"}, state.calls)
}

func TestStateSyncer_Run_PauseSyncFull(t *testing.T) {
	shutdownCh := make(chan struct{})
	var counter int
	state := &fakeState{
		syncFull: func() error {
			counter++
			if counter == 2 {
				close(shutdownCh)
			}
			return nil
		},
	}

	logger := hclog.New(nil)
	l := NewStateSyncer(state, time.Millisecond, shutdownCh, logger)
	l.Delayer = constDelayer{}
	l.retryFailInterval = time.Nanosecond

	l.Pause()

	chDone := make(chan struct{})
	chStarted := make(chan struct{})
	go func() {
		close(chStarted)
		l.Run()
		close(chDone)
	}()

	<-chStarted
	isBlocked(t, chDone)
	l.Resume()
	<-chDone

	require.Equal(t, []string{"full", "changes", "full"}, state.calls)
}

func TestStateSyncer_Run_PauseSyncChanges(t *testing.T) {
	shutdownCh := make(chan struct{})
	var counter int
	chStarted := make(chan struct{})

	state := &fakeState{
		syncFull: func() error {
			if counter == 0 {
				close(chStarted)
			}
			return nil
		},
		syncChanges: func() error {
			counter++
			if counter == 2 {
				close(shutdownCh)
			}
			return nil
		},
	}

	logger := hclog.New(nil)
	l := NewStateSyncer(state, time.Second, shutdownCh, logger)
	l.Delayer = constDelayer{}
	l.retryFailInterval = time.Second

	chDone := make(chan struct{})
	go func() {
		l.Run()
		close(chDone)
	}()

	<-chStarted
	l.Pause()
	l.SyncChanges.Trigger()
	isBlocked(t, chDone)
	l.Resume()

	time.Sleep(50 * time.Millisecond)
	l.SyncChanges.Trigger()
	<-chDone

	require.Equal(t, []string{"full", "changes", "changes"}, state.calls)
}

type constDelayer struct{}

func (constDelayer) Jitter(d time.Duration) time.Duration {
	return d
}

type fakeState struct {
	calls       []string
	syncFull    func() error
	syncChanges func() error
}

func (m *fakeState) SyncFull() error {
	m.calls = append(m.calls, "full")
	if m.syncFull != nil {
		return m.syncFull()
	}
	return nil
}

func (m *fakeState) SyncChanges() error {
	m.calls = append(m.calls, "changes")
	if m.syncChanges != nil {
		return m.syncChanges()
	}
	return nil
}
