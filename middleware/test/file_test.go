package test

import "testing"

func TestTempFile(t *testing.T) {
	_, f, e := TempFile(".", "test")
	if e != nil {
		t.Fatalf("failed to create temp file: %s", e)
	}
	defer f()
}
