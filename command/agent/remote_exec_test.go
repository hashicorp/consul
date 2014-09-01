package agent

import (
	"testing"
	"time"
)

func TestRexecWriter(t *testing.T) {
	writer := &rexecWriter{
		BufCh:    make(chan []byte, 16),
		BufSize:  16,
		BufIdle:  10 * time.Millisecond,
		CancelCh: make(chan struct{}),
	}

	// Write short, wait for idle
	start := time.Now()
	n, err := writer.Write([]byte("test"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 4 {
		t.Fatalf("bad: %v", n)
	}

	select {
	case b := <-writer.BufCh:
		if len(b) != 4 {
			t.Fatalf("Bad: %v", b)
		}
		if time.Now().Sub(start) < writer.BufIdle {
			t.Fatalf("too early")
		}
	case <-time.After(2 * writer.BufIdle):
		t.Fatalf("timeout")
	}

	// Write in succession to prevent the timeout
	writer.Write([]byte("test"))
	time.Sleep(writer.BufIdle / 2)
	writer.Write([]byte("test"))
	time.Sleep(writer.BufIdle / 2)
	start = time.Now()
	writer.Write([]byte("test"))

	select {
	case b := <-writer.BufCh:
		if len(b) != 12 {
			t.Fatalf("Bad: %v", b)
		}
		if time.Now().Sub(start) < writer.BufIdle {
			t.Fatalf("too early")
		}
	case <-time.After(2 * writer.BufIdle):
		t.Fatalf("timeout")
	}

	// Write large values, multiple flushes required
	writer.Write([]byte("01234567890123456789012345678901"))

	select {
	case b := <-writer.BufCh:
		if string(b) != "0123456789012345" {
			t.Fatalf("bad: %s", b)
		}
	default:
		t.Fatalf("should have buf")
	}
	select {
	case b := <-writer.BufCh:
		if string(b) != "6789012345678901" {
			t.Fatalf("bad: %s", b)
		}
	default:
		t.Fatalf("should have buf")
	}
}
