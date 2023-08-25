// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package forceleave

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
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

	// flags
	prune bool
	wan   bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.prune, "prune", false,
		"Remove agent completely from list of members")
	c.flags.BoolVar(&c.wan, "wan", false,
		"Exclusively leave the agent from the WAN serf pool.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	nodes := c.flags.Args()
	if len(nodes) != 1 {
		c.UI.Error("A single node name must be specified to force leave.")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	err = client.Agent().ForceLeaveOpts(nodes[0], api.ForceLeaveOpts{
		Prune: c.prune,
		WAN:   c.wan,
	})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error force leaving: %s", err))
		return 1
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Forces a member of the cluster to enter the \"left\" state"
const help = `
Usage: consul force-leave [options] name

  Forces a member of a Consul cluster to enter the "left" state. Note
  that if the member is still actually alive, it will eventually rejoin
  the cluster. This command is most useful for cleaning out "failed" nodes
  that are never coming back. If you do not force leave a failed node,
  Consul will attempt to reconnect to those failed nodes for some period of
  time before eventually reaping them.

  -prune    Remove agent completely from list of members
  -wan      Exclusively leave the agent from the WAN serf pool.
`
