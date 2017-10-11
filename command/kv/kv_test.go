package kv

import (
	"strings"
	"testing"
)

func TestKVCommand_noTabs(t *testing.T) {
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("usage has tabs")
	}
}
