package agent

import (
	"fmt"

	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// aclAccessorID is used to convert an ACLToken's secretID to its accessorID for non-
// critical purposes, such as logging. Therefore we interpret all errors as empty-string
// so we can safely log it without handling non-critical errors at the usage site.
func (a *Agent) aclAccessorID(secretID string) string {
	ident, err := a.delegate.ResolveTokenToIdentity(secretID)
	if acl.IsErrNotFound(err) {
		return ""
	}
	if err != nil {
		a.logger.Debug("non-critical error resolving acl token accessor for logging", "error", err)
		return ""
	}
	if ident == nil {
		return ""
	}
	return ident.ID()
}

// vetServiceRegister makes sure the service registration action is allowed by
// the given token.
func (a *Agent) vetServiceRegister(token string, service *structs.NodeService) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
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
		serviceName := service.CompoundServiceName()
		return acl.PermissionDenied("Missing service:write on %s", serviceName.String())
	}

	// Vet any service that might be getting overwritten.
	if existing := a.State.Service(service.CompoundServiceID()); existing != nil {
		existing.FillAuthzContext(&authzContext)
		if authz.ServiceWrite(existing.Service, &authzContext) != acl.Allow {
			serviceName := service.CompoundServiceName()
			return acl.PermissionDenied("Missing service:write on %s", serviceName.String())
		}
	}

	// If the service is a proxy, ensure that it has write on the destination too
	// since it can be discovered as an instance of that service.
	if service.Kind == structs.ServiceKindConnectProxy {
		service.FillAuthzContext(&authzContext)
		if authz.ServiceWrite(service.Proxy.DestinationServiceName, &authzContext) != acl.Allow {
			return acl.PermissionDenied("Missing service:write on %s", service.Proxy.DestinationServiceName)
		}
	}

	return nil
}

// vetServiceUpdate makes sure the service update action is allowed by the given
// token.
func (a *Agent) vetServiceUpdate(token string, serviceID structs.ServiceID) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
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
			serviceName := existing.CompoundServiceName()
			return acl.PermissionDenied("Missing service:write on %s", serviceName.String())
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
	authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
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
			return acl.PermissionDenied("Missing service:write on %v", structs.ServiceIDString(check.ServiceName, &check.EnterpriseMeta))
		}
	} else {
		if authz.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
			return acl.PermissionDenied("Missing node:write on %s", a.config.NodeName)
		}
	}

	// Vet any check that might be getting overwritten.
	if existing := a.State.Check(check.CompoundCheckID()); existing != nil {
		if len(existing.ServiceName) > 0 {
			if authz.ServiceWrite(existing.ServiceName, &authzContext) != acl.Allow {
				return acl.PermissionDenied("Missing service:write on %s", structs.ServiceIDString(existing.ServiceName, &existing.EnterpriseMeta))
			}
		} else {
			if authz.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
				return acl.PermissionDenied("Missing node:write on %s", a.config.NodeName)
			}
		}
	}

	return nil
}

// vetCheckUpdate makes sure that a check update is allowed by the given token.
func (a *Agent) vetCheckUpdate(token string, checkID structs.CheckID) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
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
				return acl.PermissionDenied("Missing service:write on %s", existing.ServiceName)
			}
		} else {
			if authz.NodeWrite(a.config.NodeName, &authzContext) != acl.Allow {
				return acl.PermissionDenied("Missing node:write on %s", a.config.NodeName)
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
	rule, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
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
		accessorID := a.aclAccessorID(token)
		a.logger.Debug("dropping node from result due to ACLs", "node", node, "accessorID", accessorID)
		m = append(m[:i], m[i+1:]...)
		i--
	}
	*members = m
	return nil
}

// filterServices redacts services that the token doesn't have access to.
func (a *Agent) filterServices(token string, services *map[structs.ServiceID]*structs.NodeService) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
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
		a.logger.Debug("dropping service from result due to ACLs", "service", id.String())
		delete(*services, id)
	}
	return nil
}

// filterChecks redacts checks that the token doesn't have access to.
func (a *Agent) filterChecks(token string, checks *map[structs.CheckID]*structs.HealthCheck) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
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
		a.logger.Debug("dropping check from result due to ACLs", "check", id.String())
		delete(*checks, id)
	}
	return nil
}
