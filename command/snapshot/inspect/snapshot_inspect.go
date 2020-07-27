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

	//flags
	enhance bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	//TODO(schristoff): better flag info would be good
	c.flags.BoolVar(&c.enhance, "verbose", false,
		"Adds more info")
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	var file string
	//TODO (schristoff): How do other flags handle this? Find other flags that take in
	//a file without a file flag and then have other flags.
	args = c.flags.Args()
	numArgs := len(args)

	//Check to make sure we have at least one argument
	if numArgs != 1 || numArgs != 2 {
		c.UI.Error(fmt.Sprint("Expected at least one argument, got %d", numArgs))
		return 1
	}

	//Set the first argument to the file name
	file = args[0]
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

	if c.enhance {
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
