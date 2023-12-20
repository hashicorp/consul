// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bindingruleupdate

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
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

	description string
	selector    string
	bindType    string
	bindName    string

	noMerge  bool
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
		"The ID of the binding rule to update. "+
			"It may be specified as a unique ID prefix but will error if the prefix "+
			"matches multiple binding rule IDs",
	)

	c.flags.StringVar(
		&c.description,
		"description",
		"",
		"A description of the binding rule.",
	)
	c.flags.StringVar(
		&c.selector,
		"selector",
		"",
		"Selector is an expression that matches against verified identity "+
			"attributes returned from the auth method during login.",
	)
	c.flags.StringVar(
		&c.bindType,
		"bind-type",
		string(api.BindingRuleBindTypeService),
		"Type of binding to perform (\"service\" or \"role\").",
	)
	c.flags.StringVar(
		&c.bindName,
		"bind-name",
		"",
		"Name to bind on match. Can use ${var} interpolation. "+
			"This flag is required.",
	)

	c.flags.BoolVar(
		&c.noMerge,
		"no-merge",
		false,
		"Do not merge the current binding rule "+
			"information with what is provided to the command. Instead overwrite all fields "+
			"with the exception of the binding rule ID which is immutable.",
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
		c.UI.Error(fmt.Sprintf("Cannot update a binding rule without specifying the -id parameter"))
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

	// Read the current binding rule in both cases so we can fail better if not found.
	currentRule, _, err := client.ACL().BindingRuleRead(ruleID, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error when retrieving current binding rule: %v", err))
		return 1
	} else if currentRule == nil {
		c.UI.Error(fmt.Sprintf("Binding rule not found with ID %q", ruleID))
		return 1
	}

	var rule *api.ACLBindingRule
	if c.noMerge {
		if c.bindType == "" {
			c.UI.Error(fmt.Sprintf("Missing required '-bind-type' flag"))
			c.UI.Error(c.Help())
			return 1
		} else if c.bindName == "" {
			c.UI.Error(fmt.Sprintf("Missing required '-bind-name' flag"))
			c.UI.Error(c.Help())
			return 1
		}

		rule = &api.ACLBindingRule{
			ID:          ruleID,
			AuthMethod:  currentRule.AuthMethod, // immutable
			Description: c.description,
			BindType:    api.BindingRuleBindType(c.bindType),
			BindName:    c.bindName,
			Selector:    c.selector,
		}

	} else {
		rule = currentRule

		if c.description != "" {
			rule.Description = c.description
		}
		if c.bindType != "" {
			rule.BindType = api.BindingRuleBindType(c.bindType)
		}
		if c.bindName != "" {
			rule.BindName = c.bindName
		}
		if isFlagSet(c.flags, "selector") {
			rule.Selector = c.selector // empty is valid
		}
	}

	rule, _, err = client.ACL().BindingRuleUpdate(rule, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error updating binding rule %q: %v", ruleID, err))
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

func isFlagSet(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

const (
	synopsis = "Update an ACL binding rule"
	help     = `
Usage: consul acl binding-rule update -id ID [options]

  Updates a binding rule. By default it will merge the binding rule
  information with its current state so that you do not have to provide all
  parameters. This behavior can be disabled by passing -no-merge.

  Update all editable fields of the binding rule:

    $ consul acl binding-rule update \
          -id=43cb72df-9c6f-4315-ac8a-01a9d98155ef \
          -description="new description" \
          -bind-type=role \
          -bind-name='k8s-${serviceaccount.name}' \
          -selector='serviceaccount.namespace==default and serviceaccount.name==web'
`
)
