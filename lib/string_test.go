package lib

import (
	"testing"
)

func TestStrContains(t *testing.T) {
	l := []string{"a", "b", "c"}
	if !StrContains(l, "b") {
		t.Fatalf("should contain")
	}
	if StrContains(l, "d") {
		t.Fatalf("should not contain")
	}
}

func TestToLowerList(t *testing.T) {
	l := []string{"ABC", "Abc", "abc"}
	for _, value := range ToLowerList(l) {
		if value != "abc" {
			t.Fatalf("failed lowercasing")
		}
	}
}
