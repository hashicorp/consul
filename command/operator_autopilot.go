package command

import (
	"github.com/mitchellh/cli"
)

type OperatorAutopilotCommand struct {
	BaseCommand
}

func (c *OperatorAutopilotCommand) Help() string {
	c.InitFlagSet()
	return c.HelpCommand(`
Usage: consul operator autopilot <subcommand> [options]

The Autopilot operator command is used to interact with Consul's Autopilot
subsystem. The command can be used to view or modify the current configuration.

`)
}

func (c *OperatorAutopilotCommand) Synopsis() string {
	return "Provides tools for modifying Autopilot configuration"
}

func (c *OperatorAutopilotCommand) Run(args []string) int {
	return cli.RunResultHelp
}
