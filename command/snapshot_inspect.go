package command

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/consul/snapshot"
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

	meta, err := snapshot.Verify(f)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error verifying snapshot: %s", err))
	}

	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 2, 6, ' ', 0)
	fmt.Fprintf(tw, "ID\t%s\n", meta.ID)
	fmt.Fprintf(tw, "Size\t%d\n", meta.Size)
	fmt.Fprintf(tw, "Index\t%d\n", meta.Index)
	fmt.Fprintf(tw, "Term\t%d\n", meta.Term)
	fmt.Fprintf(tw, "Version\t%d\n", meta.Version)
	if err = tw.Flush(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error rendering snapshot info: %s", err))
	}

	c.Ui.Info(b.String())

	return 0
}

func (c *SnapshotInspectCommand) Synopsis() string {
	return "Displays information about a Consul snapshot file"
}
