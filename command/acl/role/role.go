// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package role

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

const synopsis = "Manage Consul's ACL roles"
const help = `
Usage: consul acl role <subcommand> [options] [args]

  This command has subcommands for managing Consul's ACL roles.
  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Create a new ACL role:

      $ consul acl role create -name "new-role" \
                               -description "This is an example role" \
                               -policy-id 06acc965
  List all roles:

      $ consul acl role list

  Update a role:

      $ consul acl role update -name "other-role" -datacenter "dc1"

  Read a role:

    $ consul acl role read -id 0479e93e-091c-4475-9b06-79a004765c24

  Delete a role

    $ consul acl role delete -name "my-role"

  For more examples, ask for subcommand help or view the documentation.
`
