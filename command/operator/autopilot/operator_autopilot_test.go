package autopilot

import (
	"strings"
	"testing"
)

func TestOperatorAutopilotCommand_noTabs(t *testing.T) {
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("usage has tabs")
	}
}
