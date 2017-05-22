package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSnapshotCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &SnapshotCommand{}
}

func TestSnapshotCommand_noTabs(t *testing.T) {
	t.Parallel()
	assertNoTabs(t, new(SnapshotCommand))
}
