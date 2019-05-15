package policyread

import (
	"flag"
	"fmt"

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

	policyID   string
	policyName string
	showMeta   bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that policy metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(&c.policyID, "id", "", "The ID of the policy to read. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple policy IDs")
	c.flags.StringVar(&c.policyName, "name", "", "The name of the policy to read.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.policyID == "" && c.policyName == "" {
		c.UI.Error(fmt.Sprintf("Must specify either the -id or -name parameters"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var policyID string
	if c.policyID != "" {
		policyID, err = acl.GetPolicyIDFromPartial(client, c.policyID)
	} else {
		policyID, err = acl.GetPolicyIDByName(client, c.policyName)
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining policy ID: %v", err))
		return 1
	}

	policy, _, err := client.ACL().PolicyRead(policyID, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading policy %q: %v", policyID, err))
		return 1
	}
	acl.PrintPolicy(policy, c.UI, c.showMeta)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Read an ACL policy"
const help = `
Usage: consul acl policy read [options] POLICY

    This command will retrieve and print out the details
    of a single policy.

    Read:

        $ consul acl policy read -id fdabbcb5-9de5-4b1a-961f-77214ae88cba

    Read by name:

        $ consul acl policy read -name my-policy

`
