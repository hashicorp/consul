package ae

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
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
	l := NewStateSyner(nil, 0, nil, nil)
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
	l := NewStateSyner(nil, 0, nil, nil)
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
	l := NewStateSyner(nil, 0, nil, nil)

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
