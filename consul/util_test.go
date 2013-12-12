package consul

import (
	"testing"
)

func TestStrContains(t *testing.T) {
	l := []string{"a", "b", "c"}
	if !strContains(l, "b") {
		t.Fatalf("should contain")
	}
	if strContains(l, "d") {
		t.Fatalf("should not contain")
	}
}
