//+build darwin

package freeport

import (
	"testing"
)

func TestGetEphemeralPortRange(t *testing.T) {
	min, max, err := getEphemeralPortRange()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if min <= 0 || max <= 0 || min > max {
		t.Fatalf("unexpected values: min=%d, max=%d", min, max)
	}
	t.Logf("min=%d, max=%d", min, max)
}
