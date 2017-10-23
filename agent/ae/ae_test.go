package ae

import (
	"fmt"
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

func TestAE_nestedPauseResume(t *testing.T) {
	t.Parallel()
	l := new(StateSyncer)
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
