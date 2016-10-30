package command

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/consul/consul/snapshot"
	"github.com/mitchellh/cli"
)

// SnapshotInspectCommand is a Command implementation that is used to display
// metadata about a snapshot file
type SnapshotInspectCommand struct {
	Ui cli.Ui
}

func (c *SnapshotInspectCommand) Help() string {
	helpText := `
Usage: consul snapshot inspect [options] FILE

  Displays information about a snapshot file on disk.

  To inspect the file "backup.snap":

    $ consul snapshot inspect backup.snap

  For a full list of options and examples, please see the Consul documentation.
`

	return strings.TrimSpace(helpText)
}

func (c *SnapshotInspectCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("get", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
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

	// Open the file.
	f, err := os.Open(file)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	meta, err := snapshot.ReadMetadata(f)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing metadata: %s", err))
	}

	c.Ui.Output(fmt.Sprintf("id = %s", meta.ID))
	c.Ui.Output(fmt.Sprintf("size = %d", meta.Size))
	c.Ui.Output(fmt.Sprintf("index = %d", meta.Index))
	c.Ui.Output(fmt.Sprintf("term = %d", meta.Term))
	c.Ui.Output(fmt.Sprintf("snapshot_version = %d", meta.Version))
	c.Ui.Output(fmt.Sprintf("configuration_index = %d", meta.ConfigurationIndex))
	c.Ui.Output("\nservers:\n")
	for _, server := range meta.Configuration.Servers {
		c.Ui.Output(string(server.ID))
		c.Ui.Output(fmt.Sprintf("\taddress = %s", server.Address))
		c.Ui.Output(fmt.Sprintf("\tsuffrage = %s", server.Suffrage.String()))
	}

	return 0
}

func (c *SnapshotInspectCommand) Synopsis() string {
	return "Displays information about a Consul snapshot file"
}
