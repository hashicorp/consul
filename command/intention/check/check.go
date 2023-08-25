// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package check

import (
	"flag"
	"fmt"
	"io"

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

	// testStdin is the input for testing.
	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 2
	}

	args = c.flags.Args()
	if len(args) != 2 {
		c.UI.Error(fmt.Sprintf("Error: command requires exactly two arguments: src and dst"))
		return 2
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 2
	}

	// Check the intention
	allowed, _, err := client.Connect().IntentionCheck(&api.IntentionCheck{
		Source:      args[0],
		Destination: args[1],
		SourceType:  api.IntentionSourceConsul,
	}, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error checking the connection: %s", err))
		return 2
	}

	if allowed {
		c.UI.Output("Allowed")
		return 0
	}

	c.UI.Output("Denied")
	return 1
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Check whether a connection between two services is allowed."
	help     = `
Usage: consul intention check [options] SRC DST

  Check whether a connection between SRC and DST would be allowed by
  Connect given the current Consul configuration.

      $ consul intention check web db

`
)
