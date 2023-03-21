// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package services

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

const synopsis = "Interact with services"
const help = `
Usage: consul services <subcommand> [options] [args]

  This command has subcommands for interacting with services. The subcommands
  default to working with services registered with the local agent. Please see
  the "consul catalog" command for interacting with the entire catalog.

  For more examples, ask for subcommand help or view the documentation.
`
