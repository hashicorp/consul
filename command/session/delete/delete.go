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
	help string
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

	var id string
	switch c.flags.NArg() {
	case 0:
		c.UI.Error("Must specify a session UUID.")
		return 1
	case 1:
		id = c.flags.Arg(0)
	default:
		c.UI.Error("Extra arguments after the session UUID.")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if _, err := client.Session().Destroy(id, nil); err != nil {
		c.UI.Error(fmt.Sprintf("Error destroying session %q: %s", id, err))
		return 1
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Delete a session"
	help = `
Usage: consul session delete [options] SESSIONID

    Delete a session by providing the ID.

    Example:

        $ consul session delete b2caae8a-e80e-15f4-17aa-2be947c7968e
`
)
