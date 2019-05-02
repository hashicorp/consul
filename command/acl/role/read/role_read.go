package roleread

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
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

	roleID   string
	roleName string
	showMeta bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that role metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(&c.roleID, "id", "", "The ID of the role to read. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple policy IDs")
	c.flags.StringVar(&c.roleName, "name", "", "The name of the role to read.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
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

	var role *api.ACLRole

	if c.roleID != "" {
		roleID, err := acl.GetRoleIDFromPartial(client, c.roleID)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error determining role ID: %v", err))
			return 1
		}
		role, _, err = client.ACL().RoleRead(roleID, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading role %q: %v", roleID, err))
			return 1
		} else if role == nil {
			c.UI.Error(fmt.Sprintf("Role not found with ID %q", roleID))
			return 1
		}

	} else {
		role, _, err = client.ACL().RoleReadByName(c.roleName, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading role %q: %v", c.roleName, err))
			return 1
		} else if role == nil {
			c.UI.Error(fmt.Sprintf("Role not found with name %q", c.roleName))
			return 1
		}
	}

	acl.PrintRole(role, c.UI, c.showMeta)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Read an ACL role"
const help = `
Usage: consul acl role read [options] ROLE

    This command will retrieve and print out the details
    of a single role.

    Read:

        $ consul acl role read -id fdabbcb5-9de5-4b1a-961f-77214ae88cba

    Read by name:

        $ consul acl role read -name my-policy

`
