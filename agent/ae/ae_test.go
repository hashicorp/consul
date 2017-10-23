package ae

import (
	"testing"
	"time"
)

func TestAE_scale(t *testing.T) {
	t.Parallel()
	intv := time.Minute
	if v := aeScale(intv, 100); v != intv {
		t.Fatalf("Bad: %v", v)
	}
	if v := aeScale(intv, 200); v != 2*intv {
		t.Fatalf("Bad: %v", v)
	}
	if v := aeScale(intv, 1000); v != 4*intv {
		t.Fatalf("Bad: %v", v)
	}
	if v := aeScale(intv, 10000); v != 8*intv {
		t.Fatalf("Bad: %v", v)
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
