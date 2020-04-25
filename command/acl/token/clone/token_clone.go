package tokenclone

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/acl/token"
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

	tokenID     string
	description string
	format      string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.tokenID, "id", "", "The Accessor ID of the token to clone. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple token Accessor IDs. The special value of 'anonymous' may "+
		"be provided instead of the anonymous tokens accessor ID")
	c.flags.StringVar(&c.description, "description", "", "A description of the new cloned token")
	c.flags.StringVar(
		&c.format,
		"format",
		token.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(token.GetSupportedFormats(), "|")),
	)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.NamespaceFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.tokenID == "" {
		c.UI.Error(fmt.Sprintf("Cannot update a token without specifying the -id parameter"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	tokenID, err := acl.GetTokenIDFromPartial(client, c.tokenID)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining token ID: %v", err))
		return 1
	}

	t, _, err := client.ACL().TokenClone(tokenID, c.description, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error cloning token: %v", err))
		return 1
	}

	formatter, err := token.NewFormatter(c.format, false)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatToken(t)
	if err != nil {
		c.UI.Error(err.Error())
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
	return flags.Usage(c.help, nil)
}

const synopsis = "Clone an ACL token"
const help = `
Usage: consul acl token clone [options]

    This command will clone a token. When cloning an alternate description may be given
    for use with the new token.

    Example:

        $ consul acl token clone -id abcd -description "replication"
`
