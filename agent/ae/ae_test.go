package ae

import (
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
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
	l.Resume()
	if l.Paused() != true {
		t.Fatal("syncer should STILL be Paused after FIRST call to Resume()")
	}
	l.Resume()
	if l.Paused() != false {
		t.Fatal("syncer should NOT be Paused after SECOND call to Resume()")
	}

	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("unbalanced Resume() should cause a panic()")
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

func TestAE_Pause_ifNotPausedRun(t *testing.T) {
	l := NewStateSyncer(nil, 0, nil, nil)

	errCalled := errors.New("f called")
	f := func() error { return errCalled }

	l.Pause()
	err := l.ifNotPausedRun(f)
	if got, want := err, errPaused; !reflect.DeepEqual(got, want) {
		t.Fatalf("got error %q want %q", got, want)
	}
	l.Resume()

	err = l.ifNotPausedRun(f)
	if got, want := err, errCalled; got != want {
		t.Fatalf("got error %q want %q", got, want)
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
	l := testSyncer(state, shutdownCh)
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

func testSyncer(state State, shutdownCh chan struct{}) *StateSyncer {
	logger := log.New(os.Stderr, "", 0)
	l := NewStateSyncer(state, 0, shutdownCh, logger)
	l.stagger = func(d time.Duration) time.Duration { return d }
	l.ClusterSize = func() int { return 1 }
	return l
}
