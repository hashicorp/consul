package agent

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/serf/serf"
)

// resolveToken is the primary interface used by ACL-checkers in the agent
// endpoints, which is the one place where we do some ACL enforcement on
// clients. Some of the enforcement is normative (e.g. self and monitor)
// and some is informative (e.g. catalog and health).
func (a *Agent) resolveToken(id string) (acl.Authorizer, error) {
	// ACLs are disabled
	if !a.delegate.ACLsEnabled() {
		return nil, nil
	}

	// Disable ACLs if version 8 enforcement isn't enabled.
	if !a.config.ACLEnforceVersion8 {
		return nil, nil
	}

	if acl.RootAuthorizer(id) != nil {
		return nil, acl.ErrRootDenied
	}

	if a.tokens.IsAgentMasterToken(id) {
		return a.aclMasterAuthorizer, nil
	}
	return a.delegate.ResolveToken(id)
}

func (a *Agent) initializeACLs() error {
	// Build a policy for the agent master token.
	// The builtin agent master policy allows reading any node information
	// and allows writes to the agent with the node name of the running agent
	// only. This used to allow a prefix match on agent names but that seems
	// entirely unnecessary so it is now using an exact match.
	policy := &acl.Policy{
		PolicyRules: acl.PolicyRules{
			Agents: []*acl.AgentRule{
				&acl.AgentRule{
					Node:   a.config.NodeName,
					Policy: acl.PolicyWrite,
				},
			},
			NodePrefixes: []*acl.NodeRule{
				&acl.NodeRule{
					Name:   "",
					Policy: acl.PolicyRead,
				},
			},
		},
	}
	master, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		return err
	}
	a.aclMasterAuthorizer = master
	return nil
}

// vetServiceRegister makes sure the service registration action is allowed by
// the given token.
func (a *Agent) vetServiceRegister(token string, service *structs.NodeService) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	var authzContext acl.EnterpriseAuthorizerContext
	service.FillAuthzContext(&authzContext)
	// Vet the service itself.
	if rule.ServiceWrite(service.Service, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	// Vet any service that might be getting overwritten.
	if existing := a.State.Service(service.CompoundServiceID()); existing != nil {
		existing.FillAuthzContext(&authzContext)
		if rule.ServiceWrite(existing.Service, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	// If the service is a proxy, ensure that it has write on the destination too
	// since it can be discovered as an instance of that service.
	if service.Kind == structs.ServiceKindConnectProxy {
		service.FillAuthzContext(&authzContext)
		if rule.ServiceWrite(service.Proxy.DestinationServiceName, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	return nil
}

// vetServiceUpdate makes sure the service update action is allowed by the given
// token.
func (a *Agent) vetServiceUpdate(token string, serviceID structs.ServiceID) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	var authzContext acl.EnterpriseAuthorizerContext

	// Vet any changes based on the existing services's info.
	if existing := a.State.Service(serviceID); existing != nil {
		existing.FillAuthzContext(&authzContext)
		if rule.ServiceWrite(existing.Service, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	} else {
		return fmt.Errorf("Unknown service %q", serviceID)
	}

	return nil
}

// vetCheckRegister makes sure the check registration action is allowed by the
// given token.
func (a *Agent) vetCheckRegister(token string, check *structs.HealthCheck) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	var authzContext acl.EnterpriseAuthorizerContext
	check.FillAuthzContext(&authzContext)
	// Vet the check itself.
	if len(check.ServiceName) > 0 {
		if rule.ServiceWrite(check.ServiceName, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	} else {
		if rule.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	// Vet any check that might be getting overwritten.
	if existing := a.State.Check(check.CompoundCheckID()); existing != nil {
		if len(existing.ServiceName) > 0 {
			if rule.ServiceWrite(existing.ServiceName, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		} else {
			if rule.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		}
	}

	return nil
}

// vetCheckUpdate makes sure that a check update is allowed by the given token.
func (a *Agent) vetCheckUpdate(token string, checkID structs.CheckID) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	var authzContext acl.EnterpriseAuthorizerContext
	checkID.FillAuthzContext(&authzContext)

	// Vet any changes based on the existing check's info.
	if existing := a.State.Check(checkID); existing != nil {
		if len(existing.ServiceName) > 0 {
			if rule.ServiceWrite(existing.ServiceName, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		} else {
			if rule.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		}
	} else {
		return fmt.Errorf("Unknown check %q", checkID.String())
	}

	return nil
}

// filterMembers redacts members that the token doesn't have access to.
func (a *Agent) filterMembers(token string, members *[]serf.Member) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	var authzContext acl.EnterpriseAuthorizerContext
	structs.DefaultEnterpriseMeta().FillAuthzContext(&authzContext)
	// Filter out members based on the node policy.
	m := *members
	for i := 0; i < len(m); i++ {
		node := m[i].Name
		if rule.NodeRead(node, &authzContext) == acl.Allow {
			continue
		}
		a.logger.Printf("[DEBUG] agent: dropping node %q from result due to ACLs", node)
		m = append(m[:i], m[i+1:]...)
		i--
	}
	*members = m
	return nil
}

// filterServices redacts services that the token doesn't have access to.
func (a *Agent) filterServices(token string, services *map[structs.ServiceID]*structs.NodeService) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	var authzContext acl.EnterpriseAuthorizerContext
	// Filter out services based on the service policy.
	for id, service := range *services {
		service.FillAuthzContext(&authzContext)
		if rule.ServiceRead(service.Service, &authzContext) == acl.Allow {
			continue
		}
		a.logger.Printf("[DEBUG] agent: dropping service %q from result due to ACLs", id.String())
		delete(*services, id)
	}
	return nil
}

// filterChecks redacts checks that the token doesn't have access to.
func (a *Agent) filterChecks(token string, checks *map[structs.CheckID]*structs.HealthCheck) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	var authzContext acl.EnterpriseAuthorizerContext
	// Filter out checks based on the node or service policy.
	for id, check := range *checks {
		if len(check.ServiceName) > 0 {
			check.FillAuthzContext(&authzContext)
			if rule.ServiceRead(check.ServiceName, &authzContext) == acl.Allow {
				continue
			}
		} else {
			structs.DefaultEnterpriseMeta().FillAuthzContext(&authzContext)
			if rule.NodeRead(a.config.NodeName, &authzContext) == acl.Allow {
				continue
			}
		}
		a.logger.Printf("[DEBUG] agent: dropping check %q from result due to ACLs", id.String())
		delete(*checks, id)
	}
	return nil
}
