// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package save

import (
	"flag"
	"fmt"
	"golang.org/x/exp/slices"
	"os"
	"path/filepath"
	"strings"

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
	UI                 cli.Ui
	flags              *flag.FlagSet
	http               *flags.HTTPFlags
	help               string
	appendFileNameFlag flags.StringValue
}

func (c *cmd) getAppendFileNameFlag() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Var(&c.appendFileNameFlag, "append-filename", "Append filename flag supports the following "+
		"comma-separated arguments. 1. version, 2. dc. It appends these values to the filename provided in the command")
	return fs
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.getAppendFileNameFlag())
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

	appendFileNameFlags := strings.Split(c.appendFileNameFlag.String(), ",")

	if len(appendFileNameFlags) != 0 && len(c.appendFileNameFlag.String()) > 0 {
		fileExt := filepath.Ext(file)
		fileNameWithoutExt := strings.TrimSuffix(file, fileExt)

		if slices.Contains(appendFileNameFlags, "version") {
			operatorHealthResponse, err := client.Operator().AutopilotServerHealth(nil)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error fetching version of Consul agent Leader: %s", err))
				return 1
			}
			var version string
			for _, server := range operatorHealthResponse.Servers {
				if server.Leader {
					version = server.Version
					break
				}
			}
			fileNameWithoutExt = fileNameWithoutExt + "-" + version
		}

		if slices.Contains(appendFileNameFlags, "dc") {
			agentSelfResponse, err := client.Agent().Self()
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error connecting to Consul agent and fetching datacenter/version: %s", err))
				return 1
			}

			if config, ok := agentSelfResponse["Config"]; ok {
				if datacenter, ok := config["Datacenter"]; ok {
					fileNameWithoutExt = fileNameWithoutExt + "-" + datacenter.(string)
				}
			}
		}

		//adding extension back
		file = fileNameWithoutExt + fileExt
	}

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
