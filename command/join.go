package command

import (
	"fmt"
	"strings"
)

// JoinCommand is a Command implementation that tells a running Consul
// agent to join another.
type JoinCommand struct {
	BaseCommand
}

func (c *JoinCommand) Help() string {
	helpText := `
Usage: consul join [options] address ...

  Tells a running Consul agent (with "consul agent") to join the cluster
  by specifying at least one existing member.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *JoinCommand) Run(args []string) int {
	var wan bool

	f := c.BaseCommand.NewFlagSet(c)
	f.BoolVar(&wan, "wan", false, "Joins a server to another server in the WAN pool.")
	if err := c.BaseCommand.Parse(args); err != nil {
		return 1
	}

	addrs := f.Args()
	if len(addrs) == 0 {
		c.UI.Error("At least one address to join must be specified.")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	client, err := c.BaseCommand.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	joins := 0
	for _, addr := range addrs {
		err := client.Agent().Join(addr, wan)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error joining address '%s': %s", addr, err))
		} else {
			joins++
		}
	}

	if joins == 0 {
		c.UI.Error("Failed to join any nodes.")
		return 1
	}

	c.UI.Output(fmt.Sprintf(
		"Successfully joined cluster by contacting %d nodes.", joins))
	return 0
}

func (c *JoinCommand) Synopsis() string {
	return "Tell Consul agent to join cluster"
}
