package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
)

// IngressGatewayConfigEntry manages the configuration for an ingress service
// with the given name.
type IngressGatewayConfigEntry struct {
	Kind string
	Name string

	Listeners []IngressListener

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

type IngressListener struct {
	Port     int
	Protocol string

	Services []IngressService
}

type IngressService struct {
	Name          string
	ServiceSubset string

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

func (e *IngressGatewayConfigEntry) GetKind() string {
	return IngressGateway
}

func (e *IngressGatewayConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *IngressGatewayConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = IngressGateway
	for _, listener := range e.Listeners {
		listener.Protocol = strings.ToLower(listener.Protocol)
	}

	e.EnterpriseMeta.Normalize()

	return nil
}

func (e *IngressGatewayConfigEntry) Validate() error {
	declaredPorts := make(map[int]bool)
	for _, listener := range e.Listeners {
		if _, ok := declaredPorts[listener.Port]; ok {
			return fmt.Errorf("port %d declared on two listeners", listener.Port)
		}
		declaredPorts[listener.Port] = true

		for _, s := range listener.Services {
			if s.Name == "*" && listener.Protocol != "http" {
				return fmt.Errorf("Wildcard service name is only valid for protocol = 'http' (listener on port %d)", listener.Port)
			}
			if s.NamespaceOrDefault() == WildcardSpecifier {
				return fmt.Errorf("Wildcard namespace is not supported for ingress services (listener on port %d)", listener.Port)
			}
			if s.Name == "" {
				return fmt.Errorf("Service name cannot be blank (listener on port %d)", listener.Port)
			}
		}

		// Validate that http features aren't being used with tcp or another non-supported protocol.
		if listener.Protocol != "http" {
			if len(listener.Services) > 1 {
				return fmt.Errorf("Multiple services per listener are only supported for protocol = 'http'")
			}

			if len(listener.Services) == 0 {
				return fmt.Errorf("No service declared for listener with port %d", listener.Port)
			}
		}
	}

	return nil
}

func (e *IngressGatewayConfigEntry) CanRead(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.OperatorRead(&authzContext) == acl.Allow
}

func (e *IngressGatewayConfigEntry) CanWrite(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.OperatorWrite(&authzContext) == acl.Allow
}

func (e *IngressGatewayConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *IngressGatewayConfigEntry) GetEnterpriseMeta() *EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

// TerminatingGatewayConfigEntry manages the configuration for a terminating service
// with the given name.
type TerminatingGatewayConfigEntry struct {
	Kind     string
	Name     string
	Services []LinkedService

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

// A LinkedService is a service represented by a terminating gateway
type LinkedService struct {
	// Name is the name of the service, as defined in Consul's catalog
	Name string `json:",omitempty"`

	// CAFile is the optional path to a CA certificate to use for TLS connections
	// from the gateway to the linked service
	CAFile string `json:",omitempty"`

	// CertFile is the optional path to a client certificate to use for TLS connections
	// from the gateway to the linked service
	CertFile string `json:",omitempty"`

	// KeyFile is the optional path to a private key to use for TLS connections
	// from the gateway to the linked service
	KeyFile string `json:",omitempty"`

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

func (e *TerminatingGatewayConfigEntry) GetKind() string {
	return TerminatingGateway
}

func (e *TerminatingGatewayConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *TerminatingGatewayConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = TerminatingGateway

	for _, svc := range e.Services {
		svc.EnterpriseMeta.Normalize()
	}
	e.EnterpriseMeta.Normalize()

	return nil
}

func (e *TerminatingGatewayConfigEntry) Validate() error {
	seen := make(map[string]map[string]bool)

	for _, svc := range e.Services {
		if svc.Name == "" {
			return fmt.Errorf("Service name cannot be blank.")
		}

		ns := svc.NamespaceOrDefault()
		if ns == WildcardSpecifier {
			return fmt.Errorf("Wildcard namespace is not supported for terminating gateway services")
		}
		if _, ok := seen[ns]; !ok {
			seen[ns] = make(map[string]bool)
		}
		if ok := seen[ns][svc.Name]; ok {
			return fmt.Errorf("Service %q was specified more than once within a namespace", svc.Name)
		}
		seen[ns][svc.Name] = true
	}
	return nil
}

func (e *TerminatingGatewayConfigEntry) CanRead(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)

	return authz.OperatorRead(&authzContext) == acl.Allow
}

func (e *TerminatingGatewayConfigEntry) CanWrite(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)

	return authz.OperatorWrite(&authzContext) == acl.Allow
}

func (e *TerminatingGatewayConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *TerminatingGatewayConfigEntry) GetEnterpriseMeta() *EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}
