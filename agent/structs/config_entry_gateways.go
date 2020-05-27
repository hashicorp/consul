package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/miekg/dns"
)

// IngressGatewayConfigEntry manages the configuration for an ingress service
// with the given name.
type IngressGatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.IngressGateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	// TLS holds the TLS configuration for this gateway.
	TLS GatewayTLSConfig

	// Listeners declares what ports the ingress gateway should listen on, and
	// what services to associated to those ports.
	Listeners []IngressListener

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

type IngressListener struct {
	// Port declares the port on which the ingress gateway should listen for traffic.
	Port int

	// Protocol declares what type of traffic this listener is expected to
	// receive. Depending on the protocol, a listener might support multiplexing
	// services over a single port, or additional discovery chain features. The
	// current supported values are: (tcp | http).
	Protocol string

	// Services declares the set of services to which the listener forwards
	// traffic.
	//
	// For "tcp" protocol listeners, only a single service is allowed.
	// For "http" listeners, multiple services can be declared.
	Services []IngressService
}

type IngressService struct {
	// Name declares the service to which traffic should be forwarded.
	//
	// This can either be a specific service, or the wildcard specifier,
	// "*". If the wildcard specifier is provided, the listener must be of "http"
	// protocol and means that the listener will forward traffic to all services.
	//
	// A name can be specified on multiple listeners, and will be exposed on both
	// of the listeners
	Name string

	// Hosts is a list of hostnames which should be associated to this service on
	// the defined listener. Only allowed on layer 7 protocols, this will be used
	// to route traffic to the service by matching the Host header of the HTTP
	// request.
	//
	// If a host is provided for a service that also has a wildcard specifier
	// defined, the host will override the wildcard-specifier-provided
	// "<service-name>.*" domain for that listener.
	//
	// This cannot be specified when using the wildcard specifier, "*", or when
	// using a "tcp" listener.
	Hosts []string

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

type GatewayTLSConfig struct {
	// Indicates that TLS should be enabled for this gateway service
	Enabled bool
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
	e.EnterpriseMeta.Normalize()

	for i, listener := range e.Listeners {
		if listener.Protocol == "" {
			listener.Protocol = "tcp"
		}

		listener.Protocol = strings.ToLower(listener.Protocol)
		for i := range listener.Services {
			listener.Services[i].EnterpriseMeta.Merge(&e.EnterpriseMeta)
			listener.Services[i].EnterpriseMeta.Normalize()
		}

		// Make sure to set the item back into the array, since we are not using
		// pointers to structs
		e.Listeners[i] = listener
	}

	return nil
}

func (e *IngressGatewayConfigEntry) Validate() error {
	validProtocols := map[string]bool{
		"http": true,
		"tcp":  true,
	}
	declaredPorts := make(map[int]bool)

	for _, listener := range e.Listeners {
		if _, ok := declaredPorts[listener.Port]; ok {
			return fmt.Errorf("port %d declared on two listeners", listener.Port)
		}
		declaredPorts[listener.Port] = true

		if _, ok := validProtocols[listener.Protocol]; !ok {
			return fmt.Errorf("Protocol must be either 'http' or 'tcp', '%s' is an unsupported protocol.", listener.Protocol)
		}

		if len(listener.Services) == 0 {
			return fmt.Errorf("No service declared for listener with port %d", listener.Port)
		}

		// Validate that http features aren't being used with tcp or another non-supported protocol.
		if listener.Protocol != "http" && len(listener.Services) > 1 {
			return fmt.Errorf("Multiple services per listener are only supported for protocol = 'http' (listener on port %d)",
				listener.Port)
		}

		declaredHosts := make(map[string]bool)
		for _, s := range listener.Services {
			if listener.Protocol == "tcp" {
				if s.Name == WildcardSpecifier {
					return fmt.Errorf("Wildcard service name is only valid for protocol = 'http' (listener on port %d)", listener.Port)
				}
				if len(s.Hosts) != 0 {
					return fmt.Errorf("Associating hosts to a service is not supported for the %s protocol (listener on port %d)", listener.Protocol, listener.Port)
				}
			}
			if s.Name == "" {
				return fmt.Errorf("Service name cannot be blank (listener on port %d)", listener.Port)
			}
			if s.Name == WildcardSpecifier && len(s.Hosts) != 0 {
				return fmt.Errorf("Associating hosts to a wildcard service is not supported (listener on port %d)", listener.Port)
			}
			if s.NamespaceOrDefault() == WildcardSpecifier {
				return fmt.Errorf("Wildcard namespace is not supported for ingress services (listener on port %d)", listener.Port)
			}

			for _, h := range s.Hosts {
				if declaredHosts[h] {
					return fmt.Errorf("Hosts must be unique within a specific listener (listener on port %d)", listener.Port)
				}
				declaredHosts[h] = true
				if err := validateHost(h); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateHost(host string) error {
	wildcardPrefix := "*."
	if _, ok := dns.IsDomainName(host); !ok {
		return fmt.Errorf("Host %q must be a valid DNS hostname", host)
	}

	if strings.ContainsRune(strings.TrimPrefix(host, wildcardPrefix), '*') {
		return fmt.Errorf("Host %q is not valid, a wildcard specifier is only allowed as the leftmost label", host)
	}

	if host == "*" {
		return fmt.Errorf("Host '*' is not allowed, wildcards can only be used as a prefix/suffix")
	}

	return nil
}

func (e *IngressGatewayConfigEntry) CanRead(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ServiceRead(e.Name, &authzContext) == acl.Allow
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

func (s *IngressService) ToServiceID() ServiceID {
	return NewServiceID(s.Name, &s.EnterpriseMeta)
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

	// SNI is the optional name to specify during the TLS handshake with a linked service
	SNI string `json:",omitempty"`

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
	e.EnterpriseMeta.Normalize()

	for i := range e.Services {
		e.Services[i].EnterpriseMeta.Merge(&e.EnterpriseMeta)
		e.Services[i].EnterpriseMeta.Normalize()
	}

	return nil
}

func (e *TerminatingGatewayConfigEntry) Validate() error {
	seen := make(map[ServiceID]bool)

	for _, svc := range e.Services {
		if svc.Name == "" {
			return fmt.Errorf("Service name cannot be blank.")
		}

		ns := svc.NamespaceOrDefault()
		if ns == WildcardSpecifier {
			return fmt.Errorf("Wildcard namespace is not supported for terminating gateway services")
		}

		// Check for duplicates within the entry
		cid := NewServiceID(svc.Name, &svc.EnterpriseMeta)
		if ok := seen[cid]; ok {
			return fmt.Errorf("Service %q was specified more than once within a namespace", cid.String())
		}
		seen[cid] = true

		// If either client cert config file was specified then the CA file, client cert, and key file must be specified
		// Specifying only a CAFile is allowed for one-way TLS
		if (svc.CertFile != "" || svc.KeyFile != "") &&
			!(svc.CAFile != "" && svc.CertFile != "" && svc.KeyFile != "") {

			return fmt.Errorf("Service %q must have a CertFile, CAFile, and KeyFile specified for TLS origination", svc.Name)
		}
	}
	return nil
}

func (e *TerminatingGatewayConfigEntry) CanRead(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)

	return authz.ServiceRead(e.Name, &authzContext) == acl.Allow
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

// GatewayService is used to associate gateways with their linked services.
type GatewayService struct {
	Gateway      ServiceID
	Service      ServiceID
	GatewayKind  ServiceKind
	Port         int
	Protocol     string
	Hosts        []string
	CAFile       string
	CertFile     string
	KeyFile      string
	SNI          string
	FromWildcard bool
	RaftIndex
}

type GatewayServices []*GatewayService

func (g *GatewayService) IsSame(o *GatewayService) bool {
	return g.Gateway.Matches(&o.Gateway) &&
		g.Service.Matches(&o.Service) &&
		g.GatewayKind == o.GatewayKind &&
		g.Port == o.Port &&
		g.Protocol == o.Protocol &&
		stringslice.Equal(g.Hosts, o.Hosts) &&
		g.CAFile == o.CAFile &&
		g.CertFile == o.CertFile &&
		g.KeyFile == o.KeyFile &&
		g.SNI == o.SNI &&
		g.FromWildcard == o.FromWildcard
}

func (g *GatewayService) Clone() *GatewayService {
	return &GatewayService{
		Gateway:     g.Gateway,
		Service:     g.Service,
		GatewayKind: g.GatewayKind,
		Port:        g.Port,
		Protocol:    g.Protocol,
		// See https://github.com/go101/go101/wiki/How-to-efficiently-clone-a-slice%3F
		Hosts:        append(g.Hosts[:0:0], g.Hosts...),
		CAFile:       g.CAFile,
		CertFile:     g.CertFile,
		KeyFile:      g.KeyFile,
		SNI:          g.SNI,
		FromWildcard: g.FromWildcard,
		RaftIndex:    g.RaftIndex,
	}
}
