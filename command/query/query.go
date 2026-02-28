// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package query

import (
	"flag"

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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(help, nil)
}

const synopsis = "Interact with the prepared queries"
const help = `
Usage: consul query <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's prepared queries.
  Here are some simple examples, and more detailed examples are
  available in the subcommands or the documentation.

  Write a prepared query:

      $ consul query write redis redis.hcl

  Read a prepared query:

      $ consul query read -id 182dc666-3f3f-d5ca-ec46-093dd9396ac7

  Execute a prepared query:

      $ consul query execute -id 182dc666-3f3f-d5ca-ec46-093dd9396ac7

  List all prepared queries:

      $ consul query list

  Delete a query:

      $ consul query delete -id 182dc666-3f3f-d5ca-ec46-093dd9396ac7

  For more examples, ask for subcommand help or view the documentation.
`
