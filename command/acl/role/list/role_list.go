package rolelist

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/command/acl/role"
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

	showMeta bool
	format   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that policy metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(
		&c.format,
		"format",
		role.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(role.GetSupportedFormats(), "|")),
	)

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

	roles, _, err := client.ACL().RoleList(nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to retrieve the role list: %v", err))
		return 1
	}

	formatter, err := role.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatRoleList(roles)
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

const (
	synopsis = "Lists ACL roles"
	help     = `
Usage: consul acl role list [options]

    Lists all the ACL roles.

    Example:

        $ consul acl role list
`
)
