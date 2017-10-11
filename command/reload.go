package command

import (
	"fmt"
)

// ReloadCommand is a Command implementation that instructs
// the Consul agent to reload configurations
type ReloadCommand struct {
	BaseCommand
}

func (c *ReloadCommand) Help() string {
	c.InitFlagSet()
	return c.HelpCommand(`
Usage: consul reload

  Causes the agent to reload configurations. This can be used instead
  of sending the SIGHUP signal to the agent.

`)
}

func (c *ReloadCommand) Run(args []string) int {
	c.InitFlagSet()
	if err := c.FlagSet.Parse(args); err != nil {
		return 1
	}

	client, err := c.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if err := client.Agent().Reload(); err != nil {
		c.UI.Error(fmt.Sprintf("Error reloading: %s", err))
		return 1
	}

	c.UI.Output("Configuration reload triggered")
	return 0
}

func (c *ReloadCommand) Synopsis() string {
	return "Triggers the agent to reload configuration files"
}
