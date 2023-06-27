// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dc

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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if l := len(c.flags.Args()); l > 0 {
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 0, got %d)", l))
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	dcs, err := client.Catalog().Datacenters()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing datacenters: %s", err))
		return 1
	}

	for _, dc := range dcs {
		c.UI.Info(dc)
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Lists all known datacenters"
const help = `
Usage: consul catalog datacenters [options]

  Retrieves the list of all known datacenters. This datacenters are sorted in
  ascending order based on the estimated median round trip time from the servers
  in this datacenter to the servers in the other datacenters.

  To retrieve the list of datacenters:

      $ consul catalog datacenters

  For a full list of options and examples, please see the Consul documentation.
`
