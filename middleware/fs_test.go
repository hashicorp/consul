package middleware

import (
	"os"
	"strings"
	"testing"
)

func TestfsPath(t *testing.T) {
	if actual := fsPath(); !strings.HasSuffix(actual, ".coredns") {
		t.Errorf("Expected path to be a .coredns folder, got: %v", actual)
	}

	os.Setenv("COREDNSPATH", "testpath")
	defer os.Setenv("COREDNSPATH", "")
	if actual, expected := fsPath(), "testpath"; actual != expected {
		t.Errorf("Expected path to be %v, got: %v", expected, actual)
	}
}
