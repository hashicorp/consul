package operauto

import (
	"strings"
	"testing"
)

func TestOperatorAutopilotCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("usage has tabs")
	}
}
