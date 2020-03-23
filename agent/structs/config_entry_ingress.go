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
	Namespace     string
	ServiceSubset string
}

func (i IngressService) NamespaceOrDefault() string {
	if i.Namespace == "" {
		return IntentionDefaultNamespace
	}
	return i.Namespace
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
			if s.Namespace == WildcardSpecifier {
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
