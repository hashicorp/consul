package lib

import (
	"testing"
	"time"
)

func TestRandomStagger(t *testing.T) {
	intv := time.Minute
	for i := 0; i < 10; i++ {
		stagger := RandomStagger(intv)
		if stagger < 0 || stagger >= intv {
			t.Fatalf("Bad: %v", stagger)
		}
	}
}

func TestRateScaledInterval(t *testing.T) {
	min := 1 * time.Second
	rate := 200.0
	if v := RateScaledInterval(rate, min, 0); v != min {
		t.Fatalf("Bad: %v", v)
	}
	if v := RateScaledInterval(rate, min, 100); v != min {
		t.Fatalf("Bad: %v", v)
	}
	if v := RateScaledInterval(rate, min, 200); v != 1*time.Second {
		t.Fatalf("Bad: %v", v)
	}
	if v := RateScaledInterval(rate, min, 1000); v != 5*time.Second {
		t.Fatalf("Bad: %v", v)
	}
	if v := RateScaledInterval(rate, min, 5000); v != 25*time.Second {
		t.Fatalf("Bad: %v", v)
	}
	if v := RateScaledInterval(rate, min, 10000); v != 50*time.Second {
		t.Fatalf("Bad: %v", v)
	}
}
