package get

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/intention/finder"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
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

	// testStdin is the input for testing.
	testStdin io.Reader
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

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Get the intention ID to load
	f := &finder.Finder{Client: client}
	id, err := f.IDFromArgs(c.flags.Args())
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error: %s", err))
		return 1
	}

	// Read the intention
	ixn, _, err := client.Connect().IntentionGet(id, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading the intention: %s", err))
		return 1
	}

	// Format the tabular data
	data := []string{
		fmt.Sprintf("Source:|%s", ixn.SourceString()),
		fmt.Sprintf("Destination:|%s", ixn.DestinationString()),
		fmt.Sprintf("Action:|%s", ixn.Action),
		fmt.Sprintf("ID:|%s", ixn.ID),
	}
	if v := ixn.Description; v != "" {
		data = append(data, fmt.Sprintf("Description:|%s", v))
	}
	if len(ixn.Meta) > 0 {
		var keys []string
		for k := range ixn.Meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			data = append(data, fmt.Sprintf("Meta[%s]:|%s", k, ixn.Meta[k]))
		}
	}
	data = append(data,
		fmt.Sprintf("Created At:|%s", ixn.CreatedAt.Local().Format(time.RFC850)),
	)

	c.UI.Output(columnize.SimpleFormat(data))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Show information about an intention."
const help = `
Usage: consul intention get [options] SRC DST
Usage: consul intention get [options] ID

  Read and show the details about an intention. The intention can be looked
  up via an exact source/destination match or via the unique intention ID.

      $ consul intention get web db

`
