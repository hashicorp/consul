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
	return a.resolveTokenAndDefaultMeta(id, nil, nil)
}

// resolveTokenAndDefaultMeta is used to resolve an ACL token secret to an
// acl.Authorizer and to default any enterprise specific metadata for the request.
// The defaulted metadata is then used to fill in an acl.AuthorizationContext.
func (a *Agent) resolveTokenAndDefaultMeta(id string, entMeta *structs.EnterpriseMeta, authzContext *acl.AuthorizerContext) (acl.Authorizer, error) {
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

	return a.delegate.ResolveTokenAndDefaultMeta(id, entMeta, authzContext)
}

// ResolveIdentityFromToken is used to resolve an ACL token secret a structs.ACLIdentity.
func (a *Agent) ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error) {
	// ACLs are disabled
	if !a.delegate.ACLsEnabled() {
		return false, nil, nil
	}

	// Disable ACLs if version 8 enforcement isn't enabled.
	if !a.config.ACLEnforceVersion8 {
		return false, nil, nil
	}

	return a.delegate.ResolveIdentityFromToken(token)
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
	authz, err := a.resolveToken(token)
	if err != nil {
		return err
	}

	return a.vetServiceRegisterWithAuthorizer(authz, service)
}

func (a *Agent) vetServiceRegisterWithAuthorizer(authz acl.Authorizer, service *structs.NodeService) error {
	if authz == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext
	service.FillAuthzContext(&authzContext)
	// Vet the service itself.
	if authz.ServiceWrite(service.Service, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	// Vet any service that might be getting overwritten.
	if existing := a.State.Service(service.CompoundServiceID()); existing != nil {
		existing.FillAuthzContext(&authzContext)
		if authz.ServiceWrite(existing.Service, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	// If the service is a proxy, ensure that it has write on the destination too
	// since it can be discovered as an instance of that service.
	if service.Kind == structs.ServiceKindConnectProxy {
		service.FillAuthzContext(&authzContext)
		if authz.ServiceWrite(service.Proxy.DestinationServiceName, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	return nil
}

// vetServiceUpdate makes sure the service update action is allowed by the given
// token.
func (a *Agent) vetServiceUpdate(token string, serviceID structs.ServiceID) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.resolveToken(token)
	if err != nil {
		return err
	}

	return a.vetServiceUpdateWithAuthorizer(authz, serviceID)
}

func (a *Agent) vetServiceUpdateWithAuthorizer(authz acl.Authorizer, serviceID structs.ServiceID) error {
	if authz == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext

	// Vet any changes based on the existing services's info.
	if existing := a.State.Service(serviceID); existing != nil {
		existing.FillAuthzContext(&authzContext)
		if authz.ServiceWrite(existing.Service, &authzContext) != acl.Allow {
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
	authz, err := a.resolveToken(token)
	if err != nil {
		return err
	}

	return a.vetCheckRegisterWithAuthorizer(authz, check)
}

func (a *Agent) vetCheckRegisterWithAuthorizer(authz acl.Authorizer, check *structs.HealthCheck) error {
	if authz == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext
	check.FillAuthzContext(&authzContext)
	// Vet the check itself.
	if len(check.ServiceName) > 0 {
		if authz.ServiceWrite(check.ServiceName, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	} else {
		if authz.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	// Vet any check that might be getting overwritten.
	if existing := a.State.Check(check.CompoundCheckID()); existing != nil {
		if len(existing.ServiceName) > 0 {
			if authz.ServiceWrite(existing.ServiceName, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		} else {
			if authz.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		}
	}

	return nil
}

// vetCheckUpdate makes sure that a check update is allowed by the given token.
func (a *Agent) vetCheckUpdate(token string, checkID structs.CheckID) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.resolveToken(token)
	if err != nil {
		return err
	}

	return a.vetCheckUpdateWithAuthorizer(authz, checkID)
}

func (a *Agent) vetCheckUpdateWithAuthorizer(authz acl.Authorizer, checkID structs.CheckID) error {
	if authz == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext
	checkID.FillAuthzContext(&authzContext)

	// Vet any changes based on the existing check's info.
	if existing := a.State.Check(checkID); existing != nil {
		if len(existing.ServiceName) > 0 {
			if authz.ServiceWrite(existing.ServiceName, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		} else {
			if authz.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
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

	_, tokenIdent, err := a.delegate.ResolveIdentityFromToken(token)
	if err != nil {
		a.logger.Printf("[DEBUG] agent: failed to acquire token identity, err=%v", err)
	}

	var authzContext acl.AuthorizerContext
	structs.DefaultEnterpriseMeta().FillAuthzContext(&authzContext)
	// Filter out members based on the node policy.
	m := *members
	for i := 0; i < len(m); i++ {
		node := m[i].Name
		if rule.NodeRead(node, &authzContext) == acl.Allow {
			continue
		}
		var accessorID string
		if tokenIdent != nil {
			accessorID = tokenIdent.ID()
		}
		a.logger.Printf("[DEBUG] agent: dropping node from result due to ACLs, node=%q accessorID=%v", node, accessorID)
		m = append(m[:i], m[i+1:]...)
		i--
	}
	*members = m
	return nil
}

// filterServices redacts services that the token doesn't have access to.
func (a *Agent) filterServices(token string, services *map[structs.ServiceID]*structs.NodeService) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.resolveToken(token)
	if err != nil {
		return err
	}

	return a.filterServicesWithAuthorizer(authz, services)
}

func (a *Agent) filterServicesWithAuthorizer(authz acl.Authorizer, services *map[structs.ServiceID]*structs.NodeService) error {
	if authz == nil {
		return nil
	}
	var authzContext acl.AuthorizerContext
	// Filter out services based on the service policy.
	for id, service := range *services {
		service.FillAuthzContext(&authzContext)
		if authz.ServiceRead(service.Service, &authzContext) == acl.Allow {
			continue
		}
		a.logger.Printf("[DEBUG] agent: dropping service from result due to ACLs, service=%q ", id.String())
		delete(*services, id)
	}
	return nil
}

// filterChecks redacts checks that the token doesn't have access to.
func (a *Agent) filterChecks(token string, checks *map[structs.CheckID]*structs.HealthCheck) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.resolveToken(token)
	if err != nil {
		return err
	}

	return a.filterChecksWithAuthorizer(authz, checks)
}

func (a *Agent) filterChecksWithAuthorizer(authz acl.Authorizer, checks *map[structs.CheckID]*structs.HealthCheck) error {
	if authz == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext
	// Filter out checks based on the node or service policy.
	for id, check := range *checks {
		if len(check.ServiceName) > 0 {
			check.FillAuthzContext(&authzContext)
			if authz.ServiceRead(check.ServiceName, &authzContext) == acl.Allow {
				continue
			}
		} else {
			structs.DefaultEnterpriseMeta().FillAuthzContext(&authzContext)
			if authz.NodeRead(a.config.NodeName, &authzContext) == acl.Allow {
				continue
			}
		}
		a.logger.Printf("[DEBUG] agent: dropping check from result due to ACLs, check=%q ", id.String())
		delete(*checks, id)
	}
	return nil
}
