package list

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
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
	flagNode   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.flagNode, "node", "",
		"List the active sessions for a given node.")
	c.flags.StringVar(&c.flagFormat, "format", session.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(session.GetSupportedFormats(), "|")))

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

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var s []*api.SessionEntry
	if c.flagNode == "" {
		s, _, err = client.Session().List(nil)
	} else {
		s, _, err = client.Session().Node(c.flagNode, nil)
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing sessions: %s", err))
		return 1
	}

	formatter, err := session.NewFormatter(c.flagFormat)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to get formatter %q: %s", c.flagFormat, err))
		return 1
	}

	out, err := formatter.FormatSessionList(s)
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
	synopsis = "List active sessions"
	help     = `
Usage: consul session list [options]

    List all the active sessions.

    List all sessions in the cluster:

        $ consul acl session list

    List all sessions for a given node:

        $ consul acl session list -node=s1234.dc1
`
)
