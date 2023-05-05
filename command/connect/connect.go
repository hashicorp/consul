// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

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

const synopsis = "Interact with Consul service mesh functionality"
const help = `
Usage: consul connect <subcommand> [options] [args]

  This command has subcommands for interacting with Consul service mesh.

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Run the production service mesh proxy

      $ consul connect envoy

  For more examples, ask for subcommand help or view the documentation.
`
