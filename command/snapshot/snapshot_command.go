// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package snapshot

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

const synopsis = "Saves, restores and inspects snapshots of Consul server state"
const help = `
Usage: consul snapshot <subcommand> [options] [args]

  This command has subcommands for saving, restoring, and inspecting the state
  of the Consul servers for disaster recovery. These are atomic, point-in-time
  snapshots which include key/value entries, service catalog, prepared queries,
  sessions, and ACLs.

  If ACLs are enabled, a management token must be supplied in order to perform
  snapshot operations.

  Create a snapshot:

      $ consul snapshot save backup.snap

  Restore a snapshot:

      $ consul snapshot restore backup.snap

  Inspect a snapshot:

      $ consul snapshot inspect backup.snap

  Run a daemon process that locally saves a snapshot every hour (available only in
  Consul Enterprise) :

      $ consul snapshot agent

  For more examples, ask for subcommand help or view the documentation.
`
