package command

import (
	"fmt"
	"strings"
)

// ForceLeaveCommand is a Command implementation that tells a running Consul
// to force a member to enter the "left" state.
type ForceLeaveCommand struct {
	BaseCommand
}

func (c *ForceLeaveCommand) Run(args []string) int {
	f := c.BaseCommand.NewFlagSet(c)
	if err := c.BaseCommand.Parse(args); err != nil {
		return 1
	}

	nodes := f.Args()
	if len(nodes) != 1 {
		c.UI.Error("A single node name must be specified to force leave.")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	client, err := c.BaseCommand.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	err = client.Agent().ForceLeave(nodes[0])
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error force leaving: %s", err))
		return 1
	}

	return 0
}

func (c *ForceLeaveCommand) Synopsis() string {
	return "Forces a member of the cluster to enter the \"left\" state"
}

func (c *ForceLeaveCommand) Help() string {
	helpText := `
Usage: consul force-leave [options] name

  Forces a member of a Consul cluster to enter the "left" state. Note
  that if the member is still actually alive, it will eventually rejoin
  the cluster. This command is most useful for cleaning out "failed" nodes
  that are never coming back. If you do not force leave a failed node,
  Consul will attempt to reconnect to those failed nodes for some period of
  time before eventually reaping them.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}
