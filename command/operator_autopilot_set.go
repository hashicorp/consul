package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/command/base"
)

type OperatorAutopilotSetCommand struct {
	base.Command
}

func (c *OperatorAutopilotSetCommand) Help() string {
	helpText := `
Usage: consul operator autopilot set-config [options]

Modifies the current Autopilot configuration.

` + c.Command.Help()

	return strings.TrimSpace(helpText)
}

func (c *OperatorAutopilotSetCommand) Synopsis() string {
	return "Modify the current Autopilot configuration"
}

func (c *OperatorAutopilotSetCommand) Run(args []string) int {
	var deadServerCleanup base.BoolValue

	f := c.Command.NewFlagSet(c)

	f.Var(&deadServerCleanup, "dead-server-cleanup",
		"Controls whether Consul will automatically remove dead servers "+
			"when new ones are successfully added. Must be one of `true|false`.")

	if err := c.Command.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.Command.HTTPClient()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch the current configuration.
	operator := client.Operator()
	conf, err := operator.AutopilotGetConfiguration(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Autopilot configuration: %s", err))
		return 1
	}

	// Update the config values.
	deadServerCleanup.Merge(&conf.DeadServerCleanup)

	// Check-and-set the new configuration.
	result, err := operator.AutopilotCASConfiguration(conf, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error setting Autopilot configuration: %s", err))
		return 1
	}
	if result {
		c.Ui.Output("Configuration updated")
	} else {
		c.Ui.Output("Configuration could not be atomically updated")
	}

	return 0
}
