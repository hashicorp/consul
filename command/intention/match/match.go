// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package match

import (
	"flag"
	"fmt"
	"io"

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
	flagSource      bool
	flagDestination bool

	// testStdin is the input for testing.
	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.flagSource, "source", false,
		"Match intentions with the given source.")
	c.flags.BoolVar(&c.flagDestination, "destination", false,
		"Match intentions with the given destination.")

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
	if len(args) != 1 {
		c.UI.Error(fmt.Sprintf("Error: command requires exactly one argument: src or dst"))
		return 1
	}

	if c.flagSource && c.flagDestination {
		c.UI.Error(fmt.Sprintf("Error: only one of -source or -destination may be specified"))
		return 1
	}

	by := api.IntentionMatchDestination
	if c.flagSource {
		by = api.IntentionMatchSource
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Match the intention
	matches, _, err := client.Connect().IntentionMatch(&api.IntentionMatch{
		By:    by,
		Names: []string{args[0]},
	}, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error matching the connection: %s", err))
		return 1
	}

	for _, ixn := range matches[args[0]] {
		c.UI.Output(ixn.String())
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Show intentions that match a source or destination."
	help     = `
Usage: consul intention match [options] SRC|DST

  Show the list of intentions that would be enforced for a given source
  or destination. The intentions are listed in the order they would be
  evaluated.

      $ consul intention match db
      $ consul intention match -source web

`
)
