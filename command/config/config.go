package config

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

const synopsis = "Interact with Consul's Centralized Configurations"
const help = `
Usage: consul config <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's Centralized
  Configuration system. Here are some simple examples, and more detailed
  examples are available in the subcommands or the documentation.

  Write a config:

    $ consul config write web.serviceconf.hcl

  Read a config:

    $ consul config read -kind service-defaults -name web

  List all configs for a type:

    $ consul config list -kind service-defaults

  Delete a config:

    $ consul config delete -kind service-defaults -name web

  For more examples, ask for subcommand help or view the documentation.
`
