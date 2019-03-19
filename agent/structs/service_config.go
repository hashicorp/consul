package structs

type ConfigurationKind string

const (
	ServiceDefaults ConfigurationKind = "service-defaults"
	ProxyDefaults   ConfigurationKind = "proxy-defaults"
)

// Should this be an interface or a switch on the existing config types?
type Configuration interface {
	GetKind() ConfigurationKind
	GetName() string
	Validate() error
}

// ServiceConfiguration is the top-level struct for the configuration of a service
// across the entire cluster.
type ServiceConfiguration struct {
	Kind                      ConfigurationKind
	Name                      string
	Protocol                  string
	Connect                   ConnectConfiguration
	ServiceDefinitionDefaults ServiceDefinitionDefaults

	RaftIndex
}

func (s *ServiceConfiguration) GetKind() ConfigurationKind {
	return ServiceDefaults
}

type ConnectConfiguration struct {
	SidecarProxy bool
}

type ServiceDefinitionDefaults struct {
	EnableTagOverride bool

	// Non script/docker checks only
	Check  *HealthCheck
	Checks HealthChecks

	// Kind is allowed to accommodate non-sidecar proxies but it will be an error
	// if they also set Connect.DestinationServiceID since sidecars are
	// configured via their associated service's config.
	Kind ServiceKind

	// Only DestinationServiceName and Config are supported.
	Proxy ConnectProxyConfig

	Connect ServiceConnect

	Weights Weights

	// DisableDirectDiscovery is a field that marks the service instance as
	// not discoverable. This is useful in two cases:
	//   1. Truly headless services like job workers that still need Connect
	//      sidecars to connect to upstreams.
	//   2. Connect applications that expose services only through their sidecar
	//      and so discovery of their IP/port is meaningless since they can't be
	//      connected to by that means.
	DisableDirectDiscovery bool
}

// ProxyConfiguration is the top-level struct for global proxy configuration defaults.
type ProxyConfiguration struct {
	Kind        ConfigurationKind
	Name        string
	ProxyConfig ConnectProxyConfig
}

func (p *ProxyConfiguration) GetKind() ConfigurationKind {
	return ProxyDefaults
}
