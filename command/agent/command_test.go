package agent

import (
	"github.com/mitchellh/cli"
	"testing"
)

func TestCommand_implements(t *testing.T) {
	var _ cli.Command = new(Command)
}
