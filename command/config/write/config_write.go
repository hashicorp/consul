package write

import (
	"flag"
	"fmt"
	"io"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/hcl"
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

	args = c.flags.Args()
	if len(args) != 1 {
		c.UI.Error("Must provide exactly one positional argument to specify the config entry to write")
		return 1
	}

	data, err := helpers.LoadDataSourceNoRaw(args[0], c.testStdin)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to load data: %v", err))
		return 1
	}

	// parse the data
	var raw map[string]interface{}
	err = hcl.Decode(&raw, data)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to decode config entry input: %v", err))
		return 1
	}

	entry, err := api.DecodeConfigEntry(raw)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to decode config entry input: %v", err))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	_, err = client.ConfigEntries().ConfigEntrySet(entry, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing config entry %q / %q: %v", entry.GetKind(), entry.GetName(), err))
		return 1
	}

	// TODO (mkeeler) should we output anything when successful
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Create or update a centralized config entry"
const help = `
Usage: consul config write [options] <configuration>

Request a config entry to be created or updated. The configuration
argument is either a file path or '-' to indicate that the config
should be read from stdin. The data should be either in HCL or
JSON form.

Example (from file):

    $ consul config write web.service.hcl

Example (from stdin):

    $ consul config write -
`
