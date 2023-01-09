package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

// aclAccessorID is used to convert an ACLToken's secretID to its accessorID for non-
// critical purposes, such as logging. Therefore we interpret all errors as empty-string
// so we can safely log it without handling non-critical errors at the usage site.
func (a *Agent) aclAccessorID(secretID string) string {
	ident, err := a.delegate.ResolveTokenAndDefaultMeta(secretID, nil, nil)
	if acl.IsErrNotFound(err) {
		return ""
	}
	if err != nil {
		a.logger.Debug("non-critical error resolving acl token accessor for logging", "error", err)
		return ""
	}
	return ident.AccessorID()
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
	var authzContext acl.AuthorizerContext

	// Vet the service itself.
	service.FillAuthzContext(&authzContext)
	if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(service.Service, &authzContext); err != nil {
		return err
	}

	// Vet any service that might be getting overwritten.
	if existing := a.State.Service(service.CompoundServiceID()); existing != nil {
		existing.FillAuthzContext(&authzContext)
		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(existing.Service, &authzContext); err != nil {
			return err
		}
	}

	// If the service is a proxy, ensure that it has write on the destination too
	// since it can be discovered as an instance of that service.
	if service.Kind == structs.ServiceKindConnectProxy {
		service.FillAuthzContext(&authzContext)
		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(service.Proxy.DestinationServiceName, &authzContext); err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) vetServiceUpdateWithAuthorizer(authz acl.Authorizer, serviceID structs.ServiceID) error {
	var authzContext acl.AuthorizerContext

	// Vet any changes based on the existing services's info.
	if existing := a.State.Service(serviceID); existing != nil {
		existing.FillAuthzContext(&authzContext)
		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(existing.Service, &authzContext); err != nil {
			return err

		}
	} else {
		// Take care if modifying this error message.
		// agent/local/state.go's deleteService assumes the Catalog.Deregister RPC call
		// will include "Unknown service"in the error if deregistration fails due to a
		// service with that ID not existing.
		return HTTPError{
			StatusCode: http.StatusNotFound,
			Reason:     fmt.Sprintf("Unknown service ID %q. Ensure that the service ID is passed, not the service name.", serviceID),
		}
	}

	return nil
}

func (a *Agent) vetCheckRegisterWithAuthorizer(authz acl.Authorizer, check *structs.HealthCheck) error {
	var authzContext acl.AuthorizerContext
	check.FillAuthzContext(&authzContext)

	// Vet the check itself.
	if len(check.ServiceName) > 0 {
		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(check.ServiceName, &authzContext); err != nil {
			return err
		}
	} else {
		// N.B. Should this authzContext be derived from a.AgentEnterpriseMeta()
		if err := authz.ToAllowAuthorizer().NodeWriteAllowed(a.config.NodeName, &authzContext); err != nil {
			return err
		}
	}

	// Vet any check that might be getting overwritten.
	if existing := a.State.Check(check.CompoundCheckID()); existing != nil {
		if len(existing.ServiceName) > 0 {
			// N.B. Should this authzContext be derived from existing.EnterpriseMeta?
			if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(existing.ServiceName, &authzContext); err != nil {
				return err
			}
		} else {
			// N.B. Should this authzContext be derived from a.AgentEnterpriseMeta()
			if err := authz.ToAllowAuthorizer().NodeWriteAllowed(a.config.NodeName, &authzContext); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *Agent) vetCheckUpdateWithAuthorizer(authz acl.Authorizer, checkID structs.CheckID) error {
	var authzContext acl.AuthorizerContext
	checkID.FillAuthzContext(&authzContext)

	// Vet any changes based on the existing check's info.
	if existing := a.State.Check(checkID); existing != nil {
		if len(existing.ServiceName) > 0 {
			if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(existing.ServiceName, &authzContext); err != nil {
				return err
			}
		} else {
			if err := authz.ToAllowAuthorizer().NodeWriteAllowed(a.config.NodeName, &authzContext); err != nil {
				return err
			}
		}
	} else {
		return HTTPError{
			StatusCode: http.StatusNotFound,
			Reason:     fmt.Sprintf("Unknown check ID %q. Ensure that the check ID is passed, not the check name.", checkID.String()),
		}
	}

	return nil
}

// filterMembers redacts members that the token doesn't have access to.
func (a *Agent) filterMembers(token string, members *[]serf.Member) error {
	// Resolve the token and bail if ACLs aren't enabled.
	authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return err
	}

	var authzContext acl.AuthorizerContext
	// Filter out members based on the node policy.
	m := *members
	for i := 0; i < len(m); i++ {
		node := m[i].Name
		serfMemberFillAuthzContext(&m[i], &authzContext)
		if authz.NodeRead(node, &authzContext) == acl.Allow {
			continue
		}
		accessorID := authz.AccessorID()
		a.logger.Debug("dropping node from result due to ACLs", "node", node, "accessorID", acl.AliasIfAnonymousToken(accessorID))
		m = append(m[:i], m[i+1:]...)
		i--
	}
	*members = m
	return nil
}

func (a *Agent) filterServicesWithAuthorizer(authz acl.Authorizer, services map[string]*api.AgentService) error {
	var authzContext acl.AuthorizerContext
	// Filter out services based on the service policy.
	for id, service := range services {
		agentServiceFillAuthzContext(service, &authzContext)
		if authz.ServiceRead(service.Service, &authzContext) == acl.Allow {
			continue
		}
		a.logger.Debug("dropping service from result due to ACLs", "service", id)
		delete(services, id)
	}
	return nil
}

func (a *Agent) filterChecksWithAuthorizer(authz acl.Authorizer, checks map[types.CheckID]*structs.HealthCheck) error {
	var authzContext acl.AuthorizerContext
	// Filter out checks based on the node or service policy.
	for id, check := range checks {
		check.FillAuthzContext(&authzContext)
		if len(check.ServiceName) > 0 {
			if authz.ServiceRead(check.ServiceName, &authzContext) == acl.Allow {
				continue
			}
		} else {
			if authz.NodeRead(a.config.NodeName, &authzContext) == acl.Allow {
				continue
			}
		}
		a.logger.Debug("dropping check from result due to ACLs", "check", id)
		delete(checks, id)
	}
	return nil
}
