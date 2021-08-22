package delete

import (
	"flag"
	"fmt"
	"io"

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

	// testStdin is the input for testing.
	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
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

	switch args := c.flags.Args(); len(args) {
	case 1:
		// old-style
		id := args[0]
		//nolint:staticcheck
		_, err = client.Connect().IntentionDelete(id, nil)

	case 2:
		// new-style
		source, destination := args[0], args[1]
		_, err = client.Connect().IntentionDeleteExact(source, destination, nil)

	default:
		c.UI.Error("command requires exactly 1 or 2 arguments")
		return 1
	}

	if err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting the intention: %s", err))
		return 1
	}

	c.UI.Output(fmt.Sprintf("Intention deleted."))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Delete an intention."
	help     = `
Usage: consul intention delete [options] SRC DST
Usage: consul intention delete [options] ID

  Delete an intention. This cannot be reversed. The intention can be looked
  up via an exact source/destination match or via the unique intention ID.

      $ consul intention delete web db

`
)
