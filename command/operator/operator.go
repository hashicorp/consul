// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package operator

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

const synopsis = "Provides cluster-level tools for Consul operators"
const help = `
Usage: consul operator <subcommand> [options]

  Provides cluster-level tools for Consul operators, such as interacting with
  the Raft subsystem. NOTE: Use this command with extreme caution, as improper
  use could lead to a Consul outage and even loss of data.

  If ACLs are enabled then a token with operator privileges may be required in
  order to use this command. Requests are forwarded internally to the leader
  if required, so this can be run from any Consul node in a cluster.

  Run consul operator <subcommand> with no arguments for help on that
  subcommand.
`
