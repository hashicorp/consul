// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package bindingrule

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

const synopsis = "Manage Consul's ACL binding rules"
const help = `
Usage: consul acl binding-rule <subcommand> [options] [args]

  This command has subcommands for managing Consul's ACL binding rules. Here
  are some simple examples, and more detailed examples are available in the
  subcommands or the documentation.

  Create a new binding rule:

    $ consul acl binding-rule create \
          -method=minikube \
          -bind-type=service \
          -bind-name='k8s-${serviceaccount.name}' \
          -selector='serviceaccount.namespace==default and serviceaccount.name==web'

  List all binding rules:

    $ consul acl binding-rule list

  Update a binding rule:

    $ consul acl binding-rule update -id=43cb72df-9c6f-4315-ac8a-01a9d98155ef \
          -bind-name='k8s-${serviceaccount.name}'

  Read a binding rule:

    $ consul acl binding-rule read -id fdabbcb5-9de5-4b1a-961f-77214ae88cba

  Delete a binding rule:

    $ consul acl binding-rule delete -id b6b856da-5193-4e78-845a-7d61ca8371ba

  For more examples, ask for subcommand help or view the documentation.
`
