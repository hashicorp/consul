package command

import (
	"fmt"
	"strings"
)

// LeaveWANCommand is a Command implementation that instructs
// the Consul agent to leave its WAN gossip pool
type LeaveWANCommand struct {
	BaseCommand
}

func (c *LeaveWANCommand) Help() string {
	helpText := `
Usage: consul leave wan [options]

  Causes the agent to gracefully leave the Consul cluster and shutdown.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *LeaveWANCommand) Run(args []string) int {
	f := c.BaseCommand.NewFlagSet(c)
	if err := c.BaseCommand.Parse(args); err != nil {
		return 1
	}
	nonFlagArgs := f.Args()
	if len(nonFlagArgs) > 0 {
		c.UI.Error(fmt.Sprintf("Error found unexpected args: %v", nonFlagArgs))
		c.UI.Output(c.Help())
		return 1
	}

	client, err := c.BaseCommand.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if err := client.Agent().LeaveWAN(); err != nil {
		c.UI.Error(fmt.Sprintf("Error leaving: %s", err))
		return 1
	}

	c.UI.Output("Graceful WAN leave complete")
	return 0
}

func (c *LeaveWANCommand) Synopsis() string {
	return "Gracefully leaves the Consul cluster's WAN gossip pool"
}
