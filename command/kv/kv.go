// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package kv

import (
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New() *cmd {
	return &cmd{}
}

type cmd struct{}

func (c *cmd) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(help, nil)
}

const synopsis = "Interact with the key-value store"
const help = `
Usage: consul kv <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's key-value
  store. Here are some simple examples, and more detailed examples are
  available in the subcommands or the documentation.

  Create or update the key named "redis/config/connections" with the value "5":

      $ consul kv put redis/config/connections 5

  Read this value back:

      $ consul kv get redis/config/connections

  Or get detailed key information:

      $ consul kv get -detailed redis/config/connections

  Finally, delete the key:

      $ consul kv delete redis/config/connections

  For more examples, ask for subcommand help or view the documentation.
`
