package consul

import (
	"testing"
	"time"
)

func TestTombstoneGC_invalid(t *testing.T) {
	_, err := NewTombstoneGC(0, 0)
	if err == nil {
		t.Fatalf("should fail")
	}

	_, err = NewTombstoneGC(time.Second, 0)
	if err == nil {
		t.Fatalf("should fail")
	}

	_, err = NewTombstoneGC(0, time.Second)
	if err == nil {
		t.Fatalf("should fail")
	}
}

func TestTombstoneGC(t *testing.T) {
	ttl := 20 * time.Millisecond
	gran := 5 * time.Millisecond
	gc, err := NewTombstoneGC(ttl, gran)
	if err != nil {
		t.Fatalf("should fail")
	}

	start := time.Now()
	gc.Hint(100)

	time.Sleep(2 * gran)
	start2 := time.Now()
	gc.Hint(120)
	gc.Hint(125)

	select {
	case index := <-gc.ExpireCh():
		end := time.Now()
		if end.Sub(start) < ttl {
			t.Fatalf("expired early")
		}
		if index != 100 {
			t.Fatalf("bad index: %d", index)
		}

	case <-time.After(ttl * 2):
		t.Fatalf("should get expiration")
	}

	select {
	case index := <-gc.ExpireCh():
		end := time.Now()
		if end.Sub(start2) < ttl {
			t.Fatalf("expired early")
		}
		if index != 125 {
			t.Fatalf("bad index: %d", index)
		}

	case <-time.After(ttl * 2):
		t.Fatalf("should get expiration")
	}
}
