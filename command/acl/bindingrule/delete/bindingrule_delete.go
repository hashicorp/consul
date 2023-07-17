package bindingruledelete

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

	ruleID string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(
		&c.ruleID,
		"id",
		"",
		"The ID of the binding rule to delete. "+
			"It may be specified as a unique ID prefix but will error if the prefix "+
			"matches multiple binding rule IDs",
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

	if c.ruleID == "" {
		c.UI.Error(fmt.Sprintf("Must specify the -id parameter"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	ruleID, err := acl.GetBindingRuleIDFromPartial(client, c.ruleID)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining binding rule ID: %v", err))
		return 1
	}

	if _, err := client.ACL().BindingRuleDelete(ruleID, nil); err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting binding rule %q: %v", ruleID, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Binding rule %q deleted successfully", ruleID))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Delete an ACL binding rule"
	help     = `
Usage: consul acl binding-rule delete -id ID [options]

  Deletes an ACL binding rule by providing the ID or a unique ID prefix.

  Delete by prefix:

    $ consul acl binding-rule delete -id b6b85

  Delete by full ID:

    $ consul acl binding-rule delete -id b6b856da-5193-4e78-845a-7d61ca8371ba
`
)
