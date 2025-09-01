// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package templatedpolicylist

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/acl/policy"
	"github.com/hashicorp/consul/command/acl/templatedpolicy"
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

	templatedPolicyName      string
	templatedPolicyFile      string
	templatedPolicyVariables []string
	format                   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(
		&c.format,
		"format",
		templatedpolicy.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(templatedpolicy.GetSupportedFormats(), "|")),
	)
	c.flags.Var((*flags.AppendSliceValue)(&c.templatedPolicyVariables), "var", "Templated policy variables."+
		" Must be used in combination with -name flag to specify required variables."+
		" May be specified multiple times with different variables."+
		" Format is VariableName:Value")
	c.flags.StringVar(&c.templatedPolicyName, "name", "", "The templated policy name.  Use -var flag to specify variables when required.")
	c.flags.StringVar(&c.templatedPolicyFile, "file", "", "Path to a file containing templated policies and variables.")

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

	if len(c.templatedPolicyName) == 0 && len(c.templatedPolicyFile) == 0 {
		c.UI.Error("Cannot preview a templated policy without specifying -name or -file")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	parsedTemplatedPolicies, err := acl.ExtractTemplatedPolicies(c.templatedPolicyName, c.templatedPolicyFile, c.templatedPolicyVariables)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if len(parsedTemplatedPolicies) != 1 {
		c.UI.Error("Can only preview a single templated policy at a time.")
		return 1
	}

	syntheticPolicy, _, err := client.ACL().TemplatedPolicyPreview(parsedTemplatedPolicies[0], nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to generate the templated policy preview: %v", err))
		return 1
	}

	formatter, err := policy.NewFormatter(c.format, false)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatPolicy(syntheticPolicy)
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
	synopsis = "Preview the policy rendered by the ACL templated policy"
	help     = `
Usage: consul acl templated-policy preview [options]

    Preview the policy rendered by the ACL templated policy.

    Example:

        $ consul acl templated-policy preview -name "builtin/service" -var "name:api"

    Preview a templated policy using a file.

    Example:

        $ consul acl templated-policy preview -file templated-policy-file.hcl
`
)
