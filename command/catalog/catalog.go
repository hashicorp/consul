package catalog

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

const synopsis = "Interact with the catalog"
const help = `
Usage: consul catalog <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's catalog. The
  catalog should not be confused with the agent, although the APIs and
  responses may be similar.

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  List all datacenters:

      $ consul catalog datacenters

  List all nodes:

      $ consul catalog nodes

  List all services:

      $ consul catalog services

  For more examples, ask for subcommand help or view the documentation.
`
