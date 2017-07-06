package lstate

import (
	"testing"
	"time"
)

func TestAEScale(t *testing.T) {
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
