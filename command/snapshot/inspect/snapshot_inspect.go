package inspect

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/snapshot"
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
	help  string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
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

	// Open the file.
	f, err := os.Open(file)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	meta, err := snapshot.Verify(f)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error verifying snapshot: %s", err))
		return 1
	}

	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 2, 6, ' ', 0)
	fmt.Fprintf(tw, "ID\t%s\n", meta.ID)
	fmt.Fprintf(tw, "Size\t%d\n", meta.Size)
	fmt.Fprintf(tw, "Index\t%d\n", meta.Index)
	fmt.Fprintf(tw, "Term\t%d\n", meta.Term)
	fmt.Fprintf(tw, "Version\t%d\n", meta.Version)
	if err = tw.Flush(); err != nil {
		c.UI.Error(fmt.Sprintf("Error rendering snapshot info: %s", err))
		return 1
	}

	c.UI.Info(b.String())

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Displays information about a Consul snapshot file"
const help = `
Usage: consul snapshot inspect [options] FILE

  Displays information about a snapshot file on disk.

  To inspect the file "backup.snap":

    $ consul snapshot inspect backup.snap

  For a full list of options and examples, please see the Consul documentation.
`
