package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

type OperatorAutopilotGetCommand struct {
	BaseCommand
}

func (c *OperatorAutopilotGetCommand) Help() string {
	helpText := `
Usage: consul operator autopilot get-config [options]

Displays the current Autopilot configuration.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *OperatorAutopilotGetCommand) Synopsis() string {
	return "Display the current Autopilot configuration"
}

func (c *OperatorAutopilotGetCommand) Run(args []string) int {
	c.BaseCommand.NewFlagSet(c)

	if err := c.BaseCommand.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.BaseCommand.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch the current configuration.
	opts := &api.QueryOptions{
		AllowStale: c.BaseCommand.HTTPStale(),
	}
	config, err := client.Operator().AutopilotGetConfiguration(opts)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Autopilot configuration: %s", err))
		return 1
	}
	c.UI.Output(fmt.Sprintf("CleanupDeadServers = %v", config.CleanupDeadServers))
	c.UI.Output(fmt.Sprintf("LastContactThreshold = %v", config.LastContactThreshold.String()))
	c.UI.Output(fmt.Sprintf("MaxTrailingLogs = %v", config.MaxTrailingLogs))
	c.UI.Output(fmt.Sprintf("ServerStabilizationTime = %v", config.ServerStabilizationTime.String()))
	c.UI.Output(fmt.Sprintf("RedundancyZoneTag = %q", config.RedundancyZoneTag))
	c.UI.Output(fmt.Sprintf("DisableUpgradeMigration = %v", config.DisableUpgradeMigration))

	return 0
}
