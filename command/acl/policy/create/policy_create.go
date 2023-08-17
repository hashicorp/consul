// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package policycreate

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl/policy"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
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

	name        string
	description string
	datacenters []string
	rules       string

	showMeta bool
	format   string

	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that policy metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(&c.name, "name", "", "The new policy's name. This flag is required.")
	c.flags.StringVar(&c.description, "description", "", "A description of the policy")
	c.flags.Var((*flags.AppendSliceValue)(&c.datacenters), "valid-datacenter", "Datacenter "+
		"that the policy should be valid within. This flag may be specified multiple times")
	c.flags.StringVar(&c.rules, "rules", "", "The policy rules. May be prefixed with '@' "+
		"to indicate that the value is a file path to load the rules from. '-' may also be "+
		"given to indicate that the rules are available on stdin")
	c.flags.StringVar(
		&c.format,
		"format",
		policy.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(policy.GetSupportedFormats(), "|")),
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

	if c.name == "" {
		c.UI.Error(fmt.Sprintf("Missing require '-name' flag"))
		c.UI.Error(c.Help())
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	rules, err := helpers.LoadDataSource(c.rules, c.testStdin)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error loading rules: %v", err))
		return 1
	}

	newPolicy := &api.ACLPolicy{
		Name:        c.name,
		Description: c.description,
		Datacenters: c.datacenters,
		Rules:       rules,
	}

	p, _, err := client.ACL().PolicyCreate(newPolicy, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to create new policy: %v", err))
		return 1
	}

	formatter, err := policy.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatPolicy(p)
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
	synopsis = "Create an ACL policy"
	help     = `
Usage: consul acl policy create -name NAME [options]

    The -rules option values allows loading the value from stdin, a file 
    or the raw value. To use stdin pass '-' as the value. To load the value 
    from a file prefix the value with an '@'. Any other values will be used 
    directly.

    Create a new policy:

        $ consul acl policy create -name "new-policy" \
                                   -description "This is an example policy" \
                                   -datacenter "dc1" \
                                   -datacenter "dc2" \
                                   -rules @rules.hcl
`
)
