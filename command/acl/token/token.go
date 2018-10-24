package token

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

const synopsis = "Manage Consul's ACL Tokens"
const help = `
Usage: consul acl token <subcommand> [options] [args]

  This command has subcommands for managing Consul's ACL Policies.
  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  TODO - more docs

  For more examples, ask for subcommand help or view the documentation.
`
