package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
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

	// VerifyReadACL and VerifyWriteACL return whether or not the given Authorizer
	// has permission to read or write to the config entry, respectively.
	VerifyReadACL(acl.Authorizer) bool
	VerifyWriteACL(acl.Authorizer) bool

	GetRaftIndex() *RaftIndex
}

// ServiceConfiguration is the top-level struct for the configuration of a service
// across the entire cluster.
type ServiceConfigEntry struct {
	Kind     string
	Name     string
	Protocol string
	Connect  ConnectConfiguration

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

func (e *ServiceConfigEntry) VerifyReadACL(rule acl.Authorizer) bool {
	return rule.ServiceRead(e.Name)
}

func (e *ServiceConfigEntry) VerifyWriteACL(rule acl.Authorizer) bool {
	return rule.ServiceWrite(e.Name, nil)
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
	e.Name = ProxyConfigGlobal

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

func (e *ProxyConfigEntry) VerifyReadACL(rule acl.Authorizer) bool {
	return true
}

func (e *ProxyConfigEntry) VerifyWriteACL(rule acl.Authorizer) bool {
	return rule.OperatorWrite()
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

// ConfigEntryRequest is used when creating/updating/deleting a ConfigEntry.
type ConfigEntryRequest struct {
	Op         ConfigEntryOp
	Datacenter string
	Entry      ConfigEntry

	WriteRequest
}

func (c *ConfigEntryRequest) RequestDatacenter() string {
	return c.Datacenter
}

func (c *ConfigEntryRequest) MarshalBinary() (data []byte, err error) {
	// bs will grow if needed but allocate enough to avoid reallocation in common
	// case.
	bs := make([]byte, 128)
	enc := codec.NewEncoderBytes(&bs, msgpackHandle)
	// Encode kind first
	err = enc.Encode(c.Entry.GetKind())
	if err != nil {
		return nil, err
	}
	// Then actual value using alias trick to avoid infinite recursion
	type Alias ConfigEntryRequest
	err = enc.Encode(struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	})
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func (c *ConfigEntryRequest) UnmarshalBinary(data []byte) error {
	// First decode the kind prefix
	var kind string
	dec := codec.NewDecoderBytes(data, msgpackHandle)
	if err := dec.Decode(&kind); err != nil {
		return err
	}

	// Then decode the real thing with appropriate kind of ConfigEntry
	entry, err := MakeConfigEntry(kind, "")
	if err != nil {
		return err
	}
	c.Entry = entry

	// Alias juggling to prevent infinite recursive calls back to this decode
	// method.
	type Alias ConfigEntryRequest
	as := struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := dec.Decode(&as); err != nil {
		return err
	}
	return nil
}

func MakeConfigEntry(kind, name string) (ConfigEntry, error) {
	switch kind {
	case ServiceDefaults:
		return &ServiceConfigEntry{Name: name}, nil
	case ProxyDefaults:
		return &ProxyConfigEntry{Name: name}, nil
	default:
		return nil, fmt.Errorf("invalid config entry kind: %s", kind)
	}
}

// ConfigEntryQuery is used when requesting info about a config entry.
type ConfigEntryQuery struct {
	Kind       string
	Name       string
	Datacenter string

	QueryOptions
}

func (c *ConfigEntryQuery) RequestDatacenter() string {
	return c.Datacenter
}

// ServiceConfigRequest is used when requesting the resolved configuration
// for a service.
type ServiceConfigRequest struct {
	Name       string
	Datacenter string

	QueryOptions
}

func (s *ServiceConfigRequest) RequestDatacenter() string {
	return s.Datacenter
}

type ServiceConfigResponse struct {
	Definition ServiceDefinition

	QueryMeta
}
