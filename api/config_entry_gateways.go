package api

// IngressGatewayConfigEntry manages the configuration for an ingress service
// with the given name.
type IngressGatewayConfigEntry struct {
	// Kind of the config entry. This should be set to api.IngressGateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	// Partition is the partition the IngressGateway is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the IngressGateway is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`

	// TLS holds the TLS configuration for this gateway.
	TLS GatewayTLSConfig

	// Listeners declares what ports the ingress gateway should listen on, and
	// what services to associated to those ports.
	Listeners []IngressListener

	Meta map[string]string `json:",omitempty"`

	// Defaults is default configuration for all upstream services
	Defaults *IngressServiceConfig `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64
}

type IngressServiceConfig struct {
	MaxConnections        *uint32
	MaxPendingRequests    *uint32
	MaxConcurrentRequests *uint32
}

type GatewayTLSConfig struct {
	// Indicates that TLS should be enabled for this gateway service.
	Enabled bool

	// SDS allows configuring TLS certificate from an SDS service.
	SDS *GatewayTLSSDSConfig `json:",omitempty"`

	TLSMinVersion string `json:",omitempty" alias:"tls_min_version"`
	TLSMaxVersion string `json:",omitempty" alias:"tls_max_version"`

	// Define a subset of cipher suites to restrict
	// Only applicable to connections negotiated via TLS 1.2 or earlier
	CipherSuites []string `json:",omitempty" alias:"cipher_suites"`
}

type GatewayServiceTLSConfig struct {
	// SDS allows configuring TLS certificate from an SDS service.
	SDS *GatewayTLSSDSConfig `json:",omitempty"`
}

type GatewayTLSSDSConfig struct {
	ClusterName  string `json:",omitempty" alias:"cluster_name"`
	CertResource string `json:",omitempty" alias:"cert_resource"`
}

// IngressListener manages the configuration for a listener on a specific port.
type IngressListener struct {
	// Port declares the port on which the ingress gateway should listen for traffic.
	Port int

	// Protocol declares what type of traffic this listener is expected to
	// receive. Depending on the protocol, a listener might support multiplexing
	// services over a single port, or additional discovery chain features. The
	// current supported values are: (tcp | http | http2 | grpc).
	Protocol string

	// Services declares the set of services to which the listener forwards
	// traffic.
	//
	// For "tcp" protocol listeners, only a single service is allowed.
	// For "http" listeners, multiple services can be declared.
	Services []IngressService

	// TLS allows specifying some TLS configuration per listener.
	TLS *GatewayTLSConfig `json:",omitempty"`
}

// IngressService manages configuration for services that are exposed to
// ingress traffic.
type IngressService struct {
	// Name declares the service to which traffic should be forwarded.
	//
	// This can either be a specific service, or the wildcard specifier,
	// "*". If the wildcard specifier is provided, the listener must be of "http"
	// protocol and means that the listener will forward traffic to all services.
	//
	// A name can be specified on multiple listeners, and will be exposed on both
	// of the listeners.
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

	// Namespace is the namespace where the service is located.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`

	// Partition is the partition where the service is located.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// TLS allows specifying some TLS configuration per listener.
	TLS *GatewayServiceTLSConfig `json:",omitempty"`

	// Allow HTTP header manipulation to be configured.
	RequestHeaders  *HTTPHeaderModifiers `json:",omitempty" alias:"request_headers"`
	ResponseHeaders *HTTPHeaderModifiers `json:",omitempty" alias:"response_headers"`

	MaxConnections        *uint32 `json:",omitempty" alias:"max_connections"`
	MaxPendingRequests    *uint32 `json:",omitempty" alias:"max_pending_requests"`
	MaxConcurrentRequests *uint32 `json:",omitempty" alias:"max_concurrent_requests"`
}

func (i *IngressGatewayConfigEntry) GetKind() string            { return i.Kind }
func (i *IngressGatewayConfigEntry) GetName() string            { return i.Name }
func (i *IngressGatewayConfigEntry) GetPartition() string       { return i.Partition }
func (i *IngressGatewayConfigEntry) GetNamespace() string       { return i.Namespace }
func (i *IngressGatewayConfigEntry) GetMeta() map[string]string { return i.Meta }
func (i *IngressGatewayConfigEntry) GetCreateIndex() uint64     { return i.CreateIndex }
func (i *IngressGatewayConfigEntry) GetModifyIndex() uint64     { return i.ModifyIndex }

// TerminatingGatewayConfigEntry manages the configuration for a terminating gateway
// with the given name.
type TerminatingGatewayConfigEntry struct {
	// Kind of the config entry. This should be set to api.TerminatingGateway.
	Kind string

	// Name is used to match the config entry with its associated terminating gateway
	// service. This should match the name provided in the service definition.
	Name string

	// Services is a list of service names represented by the terminating gateway.
	Services []LinkedService `json:",omitempty"`

	Meta map[string]string `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64

	// Partition is the partition the config entry is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the config entry is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`
}

// A LinkedService is a service represented by a terminating gateway
type LinkedService struct {
	// Referencing other partitions is not supported.

	// Namespace is where the service is registered.
	Namespace string `json:",omitempty"`

	// Name is the name of the service, as defined in Consul's catalog.
	Name string `json:",omitempty"`

	// CAFile is the optional path to a CA certificate to use for TLS connections
	// from the gateway to the linked service.
	CAFile string `json:",omitempty" alias:"ca_file"`

	// CertFile is the optional path to a client certificate to use for TLS connections
	// from the gateway to the linked service.
	CertFile string `json:",omitempty" alias:"cert_file"`

	// KeyFile is the optional path to a private key to use for TLS connections
	// from the gateway to the linked service.
	KeyFile string `json:",omitempty" alias:"key_file"`

	// SNI is the optional name to specify during the TLS handshake with a linked service.
	SNI string `json:",omitempty"`
}

func (g *TerminatingGatewayConfigEntry) GetKind() string            { return g.Kind }
func (g *TerminatingGatewayConfigEntry) GetName() string            { return g.Name }
func (g *TerminatingGatewayConfigEntry) GetPartition() string       { return g.Partition }
func (g *TerminatingGatewayConfigEntry) GetNamespace() string       { return g.Namespace }
func (g *TerminatingGatewayConfigEntry) GetMeta() map[string]string { return g.Meta }
func (g *TerminatingGatewayConfigEntry) GetCreateIndex() uint64     { return g.CreateIndex }
func (g *TerminatingGatewayConfigEntry) GetModifyIndex() uint64     { return g.ModifyIndex }
