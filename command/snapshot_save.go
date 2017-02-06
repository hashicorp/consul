package command

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/snapshot"
	"github.com/mitchellh/cli"
)

// SnapshotSaveCommand is a Command implementation that is used to save the
// state of the Consul servers for disaster recovery.
type SnapshotSaveCommand struct {
	Ui cli.Ui
}

func (c *SnapshotSaveCommand) Help() string {
	helpText := `
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

` + apiOptsText

	return strings.TrimSpace(helpText)
}

func (c *SnapshotSaveCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("get", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	datacenter := cmdFlags.String("datacenter", "", "")
	token := cmdFlags.String("token", "", "")
	stale := cmdFlags.Bool("stale", false, "")
	httpAddr := HTTPAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	var file string

	args = cmdFlags.Args()
	switch len(args) {
	case 0:
		c.Ui.Error("Missing FILE argument")
		return 1
	case 1:
		file = args[0]
	default:
		c.Ui.Error(fmt.Sprintf("Too many arguments (expected 1, got %d)", len(args)))
		return 1
	}

	// Create and test the HTTP client
	conf := api.DefaultConfig()
	conf.Datacenter = *datacenter
	conf.Address = *httpAddr
	conf.Token = *token
	client, err := api.NewClient(conf)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Take the snapshot.
	snap, qm, err := client.Snapshot().Save(&api.QueryOptions{
		AllowStale: *stale,
	})
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error saving snapshot: %s", err))
		return 1
	}
	defer snap.Close()

	// Save the file.
	f, err := os.Create(file)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating snapshot file: %s", err))
		return 1
	}
	if _, err := io.Copy(f, snap); err != nil {
		f.Close()
		c.Ui.Error(fmt.Sprintf("Error writing snapshot file: %s", err))
		return 1
	}
	if err := f.Close(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error closing snapshot file after writing: %s", err))
		return 1
	}

	// Read it back to verify.
	f, err = os.Open(file)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening snapshot file for verify: %s", err))
		return 1
	}
	if _, err := snapshot.Verify(f); err != nil {
		f.Close()
		c.Ui.Error(fmt.Sprintf("Error verifying snapshot file: %s", err))
		return 1
	}
	if err := f.Close(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error closing snapshot file after verify: %s", err))
		return 1
	}

	c.Ui.Info(fmt.Sprintf("Saved and verified snapshot to index %d", qm.LastIndex))
	return 0
}

func (c *SnapshotSaveCommand) Synopsis() string {
	return "Saves snapshot of Consul server state"
}
