// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policydelete

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

	policyID   string
	policyName string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.policyID, "id", "", "The ID of the policy to delete. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple policy IDs")
	c.flags.StringVar(&c.policyName, "name", "", "The name of the policy to delete.")
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

	if _, err := client.ACL().PolicyDelete(policyID, nil); err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting policy %q: %v", policyID, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Policy %q deleted successfully", policyID))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Delete an ACL policy"
	help     = `
Usage: consul acl policy delete [options] -id POLICY

    Deletes an ACL policy by providing either the ID or a unique ID prefix.

    Delete by prefix:

        $ consul acl policy delete -id b6b85

    Delete by full ID:

        $ consul acl policy delete -id b6b856da-5193-4e78-845a-7d61ca8371ba

`
)
