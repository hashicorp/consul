// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ca

import (
	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/flags"
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

const synopsis = "Interact with the Consul Connect Certificate Authority (CA)"
const help = `
Usage: consul connect ca <subcommand> [options] [args]

  This command has subcommands for interacting with Consul Connect's 
  Certificate Authority (CA).

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Get the configuration:

      $ consul connect ca get-config

  Update the configuration:

      $ consul connect ca set-config -config-file ca.json

  For more examples, ask for subcommand help or view the documentation.
`
