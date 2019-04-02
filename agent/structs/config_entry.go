package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-msgpack/codec"
)

const (
	ServiceDefaults string = "service-defaults"
	ProxyDefaults   string = "proxy-defaults"

	ProxyConfigGlobal string = "global"

	DefaultServiceProtocol = "tcp"
)

// ConfigEntry is the
type ConfigEntry interface {
	GetKind() string
	GetName() string

	// This is called in the RPC endpoint and can apply defaults or limits.
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
	if e.Protocol == "" {
		e.Protocol = DefaultServiceProtocol
	} else {
		e.Protocol = strings.ToLower(e.Protocol)
	}

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
}

// ProxyConfigEntry is the top-level struct for global proxy configuration defaults.
type ProxyConfigEntry struct {
	Kind   string
	Name   string
	Config map[string]interface{}

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

func (r *ConfigEntryRequest) MarshalBinary() (data []byte, err error) {
	// bs will grow if needed but allocate enough to avoid reallocation in common
	// case.
	bs := make([]byte, 128)
	enc := codec.NewEncoderBytes(&bs, msgpackHandle)
	// Encode kind first
	err = enc.Encode(r.Entry.GetKind())
	if err != nil {
		return nil, err
	}
	// Then actual value using alias trick to avoid infinite recursion
	type Alias ConfigEntryRequest
	err = enc.Encode(struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func (r *ConfigEntryRequest) UnmarshalBinary(data []byte) error {
	// First decode the kind prefix
	var kind string
	dec := codec.NewDecoderBytes(data, msgpackHandle)
	if err := dec.Decode(&kind); err != nil {
		return err
	}

	// Then decode the real thing with appropriate kind of ConfigEntry
	entry, err := makeConfigEntry(kind)
	if err != nil {
		return err
	}
	r.Entry = entry

	// Alias juggling to prevent infinite recursive calls back to this decode
	// method.
	type Alias ConfigEntryRequest
	as := struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := dec.Decode(&as); err != nil {
		return err
	}
	return nil
}

func makeConfigEntry(kind string) (ConfigEntry, error) {
	switch kind {
	case ServiceDefaults:
		return &ServiceConfigEntry{}, nil
	case ProxyDefaults:
		return &ProxyConfigEntry{}, nil
	default:
		return nil, fmt.Errorf("invalid config entry kind: %s", kind)
	}
}
