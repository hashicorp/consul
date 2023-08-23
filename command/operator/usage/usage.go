package usage

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

const synopsis = "Provides cluster-level usage information"
const help = `
Usage: consul operator usage <subcommand> [options] [args]

  This command has subcommands for displaying usage information. The subcommands
  default to working with services registered with the local datacenter.

  For more examples, ask for subcommand help or view the documentation.
`
