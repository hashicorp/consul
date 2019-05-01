package acl

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

const synopsis = "Interact with Consul's ACLs"
const help = `
Usage: consul acl <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's ACLs.
  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Bootstrap ACLs:

      $ consul acl bootstrap

  List all ACL tokens:

      $ consul acl token list

  Create a new ACL policy:

      $ consul acl policy create -name "new-policy" \
                                 -description "This is an example policy" \
                                 -datacenter "dc1" \
                                 -datacenter "dc2" \
                                 -rules @rules.hcl

  Set the default agent token:

      $ consul acl set-agent-token default 0bc6bc46-f25e-4262-b2d9-ffbe1d96be6f

  For more examples, ask for subcommand help or view the documentation.
`
