package agent

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
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
		Agents: []*acl.AgentPolicy{
			&acl.AgentPolicy{
				Node:   a.config.NodeName,
				Policy: acl.PolicyWrite,
			},
		},
		NodePrefixes: []*acl.NodePolicy{
			&acl.NodePolicy{
				Name:   "",
				Policy: acl.PolicyRead,
			},
		},
	}
	master, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
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

	// Vet the service itself.
	if !rule.ServiceWrite(service.Service, nil) {
		return acl.ErrPermissionDenied
	}

	// Vet any service that might be getting overwritten.
	services := a.State.Services()
	if existing, ok := services[service.ID]; ok {
		if !rule.ServiceWrite(existing.Service, nil) {
			return acl.ErrPermissionDenied
		}
	}

	// If the service is a proxy, ensure that it has write on the destination too
	// since it can be discovered as an instance of that service.
	if service.Kind == structs.ServiceKindConnectProxy {
		if !rule.ServiceWrite(service.Proxy.DestinationServiceName, nil) {
			return acl.ErrPermissionDenied
		}
	}

	return nil
}

// vetServiceUpdate makes sure the service update action is allowed by the given
// token.
func (a *Agent) vetServiceUpdate(token string, serviceID string) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	// Vet any changes based on the existing services's info.
	services := a.State.Services()
	if existing, ok := services[serviceID]; ok {
		if !rule.ServiceWrite(existing.Service, nil) {
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

	// Vet the check itself.
	if len(check.ServiceName) > 0 {
		if !rule.ServiceWrite(check.ServiceName, nil) {
			return acl.ErrPermissionDenied
		}
	} else {
		if !rule.NodeWrite(a.config.NodeName, nil) {
			return acl.ErrPermissionDenied
		}
	}

	// Vet any check that might be getting overwritten.
	checks := a.State.Checks()
	if existing, ok := checks[check.CheckID]; ok {
		if len(existing.ServiceName) > 0 {
			if !rule.ServiceWrite(existing.ServiceName, nil) {
				return acl.ErrPermissionDenied
			}
		} else {
			if !rule.NodeWrite(a.config.NodeName, nil) {
				return acl.ErrPermissionDenied
			}
		}
	}

	return nil
}

// vetCheckUpdate makes sure that a check update is allowed by the given token.
func (a *Agent) vetCheckUpdate(token string, checkID types.CheckID) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	// Vet any changes based on the existing check's info.
	checks := a.State.Checks()
	if existing, ok := checks[checkID]; ok {
		if len(existing.ServiceName) > 0 {
			if !rule.ServiceWrite(existing.ServiceName, nil) {
				return acl.ErrPermissionDenied
			}
		} else {
			if !rule.NodeWrite(a.config.NodeName, nil) {
				return acl.ErrPermissionDenied
			}
		}
	} else {
		return fmt.Errorf("Unknown check %q", checkID)
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

	// Filter out members based on the node policy.
	m := *members
	for i := 0; i < len(m); i++ {
		node := m[i].Name
		if rule.NodeRead(node) {
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
func (a *Agent) filterServices(token string, services *map[string]*structs.NodeService) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	// Filter out services based on the service policy.
	for id, service := range *services {
		if rule.ServiceRead(service.Service) {
			continue
		}
		a.logger.Printf("[DEBUG] agent: dropping service %q from result due to ACLs", id)
		delete(*services, id)
	}
	return nil
}

// filterChecks redacts checks that the token doesn't have access to.
func (a *Agent) filterChecks(token string, checks *map[types.CheckID]*structs.HealthCheck) error {
	// Resolve the token and bail if ACLs aren't enabled.
	rule, err := a.resolveToken(token)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	// Filter out checks based on the node or service policy.
	for id, check := range *checks {
		if len(check.ServiceName) > 0 {
			if rule.ServiceRead(check.ServiceName) {
				continue
			}
		} else {
			if rule.NodeRead(a.config.NodeName) {
				continue
			}
		}
		a.logger.Printf("[DEBUG] agent: dropping check %q from result due to ACLs", id)
		delete(*checks, id)
	}
	return nil
}
