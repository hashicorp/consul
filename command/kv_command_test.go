package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestKVCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &KVCommand{}
}

func TestKVCommand_noTabs(t *testing.T) {
	t.Parallel()
	assertNoTabs(t, new(KVCommand))
}
