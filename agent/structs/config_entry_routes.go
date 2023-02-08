package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// BoundRoute indicates a route that has parent gateways which
// can be accessed by calling the GetParents associated function.
type BoundRoute interface {
	ControlledConfigEntry
	GetParents() []ResourceReference
	GetProtocol() APIGatewayListenerProtocol
	GetServiceNames() []ServiceName
}

// HTTPRouteConfigEntry manages the configuration for a HTTP route
// with the given name.
type HTTPRouteConfigEntry struct {
	// Kind of the config entry. This will be set to structs.HTTPRoute.
	Kind string

	// Name is used to match the config entry with its associated set
	// of resources, which may include routers, splitters, filters, etc.
	Name string

	// Parents is a list of gateways that this route should be bound to
	Parents []ResourceReference
	// Rules are a list of HTTP-based routing rules that this route should
	// use for constructing a routing table.
	Rules []HTTPRouteRule
	// Hostnames are the hostnames for which this HTTPRoute should respond to requests.
	Hostnames []string

	Meta map[string]string `json:",omitempty"`
	// Status is the asynchronous reconciliation status which an HTTPRoute propagates to the user.
	Status             Status
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *HTTPRouteConfigEntry) GetServices() []HTTPService {
	targets := []HTTPService{}
	for _, rule := range e.Rules {
		for _, service := range rule.Services {
			targets = append(targets, service)
		}
	}
	return targets
}

func (e *HTTPRouteConfigEntry) GetServiceNames() []ServiceName {
	services := []ServiceName{}
	for _, service := range e.GetServices() {
		services = append(services, NewServiceName(service.Name, &service.EnterpriseMeta))
	}
	return services
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
	return e.Parents
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

// HTTPMatch specifies the criteria that should be
// used in determining whether or not a request should
// be routed to a given set of services.
type HTTPMatch struct {
	Headers []HTTPHeaderMatch
	Method  HTTPMatchMethod
	Path    HTTPPathMatch
	Query   []HTTPQueryMatch
}

// HTTPMatchMethod specifies which type of HTTP verb should
// be used for matching a given request.
type HTTPMatchMethod string

const (
	HTTPMatchMethodAll     HTTPMatchMethod = ""
	HTTPMatchMethodConnect HTTPMatchMethod = "CONNECT"
	HTTPMatchMethodDelete  HTTPMatchMethod = "DELETE"
	HTTPMatchMethodGet     HTTPMatchMethod = "GET"
	HTTPMatchMethodHead    HTTPMatchMethod = "HEAD"
	HTTPMatchMethodOptions HTTPMatchMethod = "OPTIONS"
	HTTPMatchMethodPatch   HTTPMatchMethod = "PATCH"
	HTTPMatchMethodPost    HTTPMatchMethod = "POST"
	HTTPMatchMethodPut     HTTPMatchMethod = "PUT"
	HTTPMatchMethodTrace   HTTPMatchMethod = "TRACE"
)

// HTTPHeaderMatchType specifies how header matching criteria
// should be applied to a request.
type HTTPHeaderMatchType string

const (
	HTTPHeaderMatchExact             HTTPHeaderMatchType = "exact"
	HTTPHeaderMatchPrefix            HTTPHeaderMatchType = "prefix"
	HTTPHeaderMatchPresent           HTTPHeaderMatchType = "present"
	HTTPHeaderMatchRegularExpression HTTPHeaderMatchType = "regex"
	HTTPHeaderMatchSuffix            HTTPHeaderMatchType = "suffix"
)

// HTTPHeaderMatch specifies how a match should be done
// on a request's headers.
type HTTPHeaderMatch struct {
	Match HTTPHeaderMatchType
	Name  string
	Value string
}

// HTTPPathMatchType specifies how path matching criteria
// should be applied to a request.
type HTTPPathMatchType string

const (
	HTTPPathMatchExact             HTTPPathMatchType = "exact"
	HTTPPathMatchPrefix            HTTPPathMatchType = "prefix"
	HTTPPathMatchRegularExpression HTTPPathMatchType = "regex"
)

// HTTPPathMatch specifies how a match should be done
// on a request's path.
type HTTPPathMatch struct {
	Match HTTPPathMatchType
	Value string
}

// HTTPQueryMatchType specifies how querys matching criteria
// should be applied to a request.
type HTTPQueryMatchType string

const (
	HTTPQueryMatchExact             HTTPQueryMatchType = "exact"
	HTTPQueryMatchPresent           HTTPQueryMatchType = "present"
	HTTPQueryMatchRegularExpression HTTPQueryMatchType = "regex"
)

// HTTPQueryMatch specifies how a match should be done
// on a request's query parameters.
type HTTPQueryMatch struct {
	Match HTTPQueryMatchType
	Name  string
	Value string
}

// HTTPFilters specifies a list of filters used to modify a request
// before it is routed to an upstream.
type HTTPFilters struct {
	Headers []HTTPHeaderFilter
}

// HTTPHeaderFilter specifies how HTTP headers should be modified.
type HTTPHeaderFilter struct {
	Add    map[string]string
	Remove []string
	Set    map[string]string
}

// HTTPRouteRule specifies the routing rules used to determine what upstream
// service an HTTP request is routed to.
type HTTPRouteRule struct {
	// Filters is a list of HTTP-based filters used to modify a request prior
	// to routing it to the upstream service
	Filters HTTPFilters
	// Matches specified the matching criteria used in the routing table. If a
	// request matches the given HTTPMatch configuration, then traffic is routed
	// to services specified in the Services field.
	Matches []HTTPMatch
	// Services is a list of HTTP-based services to route to if the request matches
	// the rules specified in the Matches field.
	Services []HTTPService
}

// HTTPService is a service reference for HTTP-based routing rules
type HTTPService struct {
	Name string
	// Weight is an arbitrary integer used in calculating how much
	// traffic should be sent to the given service.
	Weight int
	// Filters is a list of HTTP-based filters used to modify a request prior
	// to routing it to the upstream service
	Filters HTTPFilters

	acl.EnterpriseMeta
}

var _ ControlledConfigEntry = (*HTTPRouteConfigEntry)(nil)

func (e *HTTPRouteConfigEntry) GetStatus() Status {
	return e.Status
}

func (e *HTTPRouteConfigEntry) SetStatus(status Status) {
	e.Status = status
}

func (e *HTTPRouteConfigEntry) DefaultStatus() Status {
	return Status{}
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
	// Status is the asynchronous reconciliation status which a TCPRoute propagates to the user.
	Status             Status
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *TCPRouteConfigEntry) GetServices() []TCPService {
	return e.Services
}

func (e *TCPRouteConfigEntry) GetServiceNames() []ServiceName {
	services := []ServiceName{}
	for _, service := range e.Services {
		services = append(services, NewServiceName(service.Name, &service.EnterpriseMeta))
	}
	return services
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

var _ ControlledConfigEntry = (*TCPRouteConfigEntry)(nil)

func (e *TCPRouteConfigEntry) GetStatus() Status {
	return e.Status
}

func (e *TCPRouteConfigEntry) SetStatus(status Status) {
	e.Status = status
}

func (e *TCPRouteConfigEntry) DefaultStatus() Status {
	return Status{}
}

// TCPService is a service reference for a TCPRoute
type TCPService struct {
	Name string
	// Weight specifies the proportion of requests forwarded to the referenced service.
	// This is computed as weight/(sum of all weights in the list of services).
	Weight int

	acl.EnterpriseMeta
}
