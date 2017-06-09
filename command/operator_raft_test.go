package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func testOperatorRaftCommand(t *testing.T) (*cli.MockUi, *OperatorRaftCommand) {
	ui := cli.NewMockUi()
	return ui, &OperatorRaftCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetHTTP,
		},
	}
}

func TestOperator_Raft_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorRaftCommand{}
}
