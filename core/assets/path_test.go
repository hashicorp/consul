package assets

import (
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	if actual := Path(); !strings.HasSuffix(actual, ".coredns") {
		t.Errorf("Expected path to be a .coredns folder, got: %v", actual)
	}
}
