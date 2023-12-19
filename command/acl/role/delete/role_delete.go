package roledelete

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/acl"
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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.roleID, "id", "", "The ID of the role to delete. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple role IDs")
	c.flags.StringVar(&c.roleName, "name", "", "The name of the role to delete.")
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
		c.UI.Error(fmt.Sprintf("Must specify the -id or -name parameters"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var roleID string
	if c.roleID != "" {
		roleID, err = acl.GetRoleIDFromPartial(client, c.roleID)
	} else {
		roleID, err = acl.GetRoleIDByName(client, c.roleName)
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining role ID: %v", err))
		return 1
	}

	if _, err := client.ACL().RoleDelete(roleID, nil); err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting role %q: %v", roleID, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Role %q deleted successfully", roleID))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Delete an ACL role"
	help     = `
Usage: consul acl role delete [options] -id ROLE

    Deletes an ACL role by providing the ID or a unique ID prefix.

    Delete by prefix:

        $ consul acl role delete -id b6b85

    Delete by full ID:

        $ consul acl role delete -id b6b856da-5193-4e78-845a-7d61ca8371ba

`
)
