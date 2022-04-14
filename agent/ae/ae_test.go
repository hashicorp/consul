package ae

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestAE_scaleFactor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		nodes int
		scale int
	}{
		{100, 1},
		{200, 2},
		{1000, 4},
		{10000, 8},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d nodes", tt.nodes), func(t *testing.T) {
			if got, want := scaleFactor(tt.nodes), tt.scale; got != want {
				t.Fatalf("got scale factor %d want %d", got, want)
			}
		})
	}
}

func TestAE_Pause_nestedPauseResume(t *testing.T) {
	t.Parallel()
	l := NewStateSyncer(nil, 0, nil, nil)
	if l.Paused() != false {
		t.Fatal("syncer should be unPaused after init")
	}
	l.Pause()
	if l.Paused() != true {
		t.Fatal("syncer should be Paused after first call to Pause()")
	}
	l.Pause()
	if l.Paused() != true {
		t.Fatal("syncer should STILL be Paused after second call to Pause()")
	}
	gotR := l.Resume()
	if l.Paused() != true {
		t.Fatal("syncer should STILL be Paused after FIRST call to Resume()")
	}
	assert.False(t, gotR)
	gotR = l.Resume()
	if l.Paused() != false {
		t.Fatal("syncer should NOT be Paused after SECOND call to Resume()")
	}
	assert.True(t, gotR)

	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("unbalanced Resume() should panic")
		}
	}()
	l.Resume()
}

func TestAE_Pause_ResumeTriggersSyncChanges(t *testing.T) {
	l := NewStateSyncer(nil, 0, nil, nil)
	l.Pause()
	l.Resume()
	select {
	case <-l.SyncChanges.Notif():
		// expected
	case <-l.SyncFull.Notif():
		t.Fatal("resume triggered SyncFull instead of SyncChanges")
	default:
		t.Fatal("resume did not trigger SyncFull")
	}
}

func TestAE_staggerDependsOnClusterSize(t *testing.T) {
	libRandomStagger = func(d time.Duration) time.Duration { return d }
	defer func() { libRandomStagger = lib.RandomStagger }()

	l := testSyncer(t)
	if got, want := l.staggerFn(10*time.Millisecond), 10*time.Millisecond; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	l.ClusterSize = func() int { return 256 }
	if got, want := l.staggerFn(10*time.Millisecond), 20*time.Millisecond; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAE_Run_SyncFullBeforeChanges(t *testing.T) {
	shutdownCh := make(chan struct{})
	state := &mock{
		syncChanges: func() error {
			close(shutdownCh)
			return nil
		},
	}

	// indicate that we have partial changes before starting Run
	l := testSyncer(t)
	l.State = state
	l.ShutdownCh = shutdownCh
	l.SyncChanges.Trigger()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		l.Run()
	}()
	wg.Wait()

	if got, want := state.seq, []string{"full", "changes"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got call sequence %v want %v", got, want)
	}
}

func TestAE_Run_Quit(t *testing.T) {
	t.Run("Run panics without ClusterSize", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("Run should panic")
			}
		}()
		l := testSyncer(t)
		l.ClusterSize = nil
		l.Run()
	})
	t.Run("runFSM quits", func(t *testing.T) {
		// start timer which explodes if runFSM does not quit
		tm := time.AfterFunc(time.Second, func() { panic("timeout") })

		l := testSyncer(t)
		l.runFSM(fullSyncState, func(fsmState) fsmState { return doneState })
		// should just quit
		tm.Stop()
	})
}

func TestAE_FSM(t *testing.T) {
	t.Run("fullSyncState", func(t *testing.T) {
		t.Run("Paused -> retryFullSyncState", func(t *testing.T) {
			l := testSyncer(t)
			l.Pause()
			fs := l.nextFSMState(fullSyncState)
			if got, want := fs, retryFullSyncState; got != want {
				t.Fatalf("got state %v want %v", got, want)
			}
		})
		t.Run("SyncFull() error -> retryFullSyncState", func(t *testing.T) {
			l := testSyncer(t)
			l.State = &mock{syncFull: func() error { return errors.New("boom") }}
			fs := l.nextFSMState(fullSyncState)
			if got, want := fs, retryFullSyncState; got != want {
				t.Fatalf("got state %v want %v", got, want)
			}
		})
		t.Run("SyncFull() OK -> partialSyncState", func(t *testing.T) {
			l := testSyncer(t)
			l.State = &mock{}
			fs := l.nextFSMState(fullSyncState)
			if got, want := fs, partialSyncState; got != want {
				t.Fatalf("got state %v want %v", got, want)
			}
		})
	})

	t.Run("retryFullSyncState", func(t *testing.T) {
		// helper for testing state transitions from retrySyncFullState
		test := func(ev event, to fsmState) {
			l := testSyncer(t)
			l.retrySyncFullEvent = func() event { return ev }
			fs := l.nextFSMState(retryFullSyncState)
			if got, want := fs, to; got != want {
				t.Fatalf("got state %v want %v", got, want)
			}
		}
		t.Run("shutdownEvent -> doneState", func(t *testing.T) {
			test(shutdownEvent, doneState)
		})
		t.Run("syncFullNotifEvent -> fullSyncState", func(t *testing.T) {
			test(syncFullNotifEvent, fullSyncState)
		})
		t.Run("syncFullTimerEvent -> fullSyncState", func(t *testing.T) {
			test(syncFullTimerEvent, fullSyncState)
		})
		t.Run("invalid event -> panic ", func(t *testing.T) {
			defer func() {
				err := recover()
				if err == nil {
					t.Fatal("invalid event should panic")
				}
			}()
			test(event("invalid"), fsmState(""))
		})
	})

	t.Run("partialSyncState", func(t *testing.T) {
		// helper for testing state transitions from partialSyncState
		test := func(ev event, to fsmState) {
			l := testSyncer(t)
			l.syncChangesEvent = func() event { return ev }
			fs := l.nextFSMState(partialSyncState)
			if got, want := fs, to; got != want {
				t.Fatalf("got state %v want %v", got, want)
			}
		}
		t.Run("shutdownEvent -> doneState", func(t *testing.T) {
			test(shutdownEvent, doneState)
		})
		t.Run("syncFullNotifEvent -> fullSyncState", func(t *testing.T) {
			test(syncFullNotifEvent, fullSyncState)
		})
		t.Run("syncFullTimerEvent -> fullSyncState", func(t *testing.T) {
			test(syncFullTimerEvent, fullSyncState)
		})
		t.Run("syncChangesEvent+Paused -> partialSyncState", func(t *testing.T) {
			l := testSyncer(t)
			l.Pause()
			l.syncChangesEvent = func() event { return syncChangesNotifEvent }
			fs := l.nextFSMState(partialSyncState)
			if got, want := fs, partialSyncState; got != want {
				t.Fatalf("got state %v want %v", got, want)
			}
		})
		t.Run("syncChangesEvent+SyncChanges() error -> partialSyncState", func(t *testing.T) {
			l := testSyncer(t)
			l.State = &mock{syncChanges: func() error { return errors.New("boom") }}
			l.syncChangesEvent = func() event { return syncChangesNotifEvent }
			fs := l.nextFSMState(partialSyncState)
			if got, want := fs, partialSyncState; got != want {
				t.Fatalf("got state %v want %v", got, want)
			}
		})
		t.Run("syncChangesEvent+SyncChanges() OK -> partialSyncState", func(t *testing.T) {
			l := testSyncer(t)
			l.State = &mock{}
			l.syncChangesEvent = func() event { return syncChangesNotifEvent }
			fs := l.nextFSMState(partialSyncState)
			if got, want := fs, partialSyncState; got != want {
				t.Fatalf("got state %v want %v", got, want)
			}
		})
		t.Run("invalid event -> panic ", func(t *testing.T) {
			defer func() {
				err := recover()
				if err == nil {
					t.Fatal("invalid event should panic")
				}
			}()
			test(event("invalid"), fsmState(""))
		})
	})
	t.Run("invalid state -> panic ", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("invalid state should panic")
			}
		}()
		l := testSyncer(t)
		l.nextFSMState(fsmState("invalid"))
	})
}

func TestAE_RetrySyncFullEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("trigger shutdownEvent", func(t *testing.T) {
		l := testSyncer(t)
		l.ShutdownCh = make(chan struct{})
		evch := make(chan event)
		go func() { evch <- l.retrySyncFullEvent() }()
		close(l.ShutdownCh)
		if got, want := <-evch, shutdownEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
	t.Run("trigger shutdownEvent during FullNotif", func(t *testing.T) {
		l := testSyncer(t)
		l.ShutdownCh = make(chan struct{})
		evch := make(chan event)
		go func() { evch <- l.retrySyncFullEvent() }()
		l.SyncFull.Trigger()
		time.Sleep(100 * time.Millisecond)
		close(l.ShutdownCh)
		if got, want := <-evch, shutdownEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
	t.Run("trigger syncFullNotifEvent", func(t *testing.T) {
		l := testSyncer(t)
		l.serverUpInterval = 10 * time.Millisecond
		evch := make(chan event)
		go func() { evch <- l.retrySyncFullEvent() }()
		l.SyncFull.Trigger()
		if got, want := <-evch, syncFullNotifEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
	t.Run("trigger syncFullTimerEvent", func(t *testing.T) {
		l := testSyncer(t)
		l.retryFailInterval = 10 * time.Millisecond
		evch := make(chan event)
		go func() { evch <- l.retrySyncFullEvent() }()
		if got, want := <-evch, syncFullTimerEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
}

func TestAE_SyncChangesEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("trigger shutdownEvent", func(t *testing.T) {
		l := testSyncer(t)
		l.ShutdownCh = make(chan struct{})
		evch := make(chan event)
		go func() { evch <- l.syncChangesEvent() }()
		close(l.ShutdownCh)
		if got, want := <-evch, shutdownEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
	t.Run("trigger shutdownEvent during FullNotif", func(t *testing.T) {
		l := testSyncer(t)
		l.ShutdownCh = make(chan struct{})
		evch := make(chan event)
		go func() { evch <- l.syncChangesEvent() }()
		l.SyncFull.Trigger()
		time.Sleep(100 * time.Millisecond)
		close(l.ShutdownCh)
		if got, want := <-evch, shutdownEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
	t.Run("trigger syncFullNotifEvent", func(t *testing.T) {
		l := testSyncer(t)
		l.serverUpInterval = 10 * time.Millisecond
		evch := make(chan event)
		go func() { evch <- l.syncChangesEvent() }()
		l.SyncFull.Trigger()
		if got, want := <-evch, syncFullNotifEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
	t.Run("trigger syncFullTimerEvent", func(t *testing.T) {
		l := testSyncer(t)
		l.Interval = 10 * time.Millisecond
		evch := make(chan event)
		go func() { evch <- l.syncChangesEvent() }()
		if got, want := <-evch, syncFullTimerEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
	t.Run("trigger syncChangesNotifEvent", func(t *testing.T) {
		l := testSyncer(t)
		evch := make(chan event)
		go func() { evch <- l.syncChangesEvent() }()
		l.SyncChanges.Trigger()
		if got, want := <-evch, syncChangesNotifEvent; got != want {
			t.Fatalf("got event %q want %q", got, want)
		}
	})
}

type mock struct {
	seq                   []string
	syncFull, syncChanges func() error
}

func (m *mock) SyncFull() error {
	m.seq = append(m.seq, "full")
	if m.syncFull != nil {
		return m.syncFull()
	}
	return nil
}

func (m *mock) SyncChanges() error {
	m.seq = append(m.seq, "changes")
	if m.syncChanges != nil {
		return m.syncChanges()
	}
	return nil
}

func testSyncer(t *testing.T) *StateSyncer {
	logger := hclog.New(&hclog.LoggerOptions{
		Output: testutil.NewLogBuffer(t),
	})

	l := NewStateSyncer(nil, time.Second, nil, logger)
	l.stagger = func(d time.Duration) time.Duration { return d }
	l.ClusterSize = func() int { return 1 }
	l.resetNextFullSyncCh()
	return l
}
