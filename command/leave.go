package command

import (
	"fmt"
)

// LeaveCommand is a Command implementation that instructs
// the Consul agent to gracefully leave the cluster
type LeaveCommand struct {
	BaseCommand
}

func (c *LeaveCommand) Help() string {
	c.InitFlagSet()
	return c.HelpCommand(`
Usage: consul leave [options]

  Causes the agent to gracefully leave the Consul cluster and shutdown.

`)
}

func (c *LeaveCommand) Run(args []string) int {
	c.InitFlagSet()
	if err := c.FlagSet.Parse(args); err != nil {
		return 1
	}
	nonFlagArgs := c.FlagSet.Args()
	if len(nonFlagArgs) > 0 {
		c.UI.Error(fmt.Sprintf("Error found unexpected args: %v", nonFlagArgs))
		c.UI.Output(c.Help())
		return 1
	}

	client, err := c.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if err := client.Agent().Leave(); err != nil {
		c.UI.Error(fmt.Sprintf("Error leaving: %s", err))
		return 1
	}

	c.UI.Output("Graceful leave complete")
	return 0
}

func (c *LeaveCommand) Synopsis() string {
	return "Gracefully leaves the Consul cluster and shuts down"
}
