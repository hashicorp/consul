// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package policyupdate

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
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

	policyID       string
	nameSet        bool
	name           string
	descriptionSet bool
	description    string
	datacenters    []string
	rulesSet       bool
	rules          string
	noMerge        bool
	showMeta       bool
	format         string
	testStdin      io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that policy metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(&c.policyID, "id", "", "The ID of the policy to update. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple policy IDs")
	c.flags.StringVar(&c.name, "name", "", "The policies name.")
	c.flags.StringVar(&c.description, "description", "", "A description of the policy")
	c.flags.Var((*flags.AppendSliceValue)(&c.datacenters), "valid-datacenter", "Datacenter "+
		"that the policy should be valid within. This flag may be specified multiple times")
	c.flags.StringVar(&c.rules, "rules", "", "The policy rules. May be prefixed with '@' "+
		"to indicate that the value is a file path to load the rules from. '-' may also be "+
		"given to indicate that the rules are available on stdin")
	c.flags.BoolVar(&c.noMerge, "no-merge", false, "Do not merge the current policy "+
		"information with what is provided to the command. Instead overwrite all fields "+
		"with the exception of the policy ID which is immutable.")
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

func (c *cmd) checkSet(f *flag.Flag) {
	switch f.Name {
	case "name":
		c.nameSet = true
	case "description":
		c.descriptionSet = true
	case "rules":
		c.rulesSet = true
	}
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	c.flags.Visit(c.checkSet)

	if c.policyID == "" && c.name == "" {
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
		policyID, err = acl.GetPolicyIDByName(client, c.name)
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining policy ID: %v", err))
		return 1
	}

	rules, err := helpers.LoadDataSource(c.rules, c.testStdin)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error loading data source: %v", err))
		return 1
	}
	var updated *api.ACLPolicy
	if c.noMerge {
		updated = &api.ACLPolicy{
			ID:          policyID,
			Name:        c.name,
			Description: c.description,
			Datacenters: c.datacenters,
			Rules:       rules,
		}
	} else {
		p, _, err := client.ACL().PolicyRead(policyID, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading policy %q: %v", policyID, err))
			return 1
		}

		updated = &api.ACLPolicy{
			ID:          policyID,
			Name:        p.Name,
			Description: p.Description,
			Datacenters: p.Datacenters,
			Rules:       p.Rules,
		}

		if c.nameSet {
			updated.Name = c.name
		}
		if c.descriptionSet {
			updated.Description = c.description
		}
		if c.rulesSet {
			updated.Rules = rules
		}
		if c.datacenters != nil {
			updated.Datacenters = c.datacenters
		}
	}

	p, _, err := client.ACL().PolicyUpdate(updated, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error updating policy %q: %v", policyID, err))
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
	synopsis = "Update an ACL policy"
	help     = `
Usage: consul acl policy update [options]

  Updates a policy. By default it will merge the policy information with its
  current state so that you do not have to provide all parameters. This
  behavior can be disabled by passing -no-merge.

  Rename the policy:

          $ consul acl policy update -id abcd -name "better-name"

  Override all policy attributes:

          # this will remove any datacenter scope if provided and will remove
          # the description
          $consul acl policy update -id abcd -name "better-name" -rules @rules.hcl
`
)
