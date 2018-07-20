package loop

import "testing"

func TestLoop(t *testing.T) {
	l := New(".")
	l.inc()
	if l.seen() != 1 {
		t.Errorf("Failed to inc loop, expected %d, got %d", 1, l.seen())
	}
}
