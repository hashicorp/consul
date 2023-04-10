// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bindingruleread

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/acl/bindingrule"
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

	showMeta bool
	format   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.BoolVar(
		&c.showMeta,
		"meta",
		false,
		"Indicates that binding rule metadata such "+
			"as the raft indices should be shown for each entry.",
	)

	c.flags.StringVar(
		&c.ruleID,
		"id",
		"",
		"The ID of the binding rule to read. "+
			"It may be specified as a unique ID prefix but will error if the prefix "+
			"matches multiple binding rule IDs",
	)

	c.flags.StringVar(
		&c.format,
		"format",
		bindingrule.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(bindingrule.GetSupportedFormats(), "|")),
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
		c.UI.Error(fmt.Sprintf("Must specify the -id parameter."))
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

	rule, _, err := client.ACL().BindingRuleRead(ruleID, nil)
	switch {
	case err != nil:
		c.UI.Error(fmt.Sprintf("Error reading binding rule %q: %v", ruleID, err))
		return 1
	case rule == nil:
		c.UI.Error(fmt.Sprintf("Binding rule not found with ID %q", ruleID))
		return 1
	}

	formatter, err := bindingrule.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	out, err := formatter.FormatBindingRule(rule)
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
	synopsis = "Read an ACL binding rule"
	help     = `
Usage: consul acl binding-rule read -id ID [options]

  This command will retrieve and print out the details of a single binding
  rule.

  Read a binding rule:

    $ consul acl binding-rule read -id fdabbcb5-9de5-4b1a-961f-77214ae88cba
`
)
