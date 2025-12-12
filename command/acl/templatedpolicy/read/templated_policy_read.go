// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package templatedpolicyread

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl/templatedpolicy"
	"github.com/hashicorp/consul/command/flags"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
	synopsis            = "Read an ACL Templated Policy"
	help                = `
Usage: consul acl templated-policy read [options] TEMPLATED_POLICY

  This command will retrieve and print out the details of a single templated policy.

  Example:

      $ consul acl templated-policy read -name templated-policy-name
`
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

	templateName string
	format       string
	showMeta     bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.templateName, "name", "", "The name of the templated policy to read.")
	c.flags.StringVar(
		&c.format,
		"format",
		templatedpolicy.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(templatedpolicy.GetSupportedFormats(), "|")),
	)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that templated policy metadata such "+
		"as the schema and template code should be shown for each entry.")
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

	if c.templateName == "" {
		c.UI.Error("Must specify the -name parameter")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var tp *api.ACLTemplatedPolicyResponse

	tp, _, err = client.ACL().TemplatedPolicyReadByName(c.templateName, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading templated policy %q: %v", c.templateName, err))
		return 1
	} else if tp == nil {
		c.UI.Error(fmt.Sprintf("Templated policy not found with name %q", c.templateName))
		return 1
	}

	formatter, err := templatedpolicy.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatTemplatedPolicy(*tp)
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
