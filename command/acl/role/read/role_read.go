package roleread

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/acl/role"
	"github.com/hashicorp/consul/command/flags"
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

	roleID   string
	roleName string
	showMeta bool
	format   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that role metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(&c.roleID, "id", "", "The ID of the role to read. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple policy IDs")
	c.flags.StringVar(&c.roleName, "name", "", "The name of the role to read.")
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

	if c.roleID == "" && c.roleName == "" {
		c.UI.Error(fmt.Sprintf("Must specify either the -id or -name parameters"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var r *api.ACLRole

	if c.roleID != "" {
		roleID, err := acl.GetRoleIDFromPartial(client, c.roleID)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error determining role ID: %v", err))
			return 1
		}
		r, _, err = client.ACL().RoleRead(roleID, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading role %q: %v", roleID, err))
			return 1
		} else if r == nil {
			c.UI.Error(fmt.Sprintf("Role not found with ID %q", roleID))
			return 1
		}

	} else {
		r, _, err = client.ACL().RoleReadByName(c.roleName, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading role %q: %v", c.roleName, err))
			return 1
		} else if r == nil {
			c.UI.Error(fmt.Sprintf("Role not found with name %q", c.roleName))
			return 1
		}
	}

	formatter, err := role.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatRole(r)
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
	synopsis = "Read an ACL role"
	help     = `
Usage: consul acl role read [options] ROLE

    This command will retrieve and print out the details
    of a single role.

    Read:

        $ consul acl role read -id fdabbcb5-9de5-4b1a-961f-77214ae88cba

    Read by name:

        $ consul acl role read -name my-policy

`
)
