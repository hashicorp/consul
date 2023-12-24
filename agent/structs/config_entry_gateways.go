// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/miekg/dns"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/types"
)

const (
	wildcardPrefix = "*."
)

// IngressGatewayConfigEntry manages the configuration for an ingress service
// with the given name.
type IngressGatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.IngressGateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	// TLS holds the TLS configuration for this gateway. It would be nicer if it
	// were a pointer so it could be omitempty when read back in JSON but that
	// would be a breaking API change now as we currently always return it.
	TLS GatewayTLSConfig

	// Listeners declares what ports the ingress gateway should listen on, and
	// what services to associated to those ports.
	Listeners []IngressListener

	// Defaults contains default configuration for all upstream service instances
	Defaults *IngressServiceConfig `json:",omitempty"`

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

func (e *IngressGatewayConfigEntry) SetHash(h uint64) {
	e.Hash = h
}

func (e *IngressGatewayConfigEntry) GetHash() uint64 {
	return e.Hash
}

type IngressServiceConfig struct {
	MaxConnections        uint32
	MaxPendingRequests    uint32
	MaxConcurrentRequests uint32

	// PassiveHealthCheck configuration determines how upstream proxy instances will
	// be monitored for removal from the load balancing pool.
	PassiveHealthCheck *PassiveHealthCheck `json:",omitempty" alias:"passive_health_check"`
}

type IngressListener struct {
	// Port declares the port on which the ingress gateway should listen for traffic.
	Port int

	// Protocol declares what type of traffic this listener is expected to
	// receive. Depending on the protocol, a listener might support multiplexing
	// services over a single port, or additional discovery chain features. The
	// current supported values are: (tcp | http | http2 | grpc).
	Protocol string

	// TLS config for this listener.
	TLS *GatewayTLSConfig `json:",omitempty"`

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

	// TLS configuration overrides for this service. At least one entry must exist
	// in Hosts to use set and the Listener must also have a default Cert loaded
	// from SDS.
	TLS *GatewayServiceTLSConfig `json:",omitempty"`

	// Allow HTTP header manipulation to be configured.
	RequestHeaders  *HTTPHeaderModifiers `json:",omitempty" alias:"request_headers"`
	ResponseHeaders *HTTPHeaderModifiers `json:",omitempty" alias:"response_headers"`

	MaxConnections        uint32 `json:",omitempty" alias:"max_connections"`
	MaxPendingRequests    uint32 `json:",omitempty" alias:"max_pending_requests"`
	MaxConcurrentRequests uint32 `json:",omitempty" alias:"max_concurrent_requests"`

	// PassiveHealthCheck configuration determines how upstream proxy instances will
	// be monitored for removal from the load balancing pool.
	PassiveHealthCheck *PassiveHealthCheck `json:",omitempty" alias:"passive_health_check"`

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

type GatewayTLSConfig struct {
	// Indicates that TLS should be enabled for this gateway or listener
	Enabled bool

	// SDS allows configuring TLS certificate from an SDS service.
	SDS *GatewayTLSSDSConfig `json:",omitempty"`

	TLSMinVersion types.TLSVersion `json:",omitempty" alias:"tls_min_version"`
	TLSMaxVersion types.TLSVersion `json:",omitempty" alias:"tls_max_version"`

	// Define a subset of cipher suites to restrict
	// Only applicable to connections negotiated via TLS 1.2 or earlier
	CipherSuites []types.TLSCipherSuite `json:",omitempty" alias:"cipher_suites"`
}

type GatewayServiceTLSConfig struct {
	// Note no Enabled field here since it doesn't make sense to disable TLS on
	// one host on a TLS-configured listener.

	// SDS allows configuring TLS certificate from an SDS service.
	SDS *GatewayTLSSDSConfig `json:",omitempty"`
}

type GatewayTLSSDSConfig struct {
	ClusterName  string `json:",omitempty" alias:"cluster_name"`
	CertResource string `json:",omitempty" alias:"cert_resource"`
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

func (e *IngressGatewayConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
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

	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h

	return nil
}

// validateServiceSDS validates the SDS config for a specific service on a
// specific listener. It checks inherited config properties from listener and
// gateway level and ensures they are valid all the way down. If called on
// several services some of these checks will be duplicated but that isn't a big
// deal and it's significantly easier to reason about and read if this is in one
// place rather than threaded through the multi-level loop in Validate with
// other checks.
func (e *IngressGatewayConfigEntry) validateServiceSDS(lis IngressListener, svc IngressService) error {
	// First work out if there is valid gateway-level SDS config
	gwSDSClusterSet := false
	gwSDSCertSet := false
	if e.TLS.SDS != nil {
		// Gateway level SDS config must set ClusterName if it specifies a default
		// certificate. Just a clustername is OK though if certs are specified
		// per-listener.
		if e.TLS.SDS.ClusterName == "" && e.TLS.SDS.CertResource != "" {
			return fmt.Errorf("TLS.SDS.ClusterName is required if CertResource is set")
		}
		// Note we rely on the fact that ClusterName must be non-empty if any SDS
		// properties are defined at this level (as validated above)  in validation
		// below that uses this variable. If that changes we will need to change the
		// code below too.
		gwSDSClusterSet = (e.TLS.SDS.ClusterName != "")
		gwSDSCertSet = (e.TLS.SDS.CertResource != "")
	}

	// Validate listener-level SDS config.
	lisSDSCertSet := false
	lisSDSClusterSet := false
	if lis.TLS != nil && lis.TLS.SDS != nil {
		lisSDSCertSet = (lis.TLS.SDS.CertResource != "")
		lisSDSClusterSet = (lis.TLS.SDS.ClusterName != "")
	}

	// If SDS was setup at gw level but without a default CertResource, the
	// listener MUST set a CertResource.
	if gwSDSClusterSet && !gwSDSCertSet && !lisSDSCertSet {
		return fmt.Errorf("TLS.SDS.CertResource is required if ClusterName is set for gateway (listener on port %d)", lis.Port)
	}

	// If listener set a cluster name then it requires a cert resource too.
	if lisSDSClusterSet && !lisSDSCertSet {
		return fmt.Errorf("TLS.SDS.CertResource is required if ClusterName is set for listener (listener on port %d)", lis.Port)
	}

	// If a listener-level cert is given, we need a cluster from at least one
	// level.
	if lisSDSCertSet && !lisSDSClusterSet && !gwSDSClusterSet {
		return fmt.Errorf("TLS.SDS.ClusterName is required if CertResource is set (listener on port %d)", lis.Port)
	}

	// Validate service-level SDS config
	svcSDSSet := (svc.TLS != nil && svc.TLS.SDS != nil && svc.TLS.SDS.CertResource != "")

	// Service SDS is only supported with Host names because we need to bind
	// specific service certs to one or more SNI hostnames.
	if svcSDSSet && len(svc.Hosts) < 1 {
		sid := NewServiceID(svc.Name, &svc.EnterpriseMeta)
		return fmt.Errorf("A service specifying TLS.SDS.CertResource must have at least one item in Hosts (service %q on listener on port %d)",
			sid.String(), lis.Port)
	}
	// If this service specified a certificate, there must be an SDS cluster set
	// at one of the three levels.
	if svcSDSSet && svc.TLS.SDS.ClusterName == "" && !lisSDSClusterSet && !gwSDSClusterSet {
		sid := NewServiceID(svc.Name, &svc.EnterpriseMeta)
		return fmt.Errorf("TLS.SDS.ClusterName is required if CertResource is set (service %q on listener on port %d)",
			sid.String(), lis.Port)
	}
	return nil
}

func validateGatewayTLSConfig(tlsCfg GatewayTLSConfig) error {
	return validateTLSConfig(tlsCfg.TLSMinVersion, tlsCfg.TLSMaxVersion, tlsCfg.CipherSuites)
}

func (e *IngressGatewayConfigEntry) Validate() error {
	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	if err := validateGatewayTLSConfig(e.TLS); err != nil {
		return err
	}

	validProtocols := map[string]bool{
		"tcp":   true,
		"http":  true,
		"http2": true,
		"grpc":  true,
	}
	declaredPorts := make(map[int]bool)

	for _, listener := range e.Listeners {
		if _, ok := declaredPorts[listener.Port]; ok {
			return fmt.Errorf("port %d declared on two listeners", listener.Port)
		}
		declaredPorts[listener.Port] = true

		if _, ok := validProtocols[listener.Protocol]; !ok {
			return fmt.Errorf("protocol must be 'tcp', 'http', 'http2', or 'grpc'. '%s' is an unsupported protocol", listener.Protocol)
		}

		if len(listener.Services) == 0 {
			return fmt.Errorf("No service declared for listener with port %d", listener.Port)
		}

		// Validate that http features aren't being used with tcp or another non-supported protocol.
		if !IsProtocolHTTPLike(listener.Protocol) && len(listener.Services) > 1 {
			return fmt.Errorf("Multiple services per listener are only supported for L7 protocols, 'http', 'grpc' and 'http2' (listener on port %d)",
				listener.Port)
		}

		if listener.TLS != nil {
			if err := validateGatewayTLSConfig(*listener.TLS); err != nil {
				return err
			}
		}

		declaredHosts := make(map[string]bool)
		serviceNames := make(map[ServiceID]struct{})
		for _, s := range listener.Services {
			sn := NewServiceName(s.Name, &s.EnterpriseMeta)
			if err := s.RequestHeaders.Validate(listener.Protocol); err != nil {
				return fmt.Errorf("request headers %s (service %q on listener on port %d)", err, sn.String(), listener.Port)
			}
			if err := s.ResponseHeaders.Validate(listener.Protocol); err != nil {
				return fmt.Errorf("response headers %s (service %q on listener on port %d)", err, sn.String(), listener.Port)
			}

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
			sid := NewServiceID(s.Name, &s.EnterpriseMeta)
			if _, ok := serviceNames[sid]; ok {
				return fmt.Errorf("Service %s cannot be added multiple times (listener on port %d)", sid, listener.Port)
			}
			serviceNames[sid] = struct{}{}

			// Validate SDS configuration for this service
			if err := e.validateServiceSDS(listener, s); err != nil {
				return err
			}

			for _, h := range s.Hosts {
				if declaredHosts[h] {
					return fmt.Errorf("Hosts must be unique within a specific listener (listener on port %d)", listener.Port)
				}
				declaredHosts[h] = true
				if err := validateHost(e.TLS.Enabled, h); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateHost(tlsEnabled bool, host string) error {
	// Special case '*' so that non-TLS ingress gateways can use it. This allows
	// an easy demo/testing experience.
	if host == "*" {
		if tlsEnabled {
			return fmt.Errorf("Host '*' is not allowed when TLS is enabled, all hosts must be valid DNS records to add as a DNSSAN")
		}
		return nil
	}

	if _, ok := dns.IsDomainName(host); !ok {
		return fmt.Errorf("Host %q must be a valid DNS hostname", host)
	}

	if strings.ContainsRune(strings.TrimPrefix(host, wildcardPrefix), '*') {
		return fmt.Errorf("Host %q is not valid, a wildcard specifier is only allowed as the leftmost label", host)
	}

	return nil
}

// ListRelatedServices implements discoveryChainConfigEntry
//
// For ingress-gateway config entries this only finds services that are
// explicitly linked in the ingress-gateway config entry. Wildcards will not
// expand to all services.
//
// This function is used during discovery chain graph validation to prevent
// erroneous sets of config entries from being created. Wildcard ingress
// filters out sets with protocol mismatch elsewhere so it isn't an issue here
// that needs fixing.
func (e *IngressGatewayConfigEntry) ListRelatedServices() []ServiceID {
	found := make(map[ServiceID]struct{})

	for _, listener := range e.Listeners {
		for _, service := range listener.Services {
			if service.Name == WildcardSpecifier {
				continue
			}
			svcID := NewServiceID(service.Name, &service.EnterpriseMeta)
			found[svcID] = struct{}{}
		}
	}

	if len(found) == 0 {
		return nil
	}

	out := make([]ServiceID, 0, len(found))
	for svc := range found {
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EnterpriseMeta.LessThan(&out[j].EnterpriseMeta) ||
			out[i].ID < out[j].ID
	})
	return out
}

func (e *IngressGatewayConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceReadAllowed(e.Name, &authzContext)
}

func (e *IngressGatewayConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *IngressGatewayConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *IngressGatewayConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (s *IngressService) ToServiceName() ServiceName {
	return NewServiceName(s.Name, &s.EnterpriseMeta)
}

// TerminatingGatewayConfigEntry manages the configuration for a terminating service
// with the given name.
type TerminatingGatewayConfigEntry struct {
	Kind     string
	Name     string
	Services []LinkedService

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

func (e *TerminatingGatewayConfigEntry) SetHash(h uint64) {
	e.Hash = h
}

func (e *TerminatingGatewayConfigEntry) GetHash() uint64 {
	return e.Hash
}

// A LinkedService is a service represented by a terminating gateway
type LinkedService struct {
	// Name is the name of the service, as defined in Consul's catalog
	Name string `json:",omitempty"`

	// CAFile is the optional path to a CA certificate to use for TLS connections
	// from the gateway to the linked service
	CAFile string `json:",omitempty" alias:"ca_file"`

	// CertFile is the optional path to a client certificate to use for TLS connections
	// from the gateway to the linked service
	CertFile string `json:",omitempty" alias:"cert_file"`

	// KeyFile is the optional path to a private key to use for TLS connections
	// from the gateway to the linked service
	KeyFile string `json:",omitempty" alias:"key_file"`

	// SNI is the optional name to specify during the TLS handshake with a linked service
	SNI string `json:",omitempty"`

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
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

func (e *TerminatingGatewayConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
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

	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h
	return nil
}

func (e *TerminatingGatewayConfigEntry) Validate() error {
	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	seen := make(map[ServiceID]bool)

	for _, svc := range e.Services {
		if svc.Name == "" {
			return fmt.Errorf("Service name cannot be blank.")
		}

		ns := svc.NamespaceOrDefault()
		if ns == WildcardSpecifier {
			return fmt.Errorf("Wildcard namespace is not supported for terminating gateway services")
		}

		cid := NewServiceID(svc.Name, &svc.EnterpriseMeta)

		if err := validateInnerEnterpriseMeta(&svc.EnterpriseMeta, &e.EnterpriseMeta); err != nil {
			return fmt.Errorf("service %q: %w", cid, err)
		}

		// Check for duplicates within the entry
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

func (e *TerminatingGatewayConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceReadAllowed(e.Name, &authzContext)
}

func (e *TerminatingGatewayConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *TerminatingGatewayConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *TerminatingGatewayConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (e *TerminatingGatewayConfigEntry) Warnings() []string {
	if e == nil {
		return nil
	}

	warnings := make([]string, 0)
	for _, svc := range e.Services {
		if (svc.CAFile != "" || svc.CertFile != "" || svc.KeyFile != "") && svc.SNI == "" {
			warning := fmt.Sprintf("TLS is configured but SNI is not set for service %q. Enabling SNI is strongly recommended when using TLS.", svc.Name)
			warnings = append(warnings, warning)
		}
	}

	return warnings
}

type GatewayServiceKind string

const (
	GatewayServiceKindUnknown     GatewayServiceKind = ""
	GatewayServiceKindDestination GatewayServiceKind = "destination"
	GatewayServiceKindService     GatewayServiceKind = "service"
)

// GatewayService is used to associate gateways with their linked services.
type GatewayService struct {
	Gateway      ServiceName
	Service      ServiceName
	GatewayKind  ServiceKind
	Port         int                `json:",omitempty"`
	Protocol     string             `json:",omitempty"`
	Hosts        []string           `json:",omitempty"`
	CAFile       string             `json:",omitempty"`
	CertFile     string             `json:",omitempty"`
	KeyFile      string             `json:",omitempty"`
	SNI          string             `json:",omitempty"`
	FromWildcard bool               `json:",omitempty"`
	ServiceKind  GatewayServiceKind `json:",omitempty"`
	RaftIndex
}

type GatewayServices []*GatewayService

func (g *GatewayService) Addresses(defaultHosts []string) []string {
	if g.Port == 0 {
		return nil
	}

	hosts := g.Hosts
	if len(hosts) == 0 {
		hosts = defaultHosts
	}

	var addresses []string
	// loop through the hosts and format that into domain.name:port format,
	// ensuring we trim any trailing DNS . characters from the domain name as we
	// go
	for _, h := range hosts {
		addresses = append(addresses, fmt.Sprintf("%s:%d", strings.TrimRight(h, "."), g.Port))
	}
	return addresses
}

func (g *GatewayService) IsSame(o *GatewayService) bool {
	return g.Gateway.Matches(o.Gateway) &&
		g.Service.Matches(o.Service) &&
		g.GatewayKind == o.GatewayKind &&
		g.Port == o.Port &&
		g.Protocol == o.Protocol &&
		stringslice.Equal(g.Hosts, o.Hosts) &&
		g.CAFile == o.CAFile &&
		g.CertFile == o.CertFile &&
		g.KeyFile == o.KeyFile &&
		g.SNI == o.SNI &&
		g.ServiceKind == o.ServiceKind &&
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
		ServiceKind:  g.ServiceKind,
	}
}

// APIGatewayConfigEntry manages the configuration for an API gateway service
// with the given name.
type APIGatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.APIGateway.
	Kind string

	// Name is used to match the config entry with its associated API gateway
	// service. This should match the name provided in the service definition.
	Name string

	// Listeners is the set of listener configuration to which an API Gateway
	// might bind.
	Listeners []APIGatewayListener

	// Status is the asynchronous status which an APIGateway propagates to the user.
	Status Status

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

func (e *APIGatewayConfigEntry) SetHash(h uint64) {
	e.Hash = h
}

func (e *APIGatewayConfigEntry) GetHash() uint64 {
	return e.Hash
}

func (e *APIGatewayConfigEntry) GetKind() string                        { return APIGateway }
func (e *APIGatewayConfigEntry) GetName() string                        { return e.Name }
func (e *APIGatewayConfigEntry) GetMeta() map[string]string             { return e.Meta }
func (e *APIGatewayConfigEntry) GetRaftIndex() *RaftIndex               { return &e.RaftIndex }
func (e *APIGatewayConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta { return &e.EnterpriseMeta }

var _ ControlledConfigEntry = (*APIGatewayConfigEntry)(nil)

func (e *APIGatewayConfigEntry) GetStatus() Status       { return e.Status }
func (e *APIGatewayConfigEntry) SetStatus(status Status) { e.Status = status }
func (e *APIGatewayConfigEntry) DefaultStatus() Status   { return Status{} }

func (e *APIGatewayConfigEntry) ListenerIsReady(name string) bool {
	for _, condition := range e.Status.Conditions {
		if !condition.Resource.IsSame(&ResourceReference{
			Kind:           APIGateway,
			SectionName:    name,
			Name:           e.Name,
			EnterpriseMeta: e.EnterpriseMeta,
		}) {
			continue
		}

		if condition.Type == "Conflicted" && condition.Status == "True" {
			return false
		}
	}

	return true
}

func (e *APIGatewayConfigEntry) Normalize() error {
	for i, listener := range e.Listeners {
		protocol := strings.ToLower(string(listener.Protocol))
		listener.Protocol = APIGatewayListenerProtocol(protocol)
		e.Listeners[i] = listener

		for i, cert := range listener.TLS.Certificates {
			if cert.Kind == "" {
				cert.Kind = InlineCertificate
			}
			cert.EnterpriseMeta.Merge(e.GetEnterpriseMeta())
			cert.EnterpriseMeta.Normalize()

			listener.TLS.Certificates[i] = cert
		}
	}

	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h
	return nil
}

func (e *APIGatewayConfigEntry) Validate() error {
	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	if len(e.Listeners) == 0 {
		return fmt.Errorf("api gateway must have at least one listener")
	}
	if err := e.validateListenerNames(); err != nil {
		return err
	}
	if err := e.validateMergedListeners(); err != nil {
		return err
	}

	return e.validateListeners()
}

var listenerNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

func (e *APIGatewayConfigEntry) validateListenerNames() error {
	listeners := make(map[string]struct{})
	for _, listener := range e.Listeners {
		if len(listener.Name) < 1 || !listenerNameRegex.MatchString(listener.Name) {
			return fmt.Errorf("listener name %q is invalid, must be at least 1 character and contain only letters, numbers, or dashes", listener.Name)
		}
		if _, found := listeners[listener.Name]; found {
			return fmt.Errorf("found multiple listeners with the name %q", listener.Name)
		}
		listeners[listener.Name] = struct{}{}
	}
	return nil
}

func (e *APIGatewayConfigEntry) validateMergedListeners() error {
	listeners := make(map[int]APIGatewayListener)
	for _, listener := range e.Listeners {
		merged, found := listeners[listener.Port]
		if found && (merged.Hostname != listener.Hostname || merged.Protocol != listener.Protocol) {
			return fmt.Errorf("listeners %q and %q cannot be merged", merged.Name, listener.Name)
		}
		listeners[listener.Port] = listener
	}
	return nil
}

func (e *APIGatewayConfigEntry) validateListeners() error {
	validProtocols := map[APIGatewayListenerProtocol]bool{
		ListenerProtocolHTTP: true,
		ListenerProtocolTCP:  true,
	}
	allowedCertificateKinds := map[string]bool{
		InlineCertificate: true,
	}

	for _, listener := range e.Listeners {
		if !validProtocols[listener.Protocol] {
			return fmt.Errorf("unsupported listener protocol %q, must be one of 'tcp', or 'http'", listener.Protocol)
		}
		if listener.Protocol == ListenerProtocolTCP && listener.Hostname != "" {
			// TODO: once we have SNI matching we should be able to implement this
			return fmt.Errorf("hostname specification is not supported when using TCP")
		}
		if listener.Port <= 0 || listener.Port > 65535 {
			return fmt.Errorf("listener port %d not in the range 1-65535", listener.Port)
		}
		if strings.ContainsRune(strings.TrimPrefix(listener.Hostname, wildcardPrefix), '*') {
			return fmt.Errorf("host %q is not valid, a wildcard specifier is only allowed as the left-most label", listener.Hostname)
		}
		for _, certificate := range listener.TLS.Certificates {
			if !allowedCertificateKinds[certificate.Kind] {
				return fmt.Errorf("unsupported certificate kind: %q, must be 'inline-certificate'", certificate.Kind)
			}
			if certificate.Name == "" {
				return fmt.Errorf("certificate reference must have a name")
			}
		}
		if err := validateTLSConfig(listener.TLS.MinVersion, listener.TLS.MaxVersion, listener.TLS.CipherSuites); err != nil {
			return err
		}
	}
	return nil
}

func (e *APIGatewayConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceReadAllowed(e.Name, &authzContext)
}

func (e *APIGatewayConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

// APIGatewayListenerProtocol is the protocol that an APIGateway listener uses
type APIGatewayListenerProtocol string

const (
	ListenerProtocolHTTP APIGatewayListenerProtocol = "http"
	ListenerProtocolTCP  APIGatewayListenerProtocol = "tcp"
)

// APIGatewayListener represents an individual listener for an APIGateway
type APIGatewayListener struct {
	// Name is the name of the listener in a given gateway. This must be
	// unique within a gateway.
	Name string
	// Hostname is the host name that a listener should be bound to. If
	// unspecified, the listener accepts requests for all hostnames.
	Hostname string
	// Port is the port at which this listener should bind.
	Port int
	// Protocol is the protocol that a listener should use. It must
	// either be http or tcp.
	Protocol APIGatewayListenerProtocol
	// TLS is the TLS settings for the listener.
	TLS APIGatewayTLSConfiguration

	// Override is the policy that overrides all other policy and route specific configuration
	Override *APIGatewayPolicy `json:",omitempty"`
	// Default is the policy that is the default for the listener and route, routes can override this behavior
	Default *APIGatewayPolicy `json:",omitempty"`
}

// APIGatewayPolicy holds the policy that configures the gateway listener, this is used in the `Override` and `Default` fields of a listener
type APIGatewayPolicy struct {
	// JWT holds the JWT configuration for the Listener
	JWT *APIGatewayJWTRequirement `json:",omitempty"`
}

func (l APIGatewayListener) GetHostname() string {
	if l.Hostname != "" {
		return l.Hostname
	}
	return "*"
}

// APIGatewayTLSConfiguration specifies the configuration of a listener’s
// TLS settings.
type APIGatewayTLSConfiguration struct {
	// Certificates is a set of references to certificates
	// that a gateway listener uses for TLS termination.
	Certificates []ResourceReference
	// MaxVersion is the maximum TLS version that the listener
	// should support.
	MaxVersion types.TLSVersion
	// MinVersion is the minimum TLS version that the listener
	// should support.
	MinVersion types.TLSVersion
	// CipherSuites is the cipher suites that the listener should support.
	CipherSuites []types.TLSCipherSuite
}

// IsEmpty returns true if all values in the struct are nil or empty.
func (a *APIGatewayTLSConfiguration) IsEmpty() bool {
	return len(a.Certificates) == 0 && len(a.MaxVersion) == 0 && len(a.MinVersion) == 0 && len(a.CipherSuites) == 0
}

// ServiceRouteReferences is a map with a key of ServiceName type for a routed to service from a
// bound gateway listener with a value being a slice of resource references of the routes that reference the service
type ServiceRouteReferences map[ServiceName][]ResourceReference

func (s ServiceRouteReferences) AddService(key ServiceName, routeRef ResourceReference) {
	if s[key] == nil {
		s[key] = make([]ResourceReference, 0)
	}

	if slices.Contains(s[key], routeRef) {
		return
	}

	s[key] = append(s[key], routeRef)
}

func (s ServiceRouteReferences) RemoveRouteRef(routeRef ResourceReference) {
	for key := range s {
		for idx, ref := range s[key] {
			if ref.IsSame(&routeRef) {
				s[key] = append(s[key][0:idx], s[key][idx+1:]...)
				if len(s[key]) == 0 {
					delete(s, key)
				}
			}
		}
	}
}

// this is to make the map value serializable for tests that compare the json output of the
// boundAPIGateway
func (s ServiceRouteReferences) MarshalJSON() ([]byte, error) {
	m := make(map[string][]ResourceReference, len(s))
	for key, val := range s {
		m[key.String()] = val
	}
	return json.Marshal(m)
}

// BoundAPIGatewayConfigEntry manages the configuration for a bound API
// gateway with the given name. This type is never written from the client.
// It is only written by the controller in order to represent an API gateway
// and the resources that are bound to it.
type BoundAPIGatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.BoundAPIGateway.
	Kind string

	// Name is used to match the config entry with its associated API gateway
	// service. This should match the name provided in the corresponding API
	// gateway service definition.
	Name string

	// Listeners are the valid listeners of an APIGateway with information about
	// what certificates and routes have successfully bound to it.
	Listeners []BoundAPIGatewayListener

	// Services are all the services that are routed to from an APIGateway
	Services ServiceRouteReferences

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

func (e *BoundAPIGatewayConfigEntry) SetHash(h uint64) {
	e.Hash = h
}

func (e *BoundAPIGatewayConfigEntry) GetHash() uint64 {
	return e.Hash
}

func (e *BoundAPIGatewayConfigEntry) IsSame(other *BoundAPIGatewayConfigEntry) bool {
	listeners := map[string]BoundAPIGatewayListener{}
	for _, listener := range e.Listeners {
		listeners[listener.Name] = listener
	}

	otherListeners := map[string]BoundAPIGatewayListener{}
	for _, listener := range other.Listeners {
		otherListeners[listener.Name] = listener
	}

	if len(listeners) != len(otherListeners) {
		return false
	}

	for name, listener := range listeners {
		otherListener, found := otherListeners[name]
		if !found {
			return false
		}
		if !listener.IsSame(otherListener) {
			return false
		}
	}

	if len(e.Services) != len(other.Services) {
		return false
	}

	for key, refs := range e.Services {
		if _, ok := other.Services[key]; !ok {
			return false
		}

		if len(refs) != len(other.Services[key]) {
			return false
		}

		for idx, ref := range refs {
			if !ref.IsSame(&other.Services[key][idx]) {
				return false
			}
		}
	}

	return true
}

// IsInitializedForGateway returns whether or not this bound api gateway is initialized with the given api gateway
// including having corresponding listener entries for the gateway.
func (e *BoundAPIGatewayConfigEntry) IsInitializedForGateway(gateway *APIGatewayConfigEntry) bool {
	if e.Name != gateway.Name || !e.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
		return false
	}

	// ensure that this has the same listener data (i.e. it's been reconciled)
	if len(gateway.Listeners) != len(e.Listeners) {
		return false
	}

	for i, listener := range e.Listeners {
		if listener.Name != gateway.Listeners[i].Name {
			return false
		}
	}

	return true
}

func (e *BoundAPIGatewayConfigEntry) GetKind() string            { return BoundAPIGateway }
func (e *BoundAPIGatewayConfigEntry) GetName() string            { return e.Name }
func (e *BoundAPIGatewayConfigEntry) GetMeta() map[string]string { return e.Meta }
func (e *BoundAPIGatewayConfigEntry) Normalize() error {
	for i, listener := range e.Listeners {
		for j, route := range listener.Routes {
			route.EnterpriseMeta.Merge(&e.EnterpriseMeta)
			route.EnterpriseMeta.Normalize()

			listener.Routes[j] = route
		}
		for j, cert := range listener.Certificates {
			cert.EnterpriseMeta.Merge(&e.EnterpriseMeta)
			cert.EnterpriseMeta.Normalize()

			listener.Certificates[j] = cert
		}

		e.Listeners[i] = listener
	}
	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h

	return nil
}

func (e *BoundAPIGatewayConfigEntry) Validate() error {
	allowedCertificateKinds := map[string]bool{
		InlineCertificate: true,
	}
	allowedRouteKinds := map[string]bool{
		HTTPRoute: true,
		TCPRoute:  true,
	}

	// These should already be validated by upstream validation
	// logic in the gateways/routes, but just in case we validate
	// here as well.
	for _, listener := range e.Listeners {
		for _, certificate := range listener.Certificates {
			if !allowedCertificateKinds[certificate.Kind] {
				return fmt.Errorf("unsupported certificate kind: %q, must be 'inline-certificate'", certificate.Kind)
			}
			if certificate.Name == "" {
				return fmt.Errorf("certificate reference must have a name")
			}
		}
		for _, route := range listener.Routes {
			if !allowedRouteKinds[route.Kind] {
				return fmt.Errorf("unsupported route kind: %q, must be one of 'http-route', or 'tcp-route'", route.Kind)
			}
			if route.Name == "" {
				return fmt.Errorf("route reference must have a name")
			}
		}
	}
	return nil
}

func (e *BoundAPIGatewayConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceReadAllowed(e.Name, &authzContext)
}

func (e *BoundAPIGatewayConfigEntry) CanWrite(_ acl.Authorizer) error {
	return acl.PermissionDenied("only writeable by controller")
}

func (e *BoundAPIGatewayConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}
	return &e.RaftIndex
}

func (e *BoundAPIGatewayConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}
	return &e.EnterpriseMeta
}

func (e *BoundAPIGatewayConfigEntry) ListRelatedServices() []ServiceID {
	if len(e.Services) == 0 {
		return nil
	}

	ids := make([]ServiceID, 0, len(e.Services))
	for key := range e.Services {
		ids = append(ids, key.ToServiceID())
	}
	return ids
}

// BoundAPIGatewayListener is an API gateway listener with information
// about the routes and certificates that have successfully bound to it.
type BoundAPIGatewayListener struct {
	Name         string
	Routes       []ResourceReference
	Certificates []ResourceReference
}

func sameResources(first, second []ResourceReference) bool {
	if len(first) != len(second) {
		return false
	}
	for _, firstRef := range first {
		found := false
		for _, secondRef := range second {
			if firstRef.IsSame(&secondRef) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (l BoundAPIGatewayListener) IsSame(other BoundAPIGatewayListener) bool {
	if l.Name != other.Name {
		return false
	}
	if !sameResources(l.Certificates, other.Certificates) {
		return false
	}
	return sameResources(l.Routes, other.Routes)
}

// BindRoute is used to create or update a route on the listener.
// It returns true if the route was able to be bound to the listener.
// Routes should only bind to listeners with their same section name
// and protocol. Be sure to check both of these before attempting
// to bind a route to the listener.
func (l *BoundAPIGatewayListener) BindRoute(routeRef ResourceReference) bool {
	// If the listener has no routes, create a new slice of routes with the given route.
	if l.Routes == nil {
		l.Routes = []ResourceReference{routeRef}
		return true
	}

	// If the route matches an existing route, update it and return.
	for i, listenerRoute := range l.Routes {
		if listenerRoute.Kind == routeRef.Kind && listenerRoute.Name == routeRef.Name && listenerRoute.EnterpriseMeta.IsSame(&routeRef.EnterpriseMeta) {
			l.Routes[i] = routeRef
			return true
		}
	}

	// If the route is new to the listener, append it.
	l.Routes = append(l.Routes, routeRef)

	return true
}

func (l *BoundAPIGatewayListener) UnbindRoute(route ResourceReference) bool {
	if l == nil {
		return false
	}

	for i, listenerRoute := range l.Routes {
		if listenerRoute.Kind == route.Kind && listenerRoute.Name == route.Name && listenerRoute.EnterpriseMeta.IsSame(&route.EnterpriseMeta) {
			l.Routes = append(l.Routes[:i], l.Routes[i+1:]...)
			return true
		}
	}

	return false
}

func (e *BoundAPIGatewayConfigEntry) GetStatus() Status       { return Status{} }
func (e *BoundAPIGatewayConfigEntry) SetStatus(status Status) {}
func (e *BoundAPIGatewayConfigEntry) DefaultStatus() Status   { return Status{} }
