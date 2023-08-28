// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tokencreate

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/acl/token"
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

	accessor      string
	secret        string
	policyIDs     []string
	policyNames   []string
	description   string
	roleIDs       []string
	roleNames     []string
	serviceIdents []string
	nodeIdents    []string
	expirationTTL time.Duration
	local         bool
	showMeta      bool
	format        string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.accessor, "accessor", "", "Create the token with this Accessor ID. "+
		"It must be a UUID. If not specified one will be auto-generated")
	c.flags.StringVar(&c.secret, "secret", "", "Create the token with this Secret ID. "+
		"It must be a UUID. If not specified one will be auto-generated")
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that token metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.BoolVar(&c.local, "local", false, "Create this as a datacenter local token")
	c.flags.StringVar(&c.description, "description", "", "A description of the token")
	c.flags.Var((*flags.AppendSliceValue)(&c.policyIDs), "policy-id", "ID of a "+
		"policy to use for this token. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.policyNames), "policy-name", "Name of a "+
		"policy to use for this token. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.roleIDs), "role-id", "ID of a "+
		"role to use for this token. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.roleNames), "role-name", "Name of a "+
		"role to use for this token. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.serviceIdents), "service-identity", "Name of a "+
		"service identity to use for this token. May be specified multiple times. Format is "+
		"the SERVICENAME or SERVICENAME:DATACENTER1,DATACENTER2,...")
	c.flags.Var((*flags.AppendSliceValue)(&c.nodeIdents), "node-identity", "Name of a "+
		"node identity to use for this token. May be specified multiple times. Format is "+
		"NODENAME:DATACENTER")
	c.flags.DurationVar(&c.expirationTTL, "expires-ttl", 0, "Duration of time this "+
		"token should be valid for")
	c.flags.StringVar(
		&c.format,
		"format",
		token.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(token.GetSupportedFormats(), "|")),
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

	if len(c.policyNames) == 0 && len(c.policyIDs) == 0 &&
		len(c.roleNames) == 0 && len(c.roleIDs) == 0 &&
		len(c.serviceIdents) == 0 && len(c.nodeIdents) == 0 {
		c.UI.Error(fmt.Sprintf("Cannot create a token without specifying -policy-name, -policy-id, -role-name, -role-id, -service-identity, or -node-identity at least once"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	newToken := &api.ACLToken{
		Description: c.description,
		Local:       c.local,
		AccessorID:  c.accessor,
		SecretID:    c.secret,
	}
	if c.expirationTTL > 0 {
		newToken.ExpirationTTL = c.expirationTTL
	}

	parsedServiceIdents, err := acl.ExtractServiceIdentities(c.serviceIdents)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	newToken.ServiceIdentities = parsedServiceIdents

	parsedNodeIdents, err := acl.ExtractNodeIdentities(c.nodeIdents)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	newToken.NodeIdentities = parsedNodeIdents

	for _, policyName := range c.policyNames {
		// We could resolve names to IDs here but there isn't any reason why its would be better
		// than allowing the agent to do it.
		newToken.Policies = append(newToken.Policies, &api.ACLTokenPolicyLink{Name: policyName})
	}

	for _, policyID := range c.policyIDs {
		policyID, err := acl.GetPolicyIDFromPartial(client, policyID)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error resolving policy ID %s: %v", policyID, err))
			return 1
		}
		newToken.Policies = append(newToken.Policies, &api.ACLTokenPolicyLink{ID: policyID})
	}

	for _, roleName := range c.roleNames {
		// We could resolve names to IDs here but there isn't any reason why its would be better
		// than allowing the agent to do it.
		newToken.Roles = append(newToken.Roles, &api.ACLTokenRoleLink{Name: roleName})
	}

	for _, roleID := range c.roleIDs {
		roleID, err := acl.GetRoleIDFromPartial(client, roleID)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error resolving role ID %s: %v", roleID, err))
			return 1
		}
		newToken.Roles = append(newToken.Roles, &api.ACLTokenRoleLink{ID: roleID})
	}

	t, _, err := client.ACL().TokenCreate(newToken, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to create new token: %v", err))
		return 1
	}

	formatter, err := token.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatToken(t)
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
	synopsis = "Create an ACL token"
	help     = `
Usage: consul acl token create [options]

  When creating a new token policies may be linked using either the -policy-id
  or the -policy-name options. When specifying policies by IDs you may use a
  unique prefix of the UUID as a shortcut for specifying the entire UUID.

  Create a new token:

          $ consul acl token create -description "Replication token" \
                                    -policy-id b52fc3de-5 \
                                    -policy-name "acl-replication" \
                                    -role-id c630d4ef-6 \
                                    -role-name "db-updater" \
                                    -service-identity "web" \
                                    -service-identity "db:east,west"
`
)
