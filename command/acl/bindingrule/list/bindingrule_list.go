// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package bindingrulelist

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/command/acl/bindingrule"
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

	authMethodName string

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
		&c.authMethodName,
		"method",
		"",
		"Only show rules linked to the auth method with the given name.",
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

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	rules, _, err := client.ACL().BindingRuleList(c.authMethodName, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to retrieve the binding rule list: %v", err))
		return 1
	}

	formatter, err := bindingrule.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	out, err := formatter.FormatBindingRuleList(rules)
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
	synopsis = "Lists ACL binding rules"
	help     = `
Usage: consul acl binding-rule list [options]

  Lists all the ACL binding rules.

  Show all:

    $ consul acl binding-rule list

  Show all for a specific auth method:

    $ consul acl binding-rule list -method="my-method"
`
)
