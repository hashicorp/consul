package pipebootstrap

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestConnectEnvoyPipeBootstrapCommand_noTabs(t *testing.T) {
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
