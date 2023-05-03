package read

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/session"
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

	flagFormat string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.flagFormat, "format", session.PrettyFormat,
		fmt.Sprintf("Output format {%s}.", strings.Join(session.GetSupportedFormats(), "|")))

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

	s, _, err := client.Session().Info(id, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading session information: %s", err))
		return 1
	}
	if s == nil {
		c.UI.Error(fmt.Sprintf("No session %q found", id))
		return 1
	}

	formatter, err := session.NewFormatter(c.flagFormat)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to get formatter %q: %s", c.flagFormat, err))
		return 1
	}

	out, err := formatter.FormatSession(s)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to format session: %s", err))
		return 1
	}
	if out != "" {
		c.UI.Info(out)
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
	synopsis = "Read session information"
	help     = `
Usage: consul session read [options] SESSIONID

    This command will retrieve and print out the details of a single session.

    Read:

        $ consul acl session read b2caae8a-e80e-15f4-17aa-2be947c7968e

    Read and format the result as JSON:

        $ consul acl session read -format=json b2caae8a-e80e-15f4-17aa-2be947c7968e
`
)
