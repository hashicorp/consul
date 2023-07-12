package raft

import (
	"strings"
	"testing"
)

func TestOperatorRaftCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
