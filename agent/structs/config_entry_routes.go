package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// BoundRoute indicates a route that has parent gateways which
// can be accessed by calling the GetParents associated function.
type BoundRoute interface {
	ConfigEntry
	GetParents() []ResourceReference
	GetProtocol() APIGatewayListenerProtocol
}

// HTTPRouteConfigEntry manages the configuration for a HTTP route
// with the given name.
type HTTPRouteConfigEntry struct {
	// Kind of the config entry. This will be set to structs.HTTPRoute.
	Kind string

	// Name is used to match the config entry with its associated set
	// of resources, which may include routers, splitters, filters, etc.
	Name string

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *HTTPRouteConfigEntry) GetKind() string {
	return HTTPRoute
}

func (e *HTTPRouteConfigEntry) GetName() string {
	if e == nil {
		return ""
	}
	return e.Name
}

func (e *HTTPRouteConfigEntry) GetParents() []ResourceReference {
	if e == nil {
		return []ResourceReference{}
	}
	// TODO HTTP Route should have "parents". Andrew will implement this in his work.
	return []ResourceReference{}
}

func (e *HTTPRouteConfigEntry) GetProtocol() APIGatewayListenerProtocol {
	return ListenerProtocolHTTP
}

func (e *HTTPRouteConfigEntry) Normalize() error {
	return nil
}

func (e *HTTPRouteConfigEntry) Validate() error {
	return nil
}

func (e *HTTPRouteConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *HTTPRouteConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *HTTPRouteConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *HTTPRouteConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}
	return &e.EnterpriseMeta
}

func (e *HTTPRouteConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}
	return &e.RaftIndex
}

// TCPRouteConfigEntry manages the configuration for a TCP route
// with the given name.
type TCPRouteConfigEntry struct {
	// Kind of the config entry. This will be set to structs.TCPRoute.
	Kind string

	// Name is used to match the config entry with its associated set
	// of resources.
	Name string

	// Parents is a list of gateways that this route should be bound to
	Parents []ResourceReference

	// Services is a list of TCP-based services that this should route to.
	// Currently, this must specify at maximum one service.
	Services []TCPService

	Meta map[string]string `json:",omitempty"`
	// Status is the asynchronous status which a TCPRoute propagates to the user.
	Status             Status
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *TCPRouteConfigEntry) GetKind() string {
	return TCPRoute
}

func (e *TCPRouteConfigEntry) GetName() string {
	if e == nil {
		return ""
	}
	return e.Name
}

func (e *TCPRouteConfigEntry) GetParents() []ResourceReference {
	if e == nil {
		return []ResourceReference{}
	}
	return e.Parents
}

func (e *TCPRouteConfigEntry) GetProtocol() APIGatewayListenerProtocol {
	return ListenerProtocolTCP
}

func (e *TCPRouteConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *TCPRouteConfigEntry) Normalize() error {
	for i, parent := range e.Parents {
		if parent.Kind == "" {
			parent.Kind = APIGateway
			e.Parents[i] = parent
		}
	}
	return nil
}

func (e *TCPRouteConfigEntry) Validate() error {
	validParentKinds := map[string]bool{
		APIGateway: true,
	}

	if len(e.Services) > 1 {
		return fmt.Errorf("tcp-route currently only supports one service")
	}
	for _, parent := range e.Parents {
		if !validParentKinds[parent.Kind] {
			return fmt.Errorf("unsupported parent kind: %q, must be 'api-gateway'", parent.Kind)
		}
	}
	return nil
}

func (e *TCPRouteConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *TCPRouteConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *TCPRouteConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}
	return &e.RaftIndex
}

func (e *TCPRouteConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}
	return &e.EnterpriseMeta
}

// TCPService is a service reference for a TCPRoute
type TCPService struct {
	Name string
	// Weight specifies the proportion of requests forwarded to the referenced service.
	// This is computed as weight/(sum of all weights in the list of services).
	Weight int

	acl.EnterpriseMeta
}
