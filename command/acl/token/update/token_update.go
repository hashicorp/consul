// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tokenupdate

import (
	"flag"
	"fmt"
	"strings"

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

	tokenAccessorID     string
	policyIDs           []string
	appendPolicyIDs     []string
	policyNames         []string
	appendPolicyNames   []string
	roleIDs             []string
	appendRoleIDs       []string
	roleNames           []string
	appendRoleNames     []string
	serviceIdents       []string
	nodeIdents          []string
	appendNodeIdents    []string
	appendServiceIdents []string
	description         string
	showMeta            bool
	format              string

	// DEPRECATED
	mergeServiceIdents bool
	mergeNodeIdents    bool
	mergeRoles         bool
	mergePolicies      bool
	tokenID            string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that token metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(&c.tokenAccessorID, "accessor-id", "", "The Accessor ID of the token to update. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple token Accessor IDs")
	c.flags.StringVar(&c.description, "description", "", "A description of the token")
	c.flags.Var((*flags.AppendSliceValue)(&c.policyIDs), "policy-id", "ID of a "+
		"policy to use for this token. Overwrites existing policies. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.appendPolicyIDs), "append-policy-id", "ID of a "+
		"policy to use for this token. The token retains existing policies. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.policyNames), "policy-name", "Name of a "+
		"policy to use for this token. Overwrites existing policies. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.appendPolicyNames), "append-policy-name", "Name of a "+
		"policy to add to this token. The token retains existing policies. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.roleIDs), "role-id", "ID of a "+
		"role to use for this token. Overwrites existing roles. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.roleNames), "role-name", "Name of a "+
		"role to use for this token. Overwrites existing roles. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.appendRoleIDs), "append-role-id", "ID of a "+
		"role to add to this token. The token retains existing roles. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.appendRoleNames), "append-role-name", "Name of a "+
		"role to add to this token. The token retains existing roles. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.serviceIdents), "service-identity", "Name of a "+
		"service identity to use for this token. May be specified multiple times. Format is "+
		"the SERVICENAME or SERVICENAME:DATACENTER1,DATACENTER2,...")
	c.flags.Var((*flags.AppendSliceValue)(&c.appendServiceIdents), "append-service-identity", "Name of a "+
		"service identity to use for this token. This token retains existing service identities. May be specified"+
		"multiple times. Format is the SERVICENAME or SERVICENAME:DATACENTER1,DATACENTER2,...")
	c.flags.Var((*flags.AppendSliceValue)(&c.nodeIdents), "node-identity", "Name of a "+
		"node identity to use for this token. May be specified multiple times. Format is "+
		"NODENAME:DATACENTER")
	c.flags.Var((*flags.AppendSliceValue)(&c.appendNodeIdents), "append-node-identity", "Name of a "+
		"node identity to use for this token. This token retains existing node identities. May be "+
		"specified multiple times. Format is NODENAME:DATACENTER")
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

	// Deprecations
	c.flags.StringVar(&c.tokenID, "id", "", "DEPRECATED. Use -accessor-id instead.")
	c.flags.BoolVar(&c.mergePolicies, "merge-policies", false, "DEPRECATED. "+
		"Use -append-policy-id or -append-policy-name instead.")
	c.flags.BoolVar(&c.mergeRoles, "merge-roles", false, "DEPRECATED. "+
		"Use -append-role-id or -append-role-name instead.")
	c.flags.BoolVar(&c.mergeServiceIdents, "merge-service-identities", false, "DEPRECATED. "+
		"Use -append-service-identity instead.")
	c.flags.BoolVar(&c.mergeNodeIdents, "merge-node-identities", false, "DEPRECATED. "+
		"Use -append-node-identity instead.")
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	tokenAccessor := c.tokenAccessorID
	if tokenAccessor == "" {
		if c.tokenID == "" {
			c.UI.Error("Cannot update a token without specifying the -accessor-id parameter")
			return 1
		} else {
			tokenAccessor = c.tokenID
			c.UI.Warn("Use the -accessor-id parameter to specify token by Accessor ID")
		}
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	tok, err := acl.GetTokenAccessorIDFromPartial(client, tokenAccessor)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining token ID: %v", err))
		return 1
	}

	t, _, err := client.ACL().TokenRead(tok, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error when retrieving current token: %v", err))
		return 1
	}

	if c.description != "" {
		// Only update description if the user specified a new one. This does make
		// it impossible to completely clear descriptions from CLI but that seems
		// better than silently deleting descriptions when using command without
		// manually giving the new description. If it's a real issue we can always
		// add another explicit `-remove-description` flag but it feels like an edge
		// case that's not going to be critical to anyone.
		t.Description = c.description
	}

	hasAppendServiceFields := len(c.appendServiceIdents) > 0
	hasServiceFields := len(c.serviceIdents) > 0
	if hasAppendServiceFields && hasServiceFields {
		c.UI.Error("Cannot combine the use of service-identity flag with append-service-identity. " +
			"To set or overwrite existing service identities, use -service-identity. " +
			"To append to existing service identities, use -append-service-identity.")
		return 1
	}

	parsedServiceIdents, err := acl.ExtractServiceIdentities(c.serviceIdents)
	if hasAppendServiceFields {
		parsedServiceIdents, err = acl.ExtractServiceIdentities(c.appendServiceIdents)
	}
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	hasAppendNodeFields := len(c.appendNodeIdents) > 0
	hasNodeFields := len(c.nodeIdents) > 0

	if hasAppendNodeFields && hasNodeFields {
		c.UI.Error("Cannot combine the use of node-identity flag with append-node-identity. " +
			"To set or overwrite existing node identities, use -node-identity. " +
			"To append to existing node identities, use -append-node-identity.")
		return 1
	}

	parsedNodeIdents, err := acl.ExtractNodeIdentities(c.nodeIdents)
	if hasAppendNodeFields {
		parsedNodeIdents, err = acl.ExtractNodeIdentities(c.appendNodeIdents)
	}
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if c.mergePolicies {
		c.UI.Warn("merge-policies is deprecated and will be removed in a future Consul version. " +
			"Use `append-policy-name` or `append-policy-id` instead.")

		for _, policyName := range c.policyNames {
			found := false
			for _, link := range t.Policies {
				if link.Name == policyName {
					found = true
					break
				}
			}

			if !found {
				// We could resolve names to IDs here but there isn't any reason why its would be better
				// than allowing the agent to do it.
				t.Policies = append(t.Policies, &api.ACLTokenPolicyLink{Name: policyName})
			}
		}

		for _, policyID := range c.policyIDs {
			policyID, err := acl.GetPolicyIDFromPartial(client, policyID)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error resolving policy ID %s: %v", policyID, err))
				return 1
			}
			found := false

			for _, link := range t.Policies {
				if link.ID == policyID {
					found = true
					break
				}
			}

			if !found {
				t.Policies = append(t.Policies, &api.ACLTokenPolicyLink{ID: policyID})
			}
		}
	} else {

		hasAddPolicyFields := len(c.appendPolicyNames) > 0 || len(c.appendPolicyIDs) > 0
		hasPolicyFields := len(c.policyIDs) > 0 || len(c.policyNames) > 0

		if hasPolicyFields && hasAddPolicyFields {
			c.UI.Error("Cannot combine the use of policy-id/policy-name flags with append- variants. " +
				"To set or overwrite existing policies, use -policy-id or -policy-name. " +
				"To append to existing policies, use -append-policy-id or -append-policy-name.")
			return 1
		}

		policyIDs := c.appendPolicyIDs
		policyNames := c.appendPolicyNames

		if hasPolicyFields {
			policyIDs = c.policyIDs
			policyNames = c.policyNames
			t.Policies = nil
		}

		for _, policyName := range policyNames {
			// We could resolve names to IDs here but there isn't any reason why its would be better
			// than allowing the agent to do it.
			t.Policies = append(t.Policies, &api.ACLTokenPolicyLink{Name: policyName})
		}

		for _, policyID := range policyIDs {
			policyID, err := acl.GetPolicyIDFromPartial(client, policyID)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error resolving policy ID %s: %v", policyID, err))
				return 1
			}
			t.Policies = append(t.Policies, &api.ACLTokenPolicyLink{ID: policyID})
		}
	}

	if c.mergeRoles {
		c.UI.Warn("merge-roles is deprecated and will be removed in a future Consul version. " +
			"Use `append-role-name` or `append-role-id` instead.")

		for _, roleName := range c.roleNames {
			found := false
			for _, link := range t.Roles {
				if link.Name == roleName {
					found = true
					break
				}
			}

			if !found {
				// We could resolve names to IDs here but there isn't any reason why its would be better
				// than allowing the agent to do it.
				t.Roles = append(t.Roles, &api.ACLTokenRoleLink{Name: roleName})
			}
		}

		for _, roleID := range c.roleIDs {
			roleID, err := acl.GetRoleIDFromPartial(client, roleID)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error resolving role ID %s: %v", roleID, err))
				return 1
			}
			found := false

			for _, link := range t.Roles {
				if link.ID == roleID {
					found = true
					break
				}
			}

			if !found {
				t.Roles = append(t.Roles, &api.ACLTokenRoleLink{Name: roleID})
			}
		}
	} else {
		hasAddRoleFields := len(c.appendRoleNames) > 0 || len(c.appendRoleIDs) > 0
		hasRoleFields := len(c.roleIDs) > 0 || len(c.roleNames) > 0

		if hasRoleFields && hasAddRoleFields {
			c.UI.Error("Cannot combine the use of role-id/role-name flags with append- variants. " +
				"To set or overwrite existing roles, use -role-id or -role-name. " +
				"To append to existing roles, use -append-role-id or -append-role-name.")
			return 1
		}

		roleNames := c.appendRoleNames
		roleIDs := c.appendRoleIDs

		if hasRoleFields {
			roleNames = c.roleNames
			roleIDs = c.roleIDs
			t.Roles = nil
		}

		for _, roleName := range roleNames {
			// We could resolve names to IDs here but there isn't any reason why its would be better
			// than allowing the agent to do it.
			t.Roles = append(t.Roles, &api.ACLTokenRoleLink{Name: roleName})
		}

		for _, roleID := range roleIDs {
			roleID, err := acl.GetRoleIDFromPartial(client, roleID)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error resolving role ID %s: %v", roleID, err))
				return 1
			}
			t.Roles = append(t.Roles, &api.ACLTokenRoleLink{ID: roleID})
		}
	}

	if c.mergeServiceIdents || hasAppendServiceFields {
		for _, svcid := range parsedServiceIdents {
			found := -1
			for i, link := range t.ServiceIdentities {
				if link.ServiceName == svcid.ServiceName {
					found = i
					break
				}
			}

			if found != -1 {
				t.ServiceIdentities[found] = svcid
			} else {
				t.ServiceIdentities = append(t.ServiceIdentities, svcid)
			}
		}
	} else {
		t.ServiceIdentities = parsedServiceIdents
	}

	if c.mergeNodeIdents || hasAppendNodeFields {
		for _, nodeid := range parsedNodeIdents {
			found := false
			for _, link := range t.NodeIdentities {
				if link.NodeName == nodeid.NodeName && link.Datacenter == nodeid.Datacenter {
					found = true
					break
				}
			}

			if !found {
				t.NodeIdentities = append(t.NodeIdentities, nodeid)
			}
		}
	} else {
		t.NodeIdentities = parsedNodeIdents
	}

	t, _, err = client.ACL().TokenUpdate(t, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to update token %s: %v", tok, err))
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
	synopsis = "Update an ACL token"
	help     = `
Usage: consul acl token update [options]

    This command will update a token. Some parts such as marking the token local
    cannot be changed.

    Update a token description and take the policies from the existing token:

        $ consul acl token update -accessor-id abcd -description "replication" -merge-policies

    Update all editable fields of the token:

        $ consul acl token update -accessor-id abcd \
                                  -description "replication" \
                                  -policy-name "token-replication" \
                                  -role-name "db-updater"
`
)
