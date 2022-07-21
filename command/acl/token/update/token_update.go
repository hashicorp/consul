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

	tokenID            string
	policyIDs          []string
	policyNames        []string
	roleIDs            []string
	roleNames          []string
	serviceIdents      []string
	nodeIdents         []string
	description        string
	mergePolicies      bool
	mergeRoles         bool
	mergeServiceIdents bool
	mergeNodeIdents    bool
	showMeta           bool
	upgradeLegacy      bool
	format             string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that token metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.BoolVar(&c.mergePolicies, "merge-policies", false, "Merge the new policies "+
		"with the existing policies")
	c.flags.BoolVar(&c.mergeRoles, "merge-roles", false, "Merge the new roles "+
		"with the existing roles")
	c.flags.BoolVar(&c.mergeServiceIdents, "merge-service-identities", false, "Merge the new service identities "+
		"with the existing service identities")
	c.flags.BoolVar(&c.mergeNodeIdents, "merge-node-identities", false, "Merge the new node identities "+
		"with the existing node identities")
	c.flags.StringVar(&c.tokenID, "id", "", "The Accessor ID of the token to update. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple token Accessor IDs")
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
	c.flags.BoolVar(&c.upgradeLegacy, "upgrade-legacy", false, "Add new polices "+
		"to a legacy token replacing all existing rules. This will cause the legacy "+
		"token to behave exactly like a new token but keep the same Secret.\n"+
		"WARNING: you must ensure that the new policy or policies specified grant "+
		"equivalent or appropriate access for the existing clients using this token.")
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

	if c.tokenID == "" {
		c.UI.Error(fmt.Sprintf("Cannot update a token without specifying the -id parameter"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	tokenID, err := acl.GetTokenIDFromPartial(client, c.tokenID)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining token ID: %v", err))
		return 1
	}

	t, _, err := client.ACL().TokenRead(tokenID, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error when retrieving current token: %v", err))
		return 1
	}

	if c.upgradeLegacy {
		if t.Rules == "" {
			// This is just for convenience it should actually be harmless to allow it
			// to go through anyway.
			c.UI.Error(fmt.Sprintf("Can't use -upgrade-legacy on a non-legacy token"))
			return 1
		}
		// Reset the rules to nothing forcing this to be updated as a non-legacy
		// token but with same secret.
		t.Rules = ""
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

	parsedServiceIdents, err := acl.ExtractServiceIdentities(c.serviceIdents)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	parsedNodeIdents, err := acl.ExtractNodeIdentities(c.nodeIdents)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if c.mergePolicies {
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
		t.Policies = nil

		for _, policyName := range c.policyNames {
			// We could resolve names to IDs here but there isn't any reason why its would be better
			// than allowing the agent to do it.
			t.Policies = append(t.Policies, &api.ACLTokenPolicyLink{Name: policyName})
		}

		for _, policyID := range c.policyIDs {
			policyID, err := acl.GetPolicyIDFromPartial(client, policyID)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error resolving policy ID %s: %v", policyID, err))
				return 1
			}
			t.Policies = append(t.Policies, &api.ACLTokenPolicyLink{ID: policyID})
		}
	}

	if c.mergeRoles {
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
		t.Roles = nil

		for _, roleName := range c.roleNames {
			// We could resolve names to IDs here but there isn't any reason why its would be better
			// than allowing the agent to do it.
			t.Roles = append(t.Roles, &api.ACLTokenRoleLink{Name: roleName})
		}

		for _, roleID := range c.roleIDs {
			roleID, err := acl.GetRoleIDFromPartial(client, roleID)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error resolving role ID %s: %v", roleID, err))
				return 1
			}
			t.Roles = append(t.Roles, &api.ACLTokenRoleLink{ID: roleID})
		}
	}

	if c.mergeServiceIdents {
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

	if c.mergeNodeIdents {
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
		c.UI.Error(fmt.Sprintf("Failed to update token %s: %v", tokenID, err))
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

        $ consul acl token update -id abcd -description "replication" -merge-policies

    Update all editable fields of the token:

        $ consul acl token update -id abcd \
                                  -description "replication" \
                                  -policy-name "token-replication" \
                                  -role-name "db-updater"
`
)
