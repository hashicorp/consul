// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

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

const synopsis = "Manage Consul's ACL policies"
const help = `
Usage: consul acl policy <subcommand> [options] [args]

  This command has subcommands for managing Consul's ACL policies.
  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Create a new ACL policy:

      $ consul acl policy create -name "new-policy" \
                                 -description "This is an example policy" \
                                 -datacenter "dc1" \
                                 -datacenter "dc2" \
                                 -rules @rules.hcl
  List all policies:

      $ consul acl policy list

  Update a policy:

      $ consul acl policy update -name "other-policy" -datacenter "dc1"

  Read a policy:

    $ consul acl policy read -id 0479e93e-091c-4475-9b06-79a004765c24

  Delete a policy

    $ consul acl policy delete -name "my-policy"

  For more examples, ask for subcommand help or view the documentation.
`
