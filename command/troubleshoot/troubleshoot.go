// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package troubleshoot

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

const synopsis = `CLI tools for troubleshooting Consul service mesh`
const help = `
Usage: consul troubleshoot <subcommand> [options]

  This command has subcommands for troubleshooting the service mesh.

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Troubleshoot Get Upstreams

    $ consul troubleshoot upstreams

  Troubleshoot Proxy

    $ consul troubleshoot proxy -upstream [options]

  For more examples, ask for subcommand help or view the documentation.
`
