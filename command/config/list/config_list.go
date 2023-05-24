// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package list

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
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

	kind   string
	filter string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.kind, "kind", "", "The kind of configurations to list.")
	c.flags.StringVar(&c.filter, "filter", "", "Filter to use with the request.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.kind == "" {
		c.UI.Error("Must specify the -kind parameter")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	entries, _, err := client.ConfigEntries().List(c.kind, &api.QueryOptions{
		Filter: c.filter,
	})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing config entries for kind %q: %v", c.kind, err))
		return 1
	}

	for _, entry := range entries {
		c.UI.Info(entry.GetName())
	}
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "List centralized config entries of a given kind"
	help     = `
Usage: consul config list [options] -kind <config kind>

  Lists all of the config entries for a given kind. The -kind parameter
  is required.

  Example:

    $ consul config list -kind service-defaults

`
)
