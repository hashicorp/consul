package restore

import (
	"flag"
	"fmt"
	"os"

	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
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

	// Open the file.
	f, err := os.Open(file)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	// Restore the snapshot.
	err = client.Snapshot().Restore(nil, f)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error restoring snapshot: %s", err))
		return 1
	}

	c.UI.Info("Restored snapshot")
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Restores snapshot of Consul server state"
const help = `
Usage: consul snapshot restore [options] FILE

  Restores an atomic, point-in-time snapshot of the state of the Consul servers
  which includes key/value entries, service catalog, prepared queries, sessions,
  and ACLs.

  Restores involve a potentially dangerous low-level Raft operation that is not
  designed to handle server failures during a restore. This command is primarily
  intended to be used when recovering from a disaster, restoring into a fresh
  cluster of Consul servers.

  If ACLs are enabled, a management token must be supplied in order to perform
  snapshot operations.

  To restore a snapshot from the file "backup.snap":

    $ consul snapshot restore backup.snap

  For a full list of options and examples, please see the Consul documentation.
`
