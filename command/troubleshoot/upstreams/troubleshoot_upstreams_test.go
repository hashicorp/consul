package upstreams

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestTroubleshootUpstreamsCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
