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

const synopsis = "Manage Consul's ACL tokens"
const help = `
Usage: consul acl token <subcommand> [options] [args]

  This command has subcommands for managing Consul ACL tokens.
  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Create a new ACL token:

      $ consul acl token create \
                                 -description "This is an example token" \
                                 -policy-id 06acc965
  List all tokens:

      $ consul acl token list

  Update a token:

      $ consul acl token update -id 986193 -description "WonderToken"

  Read a token with an accessor ID:

    $ consul acl token read -id 986193

  Delete a token

    $ consul acl token delete -id 986193

  For more examples, ask for subcommand help or view the documentation.
`
