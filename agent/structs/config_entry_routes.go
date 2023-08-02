// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"

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

func (e *HTTPRouteConfigEntry) GetKind() string                        { return HTTPRoute }
func (e *HTTPRouteConfigEntry) GetName() string                        { return e.Name }
func (e *HTTPRouteConfigEntry) GetMeta() map[string]string             { return e.Meta }
func (e *HTTPRouteConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta { return &e.EnterpriseMeta }
func (e *HTTPRouteConfigEntry) GetRaftIndex() *RaftIndex               { return &e.RaftIndex }

var _ ControlledConfigEntry = (*HTTPRouteConfigEntry)(nil)

func (e *HTTPRouteConfigEntry) GetStatus() Status       { return e.Status }
func (e *HTTPRouteConfigEntry) SetStatus(status Status) { e.Status = status }
func (e *HTTPRouteConfigEntry) DefaultStatus() Status   { return Status{} }

var _ BoundRoute = (*HTTPRouteConfigEntry)(nil)

func (e *HTTPRouteConfigEntry) GetParents() []ResourceReference         { return e.Parents }
func (e *HTTPRouteConfigEntry) GetProtocol() APIGatewayListenerProtocol { return ListenerProtocolHTTP }

func (e *HTTPRouteConfigEntry) GetServiceNames() []ServiceName {
	services := []ServiceName{}
	for _, service := range e.GetServices() {
		services = append(services, NewServiceName(service.Name, &service.EnterpriseMeta))
	}
	return services
}

func (e *HTTPRouteConfigEntry) GetServices() []HTTPService {
	targets := []HTTPService{}
	for _, rule := range e.Rules {
		targets = append(targets, rule.Services...)
	}
	return targets
}

func (e *HTTPRouteConfigEntry) Normalize() error {
	for i, parent := range e.Parents {
		if parent.Kind == "" {
			parent.Kind = APIGateway
		}
		parent.EnterpriseMeta.Merge(e.GetEnterpriseMeta())
		parent.EnterpriseMeta.Normalize()
		e.Parents[i] = parent
	}

	for i, rule := range e.Rules {
		for j, match := range rule.Matches {
			rule.Matches[j] = normalizeHTTPMatch(match)
		}

		for j, service := range rule.Services {
			rule.Services[j] = e.normalizeHTTPService(service)
		}
		e.Rules[i] = rule
	}

	return nil
}

func (e *HTTPRouteConfigEntry) normalizeHTTPService(service HTTPService) HTTPService {
	service.EnterpriseMeta.Merge(e.GetEnterpriseMeta())
	service.EnterpriseMeta.Normalize()
	if service.Weight <= 0 {
		service.Weight = 1
	}
	return service
}

func normalizeHTTPMatch(match HTTPMatch) HTTPMatch {
	method := string(match.Method)
	method = strings.ToUpper(method)
	match.Method = HTTPMatchMethod(method)

	pathMatch := match.Path.Match
	if string(pathMatch) == "" {
		match.Path.Match = HTTPPathMatchPrefix
		match.Path.Value = "/"
	}

	return match
}

func (e *HTTPRouteConfigEntry) Validate() error {
	for _, host := range e.Hostnames {
		// validate that each host referenced in a valid dns name and has
		// no wildcards in it
		if _, ok := dns.IsDomainName(host); !ok {
			return fmt.Errorf("host %q must be a valid DNS hostname", host)
		}

		if strings.ContainsRune(host, '*') {
			return fmt.Errorf("host %q must not be a wildcard", host)
		}
	}

	validParentKinds := map[string]bool{
		APIGateway: true,
	}

	uniques := make(map[ResourceReference]struct{}, len(e.Parents))

	for _, parent := range e.Parents {
		if !validParentKinds[parent.Kind] {
			return fmt.Errorf("unsupported parent kind: %q, must be 'api-gateway'", parent.Kind)
		}

		if _, ok := uniques[parent]; ok {
			return errors.New("route parents must be unique")
		}
		uniques[parent] = struct{}{}
	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	for i, rule := range e.Rules {
		if err := validateRule(rule); err != nil {
			return fmt.Errorf("Rule[%d], %w", i, err)
		}
	}

	return nil
}

func validateRule(rule HTTPRouteRule) error {
	if err := validateFilters(rule.Filters); err != nil {
		return err
	}

	for i, match := range rule.Matches {
		if err := validateMatch(match); err != nil {
			return fmt.Errorf("Match[%d], %w", i, err)
		}
	}

	for i, service := range rule.Services {
		if err := validateHTTPService(service); err != nil {
			return fmt.Errorf("Service[%d], %w", i, err)
		}
	}

	return nil
}

func validateMatch(match HTTPMatch) error {
	if match.Method != HTTPMatchMethodAll {
		if !isValidHTTPMethod(string(match.Method)) {
			return fmt.Errorf("Method contains an invalid method %q", match.Method)
		}
	}

	for i, query := range match.Query {
		if err := validateHTTPQueryMatch(query); err != nil {
			return fmt.Errorf("Query[%d], %w", i, err)
		}
	}

	for i, header := range match.Headers {
		if err := validateHTTPHeaderMatch(header); err != nil {
			return fmt.Errorf("Headers[%d], %w", i, err)
		}
	}

	if err := validateHTTPPathMatch(match.Path); err != nil {
		return fmt.Errorf("Path, %w", err)
	}

	return nil
}

func validateHTTPService(service HTTPService) error {
	return validateFilters(service.Filters)
}

func validateFilters(filter HTTPFilters) error {
	for i, header := range filter.Headers {
		if err := validateHeaderFilter(header); err != nil {
			return fmt.Errorf("HTTPFilters, Headers[%d], %w", i, err)
		}
	}

	if err := validateURLRewrite(filter.URLRewrite); err != nil {
		return fmt.Errorf("HTTPFilters, URLRewrite, %w", err)
	}

	return nil
}

func validateURLRewrite(rewrite *URLRewrite) error {
	// TODO: we don't really have validation of the actual params
	// passed as "PrefixRewrite" in our discoverychain config
	// entries, figure out if we should have something here
	return nil
}

func validateHeaderFilter(filter HTTPHeaderFilter) error {
	// TODO: we don't really have validation of the values
	// passed as header modifiers in our current discoverychain
	// config entries, figure out if we need to
	return nil
}

func validateHTTPQueryMatch(query HTTPQueryMatch) error {
	if query.Name == "" {
		return fmt.Errorf("missing required Name field")
	}

	switch query.Match {
	case HTTPQueryMatchExact,
		HTTPQueryMatchPresent,
		HTTPQueryMatchRegularExpression:
		return nil
	default:
		return fmt.Errorf("match type should be one of present, exact, or regex")
	}
}

func validateHTTPHeaderMatch(header HTTPHeaderMatch) error {
	if header.Name == "" {
		return fmt.Errorf("missing required Name field")
	}

	switch header.Match {
	case HTTPHeaderMatchExact,
		HTTPHeaderMatchPrefix,
		HTTPHeaderMatchRegularExpression,
		HTTPHeaderMatchSuffix,
		HTTPHeaderMatchPresent:
		return nil
	default:
		return fmt.Errorf("match type should be one of present, exact, prefix, suffix, or regex")
	}
}

func validateHTTPPathMatch(path HTTPPathMatch) error {
	switch path.Match {
	case HTTPPathMatchExact,
		HTTPPathMatchPrefix:
		if !strings.HasPrefix(path.Value, "/") {
			return fmt.Errorf("%s type match doesn't start with '/': %q", path.Match, path.Value)
		}
		fallthrough
	case HTTPPathMatchRegularExpression:
		return nil
	default:
		return fmt.Errorf("match type should be one of exact, prefix, or regex")
	}
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

func (e *HTTPRouteConfigEntry) FilteredHostnames(listenerHostname string) []string {
	if len(e.Hostnames) == 0 {
		// we have no hostnames specified here, so treat it like a wildcard
		return []string{listenerHostname}
	}

	wildcardHostname := strings.ContainsRune(listenerHostname, '*') || listenerHostname == "*"
	listenerHostname = strings.TrimPrefix(strings.TrimPrefix(listenerHostname, "*"), ".")

	hostnames := []string{}
	for _, hostname := range e.Hostnames {
		if wildcardHostname {
			if strings.HasSuffix(hostname, listenerHostname) {
				hostnames = append(hostnames, hostname)
			}
			continue
		}

		if hostname == listenerHostname {
			hostnames = append(hostnames, hostname)
		}
	}
	return hostnames
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
	Headers       []HTTPHeaderFilter
	URLRewrite    *URLRewrite
	RetryFilter   *RetryFilter
	TimeoutFilter *TimeoutFilter
}

// HTTPHeaderFilter specifies how HTTP headers should be modified.
type HTTPHeaderFilter struct {
	Add    map[string]string
	Remove []string
	Set    map[string]string
}

type URLRewrite struct {
	Path string
}

type RetryFilter struct {
	NumRetries            *uint32
	RetryOn               []string
	RetryOnStatusCodes    []uint32
	RetryOnConnectFailure *bool
}

type TimeoutFilter struct {
	RequestTimeout time.Duration
	IdleTimeout    time.Duration
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

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

func (s HTTPService) ServiceName() ServiceName {
	return NewServiceName(s.Name, &s.EnterpriseMeta)
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

func (e *TCPRouteConfigEntry) GetKind() string                        { return TCPRoute }
func (e *TCPRouteConfigEntry) GetName() string                        { return e.Name }
func (e *TCPRouteConfigEntry) GetMeta() map[string]string             { return e.Meta }
func (e *TCPRouteConfigEntry) GetRaftIndex() *RaftIndex               { return &e.RaftIndex }
func (e *TCPRouteConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta { return &e.EnterpriseMeta }

var _ ControlledConfigEntry = (*TCPRouteConfigEntry)(nil)

func (e *TCPRouteConfigEntry) GetStatus() Status       { return e.Status }
func (e *TCPRouteConfigEntry) SetStatus(status Status) { e.Status = status }
func (e *TCPRouteConfigEntry) DefaultStatus() Status   { return Status{} }

var _ BoundRoute = (*TCPRouteConfigEntry)(nil)

func (e *TCPRouteConfigEntry) GetParents() []ResourceReference         { return e.Parents }
func (e *TCPRouteConfigEntry) GetProtocol() APIGatewayListenerProtocol { return ListenerProtocolTCP }

func (e *TCPRouteConfigEntry) GetServiceNames() []ServiceName {
	services := []ServiceName{}
	for _, service := range e.Services {
		services = append(services, NewServiceName(service.Name, &service.EnterpriseMeta))
	}
	return services
}

func (e *TCPRouteConfigEntry) GetServices() []TCPService { return e.Services }

func (e *TCPRouteConfigEntry) Normalize() error {
	for i, parent := range e.Parents {
		if parent.Kind == "" {
			parent.Kind = APIGateway
		}
		parent.EnterpriseMeta.Merge(e.GetEnterpriseMeta())
		parent.EnterpriseMeta.Normalize()
		e.Parents[i] = parent
	}

	for i, service := range e.Services {
		service.EnterpriseMeta.Merge(e.GetEnterpriseMeta())
		service.EnterpriseMeta.Normalize()
		e.Services[i] = service
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

	uniques := make(map[ResourceReference]struct{}, len(e.Parents))

	for _, parent := range e.Parents {
		if !validParentKinds[parent.Kind] {
			return fmt.Errorf("unsupported parent kind: %q, must be 'api-gateway'", parent.Kind)
		}

		if _, ok := uniques[parent]; ok {
			return errors.New("route parents must be unique")
		}
		uniques[parent] = struct{}{}

	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
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

// TCPService is a service reference for a TCPRoute
type TCPService struct {
	Name string

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

func (s TCPService) ServiceName() ServiceName {
	return NewServiceName(s.Name, &s.EnterpriseMeta)
}
