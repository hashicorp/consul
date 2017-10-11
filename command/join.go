package command

import (
	"fmt"
)

// JoinCommand is a Command implementation that tells a running Consul
// agent to join another.
type JoinCommand struct {
	BaseCommand

	// flags
	wan bool
}

func (c *JoinCommand) initFlags() {
	c.InitFlagSet()
	c.FlagSet.BoolVar(&c.wan, "wan", false,
		"Joins a server to another server in the WAN pool.")
}

func (c *JoinCommand) Run(args []string) int {
	c.initFlags()
	if err := c.FlagSet.Parse(args); err != nil {
		return 1
	}

	addrs := c.FlagSet.Args()
	if len(addrs) == 0 {
		c.UI.Error("At least one address to join must be specified.")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	client, err := c.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	joins := 0
	for _, addr := range addrs {
		err := client.Agent().Join(addr, c.wan)
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

func (c *JoinCommand) Help() string {
	c.initFlags()
	return c.HelpCommand(`
Usage: consul join [options] address ...

  Tells a running Consul agent (with "consul agent") to join the cluster
  by specifying at least one existing member.

`)
}

func (c *JoinCommand) Synopsis() string {
	return "Tell Consul agent to join cluster"
}
