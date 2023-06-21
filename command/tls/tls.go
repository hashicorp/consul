// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tls

import (
	"os"

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

func FileDoesNotExist(file string) bool {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return true
	}
	return false
}

const synopsis = `Builtin helpers for creating CAs and certificates`
const help = `
Usage: consul tls <subcommand> <subcommand> [options]

  This command has subcommands for interacting with Consul TLS.

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Create a CA

    $ consul tls ca create

  Create a server certificate

    $ consul tls cert create -server

  Create a client certificate

    $ consul tls cert create -client

  For more examples, ask for subcommand help or view the documentation.
`
