package freeport

import "testing"

func TestFreePort_GetFreePort(t *testing.T) {
	p := Get(t)
	if p <= 0 {
		t.Fatalf("bad port: %d", p)
	}
}
