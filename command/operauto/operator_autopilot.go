package operauto

import (
	"github.com/mitchellh/cli"
)

func New() *cmd {
	return &cmd{}
}

type cmd struct{}

func (c *cmd) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *cmd) Synopsis() string {
	return "Provides tools for modifying Autopilot configuration"
}

func (c *cmd) Help() string {
	s := `Usage: consul operator autopilot <subcommand> [options]

The Autopilot operator command is used to interact with Consul's Autopilot
subsystem. The command can be used to view or modify the current configuration.`

	return s
}
