package structs

import "fmt"

const (
	ServiceDefaults string = "service-defaults"
	ProxyDefaults   string = "proxy-defaults"

	ProxyConfigGlobal string = "global"
)

// ConfigEntry is the
type ConfigEntry interface {
	GetKind() string
	GetName() string

	// This is called in the RPC endpoint and can apply defaults
	Normalize() error
	Validate() error

	GetRaftIndex() *RaftIndex
}

// ServiceConfiguration is the top-level struct for the configuration of a service
// across the entire cluster.
type ServiceConfigEntry struct {
	Kind                      string
	Name                      string
	Protocol                  string
	Connect                   ConnectConfiguration
	ServiceDefinitionDefaults ServiceDefinitionDefaults

	RaftIndex
}

func (e *ServiceConfigEntry) GetKind() string {
	return ServiceDefaults
}

func (e *ServiceConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *ServiceConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ServiceDefaults

	return nil
}

func (e *ServiceConfigEntry) Validate() error {
	return nil
}

func (e *ServiceConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
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

// ProxyConfigEntry is the top-level struct for global proxy configuration defaults.
type ProxyConfigEntry struct {
	Kind        string
	Name        string
	ProxyConfig ConnectProxyConfig

	RaftIndex
}

func (e *ProxyConfigEntry) GetKind() string {
	return ProxyDefaults
}

func (e *ProxyConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *ProxyConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ProxyDefaults

	return nil
}

func (e *ProxyConfigEntry) Validate() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	if e.Name != ProxyConfigGlobal {
		return fmt.Errorf("invalid name (%q), only %q is supported", e.Name, ProxyConfigGlobal)
	}

	return nil
}

func (e *ProxyConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

type ConfigEntryOp string

const (
	ConfigEntryUpsert ConfigEntryOp = "upsert"
	ConfigEntryDelete ConfigEntryOp = "delete"
)

type ConfigEntryRequest struct {
	Op    ConfigEntryOp
	Entry ConfigEntry
}
