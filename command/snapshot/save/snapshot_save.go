// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package save

import (
	"flag"
	"fmt"
	"os"

	"github.com/mitchellh/cli"
	"github.com/rboyer/safeio"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/snapshot"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	var file string

	args = c.flags.Args()
	switch len(args) {
	case 0:
		c.UI.Error("Missing FILE argument")
		return 1
	case 1:
		file = args[0]
	default:
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 1, got %d)", len(args)))
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Take the snapshot.
	snap, qm, err := client.Snapshot().Save(&api.QueryOptions{
		AllowStale: c.http.Stale(),
	})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error saving snapshot: %s", err))
		return 1
	}
	defer snap.Close()

	// Save the file first.
	unverifiedFile := file + ".unverified"
	if _, err := safeio.WriteToFile(snap, unverifiedFile, 0600); err != nil {
		c.UI.Error(fmt.Sprintf("Error writing unverified snapshot file: %s", err))
		return 1
	}
	defer os.Remove(unverifiedFile)

	// Read it back to verify.
	f, err := os.Open(unverifiedFile)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error opening snapshot file for verify: %s", err))
		return 1
	}
	if _, err := snapshot.Verify(f); err != nil {
		f.Close()
		c.UI.Error(fmt.Sprintf("Error verifying snapshot file: %s", err))
		return 1
	}
	if err := f.Close(); err != nil {
		c.UI.Error(fmt.Sprintf("Error closing snapshot file after verify: %s", err))
		return 1
	}

	if err := safeio.Rename(unverifiedFile, file); err != nil {
		c.UI.Error(fmt.Sprintf("Error renaming %q to %q: %v", unverifiedFile, file, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Saved and verified snapshot to index %d", qm.LastIndex))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Saves snapshot of Consul server state"
const help = `
Usage: consul snapshot save [options] FILE

  Retrieves an atomic, point-in-time snapshot of the state of the Consul servers
  which includes key/value entries, service catalog, prepared queries, sessions,
  and ACLs.

  If ACLs are enabled, a management token must be supplied in order to perform
  snapshot operations.

  To create a snapshot from the leader server and save it to "backup.snap":

    $ consul snapshot save backup.snap

  To create a potentially stale snapshot from any available server (useful if no
  leader is available):

    $ consul snapshot save -stale backup.snap

  For a full list of options and examples, please see the Consul documentation.
`
