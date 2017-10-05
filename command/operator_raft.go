package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorRaftCommand struct {
	BaseCommand
}

func (c *OperatorRaftCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *OperatorRaftCommand) Help() string {
	helpText := `
Usage: consul operator raft <subcommand> [options]

The Raft operator command is used to interact with Consul's Raft subsystem. The
command can be used to verify Raft peers or in rare cases to recover quorum by
removing invalid peers.

`

	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftCommand) Synopsis() string {
	return "Provides cluster-level tools for Consul operators"
}
