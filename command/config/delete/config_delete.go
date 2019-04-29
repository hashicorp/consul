package delete

import (
	"flag"
	"fmt"

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

	kind string
	name string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.kind, "kind", "", "The kind of configuration to delete.")
	c.flags.StringVar(&c.name, "name", "", "The name of configuration to delete.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.kind == "" {
		c.UI.Error("Must specify the -kind parameter")
		return 1
	}

	if c.name == "" {
		c.UI.Error("Must specify the -name parameter")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	_, err = client.ConfigEntries().Delete(c.kind, c.name, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting config entry %q / %q: %v", c.kind, c.name, err))
		return 1
	}

	// TODO (mkeeler) should we output anything when successful
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(help, nil)
}

const synopsis = "Delete a centralized config entry"
const help = `
Usage: consul config delete [options] -kind <config kind> -name <config name>

  Deletes the configuration entry specified by the kind and name.

  Example:

    $ consul config delete -kind service-defaults -name web
`
