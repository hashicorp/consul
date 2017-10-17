package version

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestVersionCommand_noTabs(t *testing.T) {
	if strings.ContainsRune(New(cli.NewMockUi(), "").Help(), '\t') {
		t.Fatal("usage has tabs")
	}
}
