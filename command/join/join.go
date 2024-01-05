// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package join

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/flags"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string
	wan   bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.wan, "wan", false,
		"Joins a server to another server in the WAN pool.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	addrs := c.flags.Args()
	if len(addrs) == 0 {
		c.UI.Error("At least one address to join must be specified.")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	client, err := c.http.APIClient()
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

	c.UI.Output(fmt.Sprintf("Successfully joined cluster by contacting %d nodes.", joins))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Tell Consul agent to join cluster"
const help = `
Usage: consul join [options] address ...

  Tells a running Consul agent (with "consul agent") to join the cluster
  by specifying at least one existing member.
`
