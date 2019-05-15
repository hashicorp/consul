package roleupdate

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
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

	roleID        string
	name          string
	description   string
	policyIDs     []string
	policyNames   []string
	serviceIdents []string

	noMerge  bool
	showMeta bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that role metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(&c.roleID, "id", "", "The ID of the role to update. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple role IDs")
	c.flags.StringVar(&c.name, "name", "", "The role name.")
	c.flags.StringVar(&c.description, "description", "", "A description of the role")
	c.flags.Var((*flags.AppendSliceValue)(&c.policyIDs), "policy-id", "ID of a "+
		"policy to use for this role. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.policyNames), "policy-name", "Name of a "+
		"policy to use for this role. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.serviceIdents), "service-identity", "Name of a "+
		"service identity to use for this role. May be specified multiple times. Format is "+
		"the SERVICENAME or SERVICENAME:DATACENTER1,DATACENTER2,...")
	c.flags.BoolVar(&c.noMerge, "no-merge", false, "Do not merge the current role "+
		"information with what is provided to the command. Instead overwrite all fields "+
		"with the exception of the role ID which is immutable.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.roleID == "" {
		c.UI.Error(fmt.Sprintf("Cannot update a role without specifying the -id parameter"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	roleID, err := acl.GetRoleIDFromPartial(client, c.roleID)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining role ID: %v", err))
		return 1
	}

	parsedServiceIdents, err := acl.ExtractServiceIdentities(c.serviceIdents)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	// Read the current role in both cases so we can fail better if not found.
	currentRole, _, err := client.ACL().RoleRead(roleID, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error when retrieving current role: %v", err))
		return 1
	} else if currentRole == nil {
		c.UI.Error(fmt.Sprintf("Role not found with ID %q", roleID))
		return 1
	}

	var role *api.ACLRole
	if c.noMerge {
		role = &api.ACLRole{
			ID:                c.roleID,
			Name:              c.name,
			Description:       c.description,
			ServiceIdentities: parsedServiceIdents,
		}

		for _, policyName := range c.policyNames {
			// We could resolve names to IDs here but there isn't any reason
			// why its would be better than allowing the agent to do it.
			role.Policies = append(role.Policies, &api.ACLRolePolicyLink{Name: policyName})
		}

		for _, policyID := range c.policyIDs {
			policyID, err := acl.GetPolicyIDFromPartial(client, policyID)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error resolving policy ID %s: %v", policyID, err))
				return 1
			}
			role.Policies = append(role.Policies, &api.ACLRolePolicyLink{ID: policyID})
		}
	} else {
		role = currentRole

		if c.name != "" {
			role.Name = c.name
		}
		if c.description != "" {
			role.Description = c.description
		}

		for _, policyName := range c.policyNames {
			found := false
			for _, link := range role.Policies {
				if link.Name == policyName {
					found = true
					break
				}
			}

			if !found {
				// We could resolve names to IDs here but there isn't any
				// reason why its would be better than allowing the agent to do
				// it.
				role.Policies = append(role.Policies, &api.ACLRolePolicyLink{Name: policyName})
			}
		}

		for _, policyID := range c.policyIDs {
			policyID, err := acl.GetPolicyIDFromPartial(client, policyID)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error resolving policy ID %s: %v", policyID, err))
				return 1
			}
			found := false

			for _, link := range role.Policies {
				if link.ID == policyID {
					found = true
					break
				}
			}

			if !found {
				role.Policies = append(role.Policies, &api.ACLRolePolicyLink{ID: policyID})
			}
		}

		for _, svcid := range parsedServiceIdents {
			found := -1
			for i, link := range role.ServiceIdentities {
				if link.ServiceName == svcid.ServiceName {
					found = i
					break
				}
			}

			if found != -1 {
				role.ServiceIdentities[found] = svcid
			} else {
				role.ServiceIdentities = append(role.ServiceIdentities, svcid)
			}
		}
	}

	role, _, err = client.ACL().RoleUpdate(role, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error updating role %q: %v", roleID, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Role updated successfully"))
	acl.PrintRole(role, c.UI, c.showMeta)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Update an ACL role"
const help = `
Usage: consul acl role update [options]

  Updates a role. By default it will merge the role information with its
  current state so that you do not have to provide all parameters. This
  behavior can be disabled by passing -no-merge.

  Rename the role:

          $ consul acl role update -id abcd -name "better-name"

  Update all editable fields of the role:

          $ consul acl role update -id abcd \
                                   -name "better-name" \
                                   -description "replication" \
                                   -policy-name "token-replication" \
                                   -service-identity "web"
`
