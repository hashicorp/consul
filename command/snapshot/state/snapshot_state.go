package state

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/hashicorp/consul/raftutil"
	"os"
	"strings"

	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

// Run (snapshot state):
// Dump state obtained from StateAsMap into a JSON format and print to stdout
func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	// Check that we either got no filename or exactly one.
	if len(c.flags.Args()) != 1 {
		c.UI.Error("This command takes one argument: <file>")
		return 1
	}

	path := c.flags.Args()[0]
	f, err := os.Open(path)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	state, meta, err := raftutil.RestoreFromArchive(f)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to read archive file: %s", err))
		return 1
	}

	sm := raftutil.StateAsMap(state)
	sm["SnapshotMeta"] = []interface{}{meta}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(sm); err != nil {
		c.UI.Error(fmt.Sprintf("Failed to encode output: %v", err))
		return 1
	}

	return 0
}

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	help  string

	// flags
	format string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.format, "format", PrettyFormat, fmt.Sprintf("Output format {%s}", strings.Join(GetSupportedFormats(), "|")))
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Displays information about a Consul snapshot file"
const help = `
Usage: consul snapshot state [options] <file>

  Displays a JSON representation of state in the snapshot.

  To inspect the file "backup.snap":

    $ consul snapshot state backup.snap

`
