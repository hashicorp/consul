// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/hashstructure"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/envoyextensions"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/decode"
)

const (
	ServiceDefaults       string = "service-defaults"
	ProxyDefaults         string = "proxy-defaults"
	ServiceRouter         string = "service-router"
	ServiceSplitter       string = "service-splitter"
	ServiceResolver       string = "service-resolver"
	IngressGateway        string = "ingress-gateway"
	TerminatingGateway    string = "terminating-gateway"
	ServiceIntentions     string = "service-intentions"
	MeshConfig            string = "mesh"
	ExportedServices      string = "exported-services"
	SamenessGroup         string = "sameness-group"
	APIGateway            string = "api-gateway"
	BoundAPIGateway       string = "bound-api-gateway"
	FileSystemCertificate string = "file-system-certificate"
	InlineCertificate     string = "inline-certificate"
	HTTPRoute             string = "http-route"
	TCPRoute              string = "tcp-route"
	// TODO: decide if we want to highlight 'ip' keyword in the name of RateLimitIPConfig
	RateLimitIPConfig string = "control-plane-request-limit"
	JWTProvider       string = "jwt-provider"

	ProxyConfigGlobal string = "global"
	MeshConfigMesh    string = "mesh"

	DefaultServiceProtocol = "tcp"

	ConnectionExactBalance = "exact_balance"
)

var AllConfigEntryKinds = []string{
	ServiceDefaults,
	ProxyDefaults,
	ServiceRouter,
	ServiceSplitter,
	ServiceResolver,
	IngressGateway,
	TerminatingGateway,
	ServiceIntentions,
	MeshConfig,
	ExportedServices,
	SamenessGroup,
	APIGateway,
	BoundAPIGateway,
	HTTPRoute,
	TCPRoute,
	FileSystemCertificate,
	InlineCertificate,
	RateLimitIPConfig,
	JWTProvider,
}

// ConfigEntry is the interface for centralized configuration stored in Raft.
// Currently only service-defaults and proxy-defaults are supported.
type ConfigEntry interface {
	GetKind() string
	GetName() string

	// This is called in the RPC endpoint and can apply defaults or limits.
	Normalize() error
	Validate() error

	// CanRead and CanWrite return whether or not the given Authorizer
	// has permission to read or write to the config entry, respectively.
	// TODO(acl-error-enhancements) This should be resolver.Result or similar but we have to wait until we move things to the acl package
	CanRead(acl.Authorizer) error
	CanWrite(acl.Authorizer) error

	GetMeta() map[string]string
	GetEnterpriseMeta() *acl.EnterpriseMeta
	GetRaftIndex() *RaftIndex
	GetHash() uint64
	SetHash(h uint64)
}

func HashConfigEntry(conf ConfigEntry) (uint64, error) {
	hash, err := hashstructure.Hash(conf, nil)
	if err != nil {
		return hash, err
	}
	return hash, nil
}

// ControlledConfigEntry is an optional interface implemented by a ConfigEntry
// if it is reconciled via a controller and needs to respond with Status values.
type ControlledConfigEntry interface {
	DefaultStatus() Status
	GetStatus() Status
	SetStatus(status Status)
	ConfigEntry
}

// UpdatableConfigEntry is the optional interface implemented by a ConfigEntry
// if it wants more control over how the update part of upsert works
// differently than a straight create. By default without this implementation
// all upsert operations are replacements.
type UpdatableConfigEntry interface {
	// UpdateOver is called from the state machine when an identically named
	// config entry already exists. This lets the config entry optionally
	// choose to use existing information from a config entry (such as
	// CreateTime) to slightly adjust how the update actually happens.
	UpdateOver(prev ConfigEntry) error
	ConfigEntry
}

// WarningConfigEntry is an optional interface implemented by a ConfigEntry
// if it wants to be able to emit warnings when it is being upserted.
type WarningConfigEntry interface {
	Warnings() []string

	ConfigEntry
}

type MutualTLSMode string

const (
	MutualTLSModeDefault    MutualTLSMode = ""
	MutualTLSModeStrict     MutualTLSMode = "strict"
	MutualTLSModePermissive MutualTLSMode = "permissive"
)

func (m MutualTLSMode) validate() error {
	switch m {
	case MutualTLSModeDefault, MutualTLSModeStrict, MutualTLSModePermissive:
		return nil
	}
	return fmt.Errorf("Invalid MutualTLSMode %q. Must be one of %q, %q, or %q.", m,
		MutualTLSModeDefault,
		MutualTLSModeStrict,
		MutualTLSModePermissive,
	)
}

// ServiceConfiguration is the top-level struct for the configuration of a service
// across the entire cluster.
type ServiceConfigEntry struct {
	Kind                      string
	Name                      string
	Protocol                  string
	Mode                      ProxyMode              `json:",omitempty"`
	TransparentProxy          TransparentProxyConfig `json:",omitempty" alias:"transparent_proxy"`
	MutualTLSMode             MutualTLSMode          `json:",omitempty" alias:"mutual_tls_mode"`
	MeshGateway               MeshGatewayConfig      `json:",omitempty" alias:"mesh_gateway"`
	Expose                    ExposeConfig           `json:",omitempty"`
	ExternalSNI               string                 `json:",omitempty" alias:"external_sni"`
	UpstreamConfig            *UpstreamConfiguration `json:",omitempty" alias:"upstream_config"`
	Destination               *DestinationConfig     `json:",omitempty"`
	MaxInboundConnections     int                    `json:",omitempty" alias:"max_inbound_connections"`
	LocalConnectTimeoutMs     int                    `json:",omitempty" alias:"local_connect_timeout_ms"`
	LocalRequestTimeoutMs     int                    `json:",omitempty" alias:"local_request_timeout_ms"`
	BalanceInboundConnections string                 `json:",omitempty" alias:"balance_inbound_connections"`
	RateLimits                *RateLimits            `json:",omitempty" alias:"rate_limits"`
	EnvoyExtensions           EnvoyExtensions        `json:",omitempty" alias:"envoy_extensions"`

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

func (e *ServiceConfigEntry) SetHash(h uint64) {
	e.Hash = h
}

func (e *ServiceConfigEntry) GetHash() uint64 {
	return e.Hash
}

func (e *ServiceConfigEntry) Clone() *ServiceConfigEntry {
	e2 := *e
	e2.Expose = e.Expose.Clone()
	e2.UpstreamConfig = e.UpstreamConfig.Clone()
	return &e2
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

func (e *ServiceConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *ServiceConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ServiceDefaults
	e.Protocol = strings.ToLower(e.Protocol)
	e.EnterpriseMeta.Normalize()

	var validationErr error

	if e.UpstreamConfig != nil {
		for _, override := range e.UpstreamConfig.Overrides {
			err := override.NormalizeWithName(&e.EnterpriseMeta)
			if err != nil {
				validationErr = multierror.Append(validationErr, fmt.Errorf("error in upstream override for %s: %v", override.PeeredServiceName(), err))
			}
		}

		if e.UpstreamConfig.Defaults != nil {
			err := e.UpstreamConfig.Defaults.NormalizeWithoutName()
			if err != nil {
				validationErr = multierror.Append(validationErr, fmt.Errorf("error in upstream defaults: %v", err))
			}
		}
	}
	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h

	return validationErr
}

func (e *ServiceConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}
	if e.Name == WildcardSpecifier {
		return fmt.Errorf("service-defaults name must be the name of a service, and not a wildcard")
	}

	validationErr := validateConfigEntryMeta(e.Meta)

	if !isValidConnectionBalance(e.BalanceInboundConnections) {
		validationErr = multierror.Append(validationErr, fmt.Errorf("invalid value for balance_inbound_connections: %v", e.BalanceInboundConnections))
	}

	// External endpoints are invalid with an existing service's upstream configuration
	if e.UpstreamConfig != nil && e.Destination != nil {
		validationErr = multierror.Append(validationErr, errors.New("UpstreamConfig and Destination are mutually exclusive for service defaults"))
		return validationErr
	}

	if e.UpstreamConfig != nil {
		for _, override := range e.UpstreamConfig.Overrides {
			err := override.ValidateWithName()
			if err != nil {
				validationErr = multierror.Append(validationErr, fmt.Errorf("error in upstream override for %s: %v", override.PeeredServiceName(), err))
			}
		}

		if e.UpstreamConfig.Defaults != nil {
			if err := e.UpstreamConfig.Defaults.ValidateWithoutName(); err != nil {
				validationErr = multierror.Append(validationErr, fmt.Errorf("error in upstream defaults: %v", err))
			}
		}
	}

	if e.Destination != nil {
		if e.Destination.Addresses == nil || len(e.Destination.Addresses) == 0 {
			validationErr = multierror.Append(validationErr, errors.New("Destination must contain at least one valid address"))
		}

		seen := make(map[string]bool, len(e.Destination.Addresses))
		for _, address := range e.Destination.Addresses {
			if _, ok := seen[address]; ok {
				validationErr = multierror.Append(validationErr, fmt.Errorf("Duplicate address '%s' is not allowed", address))
				continue
			}
			seen[address] = true

			if err := validateEndpointAddress(address); err != nil {
				validationErr = multierror.Append(validationErr, fmt.Errorf("Destination address '%s' is invalid %w", address, err))
			}
		}

		if e.Destination.Port < 1 || e.Destination.Port > 65535 {
			validationErr = multierror.Append(validationErr, fmt.Errorf("Invalid Port number %d", e.Destination.Port))
		}
	}

	if err := validateRatelimit(e.RateLimits); err != nil {
		validationErr = multierror.Append(validationErr, err)
	}

	if err := envoyextensions.ValidateExtensions(e.EnvoyExtensions.ToAPI()); err != nil {
		validationErr = multierror.Append(validationErr, err)
	}

	if err := e.MutualTLSMode.validate(); err != nil {
		return err
	}

	return validationErr
}

func validateEndpointAddress(address string) error {
	var valid bool

	ip := net.ParseIP(address)
	valid = ip != nil

	hasWildcard := strings.Contains(address, "*")
	_, ok := dns.IsDomainName(address)
	valid = valid || (ok && !hasWildcard)

	if !valid {
		return fmt.Errorf("Could not validate address %s as an IP or Hostname", address)
	}
	return nil
}

func (e *ServiceConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceReadAllowed(e.Name, &authzContext)
}

func (e *ServiceConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceWriteAllowed(e.Name, &authzContext)
}

func (e *ServiceConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

type UpstreamConfiguration struct {
	// Overrides is a slice of per-service configuration. The name field is
	// required.
	Overrides []*UpstreamConfig `json:",omitempty"`

	// Defaults contains default configuration for all upstreams of a given
	// service. The name field must be empty.
	Defaults *UpstreamConfig `json:",omitempty"`
}

func (c *UpstreamConfiguration) Clone() *UpstreamConfiguration {
	if c == nil {
		return nil
	}

	var c2 UpstreamConfiguration
	if len(c.Overrides) > 0 {
		c2.Overrides = make([]*UpstreamConfig, 0, len(c.Overrides))
		for _, o := range c.Overrides {
			dup := o.Clone()
			c2.Overrides = append(c2.Overrides, &dup)
		}
	}

	if c.Defaults != nil {
		def2 := c.Defaults.Clone()
		c2.Defaults = &def2
	}

	return &c2
}

// DestinationConfig represents a virtual service, i.e. one that is external to Consul
type DestinationConfig struct {
	// Addresses of the endpoint; hostname or IP
	Addresses []string `json:",omitempty"`

	// Port allowed within this endpoint
	Port int `json:",omitempty"`
}

func IsIP(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil
}

// RateLimits is rate limiting configuration that is applied to
// inbound traffic for a service.
// Rate limiting is a Consul enterprise feature.
type RateLimits struct {
	InstanceLevel InstanceLevelRateLimits `alias:"instance_level"`
}

// InstanceLevelRateLimits represents rate limit configuration
// that are applied per service instance.
type InstanceLevelRateLimits struct {
	// RequestsPerSecond is the average number of requests per second that can be
	// made without being throttled. This field is required if RequestsMaxBurst
	// is set. The allowed number of requests may exceed RequestsPerSecond up to
	// the value specified in RequestsMaxBurst.
	//
	// Internally, this is the refill rate of the token bucket used for rate limiting.
	RequestsPerSecond int `alias:"requests_per_second"`

	// RequestsMaxBurst is the maximum number of requests that can be sent
	// in a burst. Should be equal to or greater than RequestsPerSecond.
	// If unset, defaults to RequestsPerSecond.
	//
	// Internally, this is the maximum size of the token bucket used for rate limiting.
	RequestsMaxBurst int `alias:"requests_max_burst"`

	// Routes is a list of rate limits applied to specific routes.
	// For a given request, the first matching route will be applied, if any.
	// Overrides any top-level configuration.
	Routes []InstanceLevelRouteRateLimits
}

// InstanceLevelRouteRateLimits represents rate limit configuration
// applied to a route matching one of PathExact/PathPrefix/PathRegex.
type InstanceLevelRouteRateLimits struct {
	PathExact  string `alias:"path_exact"`
	PathPrefix string `alias:"path_prefix"`
	PathRegex  string `alias:"path_regex"`

	RequestsPerSecond int `alias:"requests_per_second"`
	RequestsMaxBurst  int `alias:"requests_max_burst"`
}

// ProxyConfigEntry is the top-level struct for global proxy configuration defaults.
type ProxyConfigEntry struct {
	Kind                 string
	Name                 string
	Config               map[string]interface{}
	Protocol             string                               `json:"-"`
	Mode                 ProxyMode                            `json:",omitempty"`
	TransparentProxy     TransparentProxyConfig               `json:",omitempty" alias:"transparent_proxy"`
	MutualTLSMode        MutualTLSMode                        `json:",omitempty" alias:"mutual_tls_mode"`
	MeshGateway          MeshGatewayConfig                    `json:",omitempty" alias:"mesh_gateway"`
	Expose               ExposeConfig                         `json:",omitempty"`
	AccessLogs           AccessLogsConfig                     `json:",omitempty" alias:"access_logs"`
	EnvoyExtensions      EnvoyExtensions                      `json:",omitempty" alias:"envoy_extensions"`
	FailoverPolicy       *ServiceResolverFailoverPolicy       `json:",omitempty" alias:"failover_policy"`
	PrioritizeByLocality *ServiceResolverPrioritizeByLocality `json:",omitempty" alias:"prioritize_by_locality"`

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

func (e *ProxyConfigEntry) SetHash(h uint64) {
	e.Hash = h
}

func (e *ProxyConfigEntry) GetHash() uint64 {
	return e.Hash
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

func (e *ProxyConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *ProxyConfigEntry) ComputeProtocol() error {
	// proxyConfig is a snippet from agent/xds/config.go:ProxyConfig
	// We calculate this up-front so that the expensive mapstructure decode
	// is not needed during discovery chain compile time.
	type proxyConfig struct {
		Protocol string `mapstructure:"protocol"`
	}
	var cfg proxyConfig
	err := mapstructure.WeakDecode(e.Config, &cfg)
	if err != nil {
		return err
	}
	e.Protocol = cfg.Protocol
	return nil
}

func (e *ProxyConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ProxyDefaults

	// proxy default config only accepts global configs
	// this check is replicated in normalize() and validate(),
	// since validate is not called by all the endpoints (e.g., delete)
	if e.Name != "" && e.Name != ProxyConfigGlobal {
		return fmt.Errorf("invalid name (%q), only %q is supported", e.Name, ProxyConfigGlobal)
	}
	e.Name = ProxyConfigGlobal

	e.EnterpriseMeta.Normalize()

	if err := e.ComputeProtocol(); err != nil {
		return err
	}

	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h
	return nil

}

func (e *ProxyConfigEntry) Validate() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	if e.Name != ProxyConfigGlobal {
		return fmt.Errorf("invalid name (%q), only %q is supported", e.Name, ProxyConfigGlobal)
	}

	if err := e.AccessLogs.Validate(); err != nil {
		return err
	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	if err := validateOpaqueProxyConfig(e.Config); err != nil {
		return fmt.Errorf("Config: %w", err)
	}

	if err := envoyextensions.ValidateExtensions(e.EnvoyExtensions.ToAPI()); err != nil {
		return err
	}

	if err := e.FailoverPolicy.validate(); err != nil {
		return err
	}

	if err := e.PrioritizeByLocality.validate(); err != nil {
		return err
	}

	if err := e.MutualTLSMode.validate(); err != nil {
		return err
	}

	return e.validateEnterpriseMeta()
}

func (e *ProxyConfigEntry) CanRead(authz acl.Authorizer) error {
	return nil
}

func (e *ProxyConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *ProxyConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ProxyConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (e *ProxyConfigEntry) MarshalBinary() (data []byte, err error) {
	// We mainly want to implement the BinaryMarshaller interface so that
	// we can fixup some msgpack types to coerce them into JSON compatible
	// values. No special encoding needs to be done - we just simply msgpack
	// encode the struct which requires a type alias to prevent recursively
	// calling this function.

	type alias ProxyConfigEntry

	a := alias(*e)

	// bs will grow if needed but allocate enough to avoid reallocation in common
	// case.
	bs := make([]byte, 128)
	enc := codec.NewEncoderBytes(&bs, MsgpackHandle)
	err = enc.Encode(a)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

func (e *ProxyConfigEntry) UnmarshalBinary(data []byte) error {
	// The goal here is to add a post-decoding operation to
	// decoding of a ProxyConfigEntry. The cleanest way I could
	// find to do so was to implement the BinaryMarshaller interface
	// and use a type alias to do the original round of decoding,
	// followed by a MapWalk of the Config to coerce everything
	// into JSON compatible types.
	type alias ProxyConfigEntry

	var a alias
	dec := codec.NewDecoderBytes(data, MsgpackHandle)
	if err := dec.Decode(&a); err != nil {
		return err
	}

	*e = ProxyConfigEntry(a)

	config, err := lib.MapWalk(e.Config)
	if err != nil {
		return err
	}

	e.Config = config
	return nil
}

// DecodeConfigEntry can be used to decode a ConfigEntry from a raw map value.
// Currently its used in the HTTP API to decode ConfigEntry structs coming from
// JSON. Unlike some of our custom binary encodings we don't have a preamble including
// the kind so we will not have a concrete type to decode into. In those cases we must
// first decode into a map[string]interface{} and then call this function to decode
// into a concrete type.
//
// There is an 'api' variation of this in
// command/helpers/helpers.go:newDecodeConfigEntry
func DecodeConfigEntry(raw map[string]interface{}) (ConfigEntry, error) {
	var entry ConfigEntry

	kindVal, ok := raw["Kind"]
	if !ok {
		kindVal, ok = raw["kind"]
	}
	if !ok {
		return nil, fmt.Errorf("Payload does not contain a kind/Kind key at the top level")
	}

	if kindStr, ok := kindVal.(string); ok {
		newEntry, err := MakeConfigEntry(kindStr, "")
		if err != nil {
			return nil, err
		}
		entry = newEntry
	} else {
		return nil, fmt.Errorf("Kind value in payload is not a string")
	}

	var md mapstructure.Metadata
	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			decode.HookWeakDecodeFromSlice,
			decode.HookTranslateKeys,
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToTimeHookFunc(time.RFC3339),
		),
		Metadata:         &md,
		Result:           &entry,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(raw); err != nil {
		return nil, err
	}

	if err := validateUnusedKeys(md.Unused); err != nil {
		return nil, err
	}
	return entry, nil
}

type ConfigEntryOp string

const (
	ConfigEntryUpsert              ConfigEntryOp = "upsert"
	ConfigEntryUpsertCAS           ConfigEntryOp = "upsert-cas"
	ConfigEntryUpsertWithStatusCAS ConfigEntryOp = "upsert-with-status-cas"
	ConfigEntryDelete              ConfigEntryOp = "delete"
	ConfigEntryDeleteCAS           ConfigEntryOp = "delete-cas"
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
	enc := codec.NewEncoderBytes(&bs, MsgpackHandle)
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
	dec := codec.NewDecoderBytes(data, MsgpackHandle)
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
	if configEntry := makeEnterpriseConfigEntry(kind, name); configEntry != nil {
		return configEntry, nil
	}
	switch kind {
	case ServiceDefaults:
		return &ServiceConfigEntry{Name: name}, nil
	case ProxyDefaults:
		return &ProxyConfigEntry{Name: name}, nil
	case ServiceRouter:
		return &ServiceRouterConfigEntry{Name: name}, nil
	case ServiceSplitter:
		return &ServiceSplitterConfigEntry{Name: name}, nil
	case ServiceResolver:
		return &ServiceResolverConfigEntry{Name: name}, nil
	case IngressGateway:
		return &IngressGatewayConfigEntry{Name: name}, nil
	case TerminatingGateway:
		return &TerminatingGatewayConfigEntry{Name: name}, nil
	case ServiceIntentions:
		return &ServiceIntentionsConfigEntry{Name: name}, nil
	case MeshConfig:
		return &MeshConfigEntry{}, nil
	case ExportedServices:
		return &ExportedServicesConfigEntry{Name: name}, nil
	case SamenessGroup:
		return &SamenessGroupConfigEntry{Name: name}, nil
	case APIGateway:
		return &APIGatewayConfigEntry{Name: name}, nil
	case BoundAPIGateway:
		return &BoundAPIGatewayConfigEntry{Name: name}, nil
	case FileSystemCertificate:
		return &FileSystemCertificateConfigEntry{Name: name}, nil
	case InlineCertificate:
		return &InlineCertificateConfigEntry{Name: name}, nil
	case HTTPRoute:
		return &HTTPRouteConfigEntry{Name: name}, nil
	case TCPRoute:
		return &TCPRouteConfigEntry{Name: name}, nil
	case JWTProvider:
		return &JWTProviderConfigEntry{Name: name}, nil
	default:
		return nil, fmt.Errorf("invalid config entry kind: %s", kind)
	}
}

// ConfigEntryQuery is used when requesting info about a config entry.
type ConfigEntryQuery struct {
	Kind       string
	Name       string
	Datacenter string

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	QueryOptions
}

func (c *ConfigEntryQuery) RequestDatacenter() string {
	return c.Datacenter
}

func (r *ConfigEntryQuery) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	v, err := hashstructure.Hash([]interface{}{
		r.Kind,
		r.Name,
		r.Filter,
		r.EnterpriseMeta,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

// ConfigEntryListAllRequest is used when requesting to list all config entries
// of a set of kinds.
type ConfigEntryListAllRequest struct {
	// Kinds should always be set. For backwards compatibility with versions
	// prior to 1.9.0, if this is omitted or left empty it is assumed to mean
	// the subset of config entry kinds that were present in 1.8.0:
	//
	// proxy-defaults, service-defaults, service-resolver, service-splitter,
	// service-router, terminating-gateway, and ingress-gateway.
	Kinds      []string
	Datacenter string

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	QueryOptions
}

func (r *ConfigEntryListAllRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ServiceConfigRequest is used when requesting the resolved configuration
// for a service.
type ServiceConfigRequest struct {
	Name       string
	Datacenter string

	// MeshGateway contains the mesh gateway configuration from the requesting proxy's registration
	MeshGateway MeshGatewayConfig

	// Mode indicates how the requesting proxy's listeners are dialed
	Mode ProxyMode

	// UpstreamServiceNames is a list of upstream service names to use for resolving the service config.
	UpstreamServiceNames []PeeredServiceName

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	QueryOptions
}

func (s *ServiceConfigRequest) RequestDatacenter() string {
	return s.Datacenter
}

func (r *ServiceConfigRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	// To calculate the cache key we only hash the service name and upstream set.
	// We don't want ordering of the upstreams to affect the outcome so use an
	// anonymous struct field with hash:set behavior. Note the order of fields in
	// the slice would affect cache keys if we ever persist between agent restarts
	// and change it.
	v, err := hashstructure.Hash(struct {
		Name                 string
		EnterpriseMeta       acl.EnterpriseMeta
		UpstreamServiceNames []PeeredServiceName `hash:"set"`
		MeshGatewayConfig    MeshGatewayConfig
		ProxyMode            ProxyMode
		Filter               string
	}{
		Name:                 r.Name,
		EnterpriseMeta:       r.EnterpriseMeta,
		UpstreamServiceNames: r.UpstreamServiceNames,
		ProxyMode:            r.Mode,
		MeshGatewayConfig:    r.MeshGateway,
		Filter:               r.QueryOptions.Filter,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

type UpstreamConfig struct {
	// Name is only accepted within service-defaults.upstreamConfig.overrides .
	Name string `json:",omitempty"`
	// EnterpriseMeta is only accepted within service-defaults.upstreamConfig.overrides .
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	// Peer is only accepted within service-defaults.upstreamConfig.overrides .
	Peer string

	// EnvoyListenerJSON is a complete override ("escape hatch") for the upstream's
	// listener.
	//
	// Note: This escape hatch is NOT compatible with the discovery chain and
	// will be ignored if a discovery chain is active.
	EnvoyListenerJSON string `json:",omitempty" alias:"envoy_listener_json"`

	// EnvoyClusterJSON is a complete override ("escape hatch") for the upstream's
	// cluster. The Connect client TLS certificate and context will be injected
	// overriding any TLS settings present.
	//
	// Note: This escape hatch is NOT compatible with the discovery chain and
	// will be ignored if a discovery chain is active.
	EnvoyClusterJSON string `json:",omitempty" alias:"envoy_cluster_json"`

	// Protocol describes the upstream's service protocol. Valid values are "tcp",
	// "http" and "grpc". Anything else is treated as tcp. The enables protocol
	// aware features like per-request metrics and connection pooling, tracing,
	// routing etc.
	Protocol string `json:",omitempty"`

	// ConnectTimeoutMs is the number of milliseconds to timeout making a new
	// connection to this upstream. Defaults to 5000 (5 seconds) if not set.
	ConnectTimeoutMs int `json:",omitempty" alias:"connect_timeout_ms"`

	// Limits are the set of limits that are applied to the proxy for a specific upstream of a
	// service instance.
	Limits *UpstreamLimits `json:",omitempty"`

	// PassiveHealthCheck configuration determines how upstream proxy instances will
	// be monitored for removal from the load balancing pool.
	PassiveHealthCheck *PassiveHealthCheck `json:",omitempty" alias:"passive_health_check"`

	// MeshGatewayConfig controls how Mesh Gateways are configured and used
	MeshGateway MeshGatewayConfig `json:",omitempty" alias:"mesh_gateway" `

	// BalanceOutboundConnections indicates how the proxy should attempt to distribute
	// connections across worker threads. Only used by envoy proxies.
	BalanceOutboundConnections string `json:",omitempty" alias:"balance_outbound_connections"`
}

func (cfg UpstreamConfig) Clone() UpstreamConfig {
	cfg2 := cfg

	cfg2.Limits = cfg.Limits.Clone()
	cfg2.PassiveHealthCheck = cfg.PassiveHealthCheck.Clone()

	return cfg2
}

func (cfg *UpstreamConfig) PeeredServiceName() PeeredServiceName {
	if cfg.Name == "" {
		return PeeredServiceName{}
	}
	return PeeredServiceName{
		Peer:        cfg.Peer,
		ServiceName: NewServiceName(cfg.Name, &cfg.EnterpriseMeta),
	}
}

func (cfg UpstreamConfig) MergeInto(dst map[string]interface{}) {
	// Avoid storing empty values in the map, since these can act as overrides
	if cfg.EnvoyListenerJSON != "" {
		dst["envoy_listener_json"] = cfg.EnvoyListenerJSON
	}
	if cfg.EnvoyClusterJSON != "" {
		dst["envoy_cluster_json"] = cfg.EnvoyClusterJSON
	}
	if cfg.Protocol != "" {
		dst["protocol"] = cfg.Protocol
	}
	if cfg.ConnectTimeoutMs != 0 {
		dst["connect_timeout_ms"] = cfg.ConnectTimeoutMs
	}
	if !cfg.MeshGateway.IsZero() {
		dst["mesh_gateway"] = cfg.MeshGateway
	}
	if cfg.Limits != nil {
		dst["limits"] = cfg.Limits
	}
	if cfg.PassiveHealthCheck != nil {
		dst["passive_health_check"] = cfg.PassiveHealthCheck
	}
	if cfg.BalanceOutboundConnections != "" {
		dst["balance_outbound_connections"] = cfg.BalanceOutboundConnections
	}
}

func (cfg *UpstreamConfig) NormalizeWithoutName() error {
	return cfg.normalize(false, nil)
}
func (cfg *UpstreamConfig) NormalizeWithName(entMeta *acl.EnterpriseMeta) error {
	return cfg.normalize(true, entMeta)
}
func (cfg *UpstreamConfig) normalize(named bool, entMeta *acl.EnterpriseMeta) error {
	if named {
		// If the upstream namespace is omitted it inherits that of the enclosing
		// config entry.
		cfg.EnterpriseMeta.MergeNoWildcard(entMeta)
		cfg.EnterpriseMeta.Normalize()
	}

	cfg.Protocol = strings.ToLower(cfg.Protocol)

	if cfg.ConnectTimeoutMs < 0 {
		cfg.ConnectTimeoutMs = 0
	}
	return nil
}

func (cfg UpstreamConfig) ValidateWithoutName() error {
	return cfg.validate(false)
}
func (cfg UpstreamConfig) ValidateWithName() error {
	return cfg.validate(true)
}
func (cfg UpstreamConfig) validate(named bool) error {
	if named {
		if cfg.Name == "" {
			return fmt.Errorf("Name is required")
		}
		if cfg.Name == WildcardSpecifier {
			return fmt.Errorf("Wildcard name is not supported")
		}
		if cfg.EnterpriseMeta.NamespaceOrDefault() == WildcardSpecifier {
			return fmt.Errorf("Wildcard namespace is not supported")
		}
	} else {
		if cfg.Name != "" {
			return fmt.Errorf("Name must be empty")
		}
		if cfg.EnterpriseMeta.NamespaceOrEmpty() != "" {
			return fmt.Errorf("Namespace must be empty")
		}
		if cfg.EnterpriseMeta.PartitionOrEmpty() != "" {
			return fmt.Errorf("Partition must be empty")
		}
	}

	var validationErr error

	if cfg.PassiveHealthCheck != nil {
		err := cfg.PassiveHealthCheck.Validate()
		if err != nil {
			validationErr = multierror.Append(validationErr, err)
		}
	}

	if cfg.Limits != nil {
		err := cfg.Limits.Validate()
		if err != nil {
			validationErr = multierror.Append(validationErr, err)
		}
	}

	if !isValidConnectionBalance(cfg.BalanceOutboundConnections) {
		validationErr = multierror.Append(validationErr, fmt.Errorf("invalid value for balance_outbound_connections: %v", cfg.BalanceOutboundConnections))
	}

	return validationErr
}

func ParseUpstreamConfigNoDefaults(m map[string]interface{}) (UpstreamConfig, error) {
	var cfg UpstreamConfig
	config := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			decode.HookWeakDecodeFromSlice,
			decode.HookTranslateKeys,
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result:           &cfg,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return cfg, err
	}

	if err := decoder.Decode(m); err != nil {
		return cfg, err
	}

	err = cfg.NormalizeWithoutName()

	return cfg, err
}

// ParseUpstreamConfig returns the UpstreamConfig parsed from an opaque map.
// If an error occurs during parsing it is returned along with the default
// config this allows caller to choose whether and how to report the error.
func ParseUpstreamConfig(m map[string]interface{}) (UpstreamConfig, error) {
	cfg, err := ParseUpstreamConfigNoDefaults(m)

	// Set default (even if error is returned)
	if cfg.Protocol == "" {
		cfg.Protocol = "tcp"
	}
	if cfg.ConnectTimeoutMs == 0 {
		cfg.ConnectTimeoutMs = 5000
	}

	return cfg, err
}

type PassiveHealthCheck struct {
	// Interval between health check analysis sweeps. Each sweep may remove
	// hosts or return hosts to the pool.
	Interval time.Duration `json:",omitempty"`

	// MaxFailures is the count of consecutive failures that results in a host
	// being removed from the pool.
	MaxFailures uint32 `json:",omitempty" alias:"max_failures"`

	// EnforcingConsecutive5xx is the % chance that a host will be actually ejected
	// when an outlier status is detected through consecutive 5xx.
	// This setting can be used to disable ejection or to ramp it up slowly. Defaults to 100.
	EnforcingConsecutive5xx *uint32 `json:",omitempty" alias:"enforcing_consecutive_5xx"`

	// The maximum % of an upstream cluster that can be ejected due to outlier detection.
	// Defaults to 10% but will eject at least one host regardless of the value.
	// TODO: remove me
	MaxEjectionPercent *uint32 `json:",omitempty" alias:"max_ejection_percent"`

	// The base time that a host is ejected for. The real time is equal to the base time
	// multiplied by the number of times the host has been ejected and is capped by
	// max_ejection_time (Default 300s). Defaults to 30000ms or 30s.
	BaseEjectionTime *time.Duration `json:",omitempty" alias:"base_ejection_time"`
}

func (chk *PassiveHealthCheck) Clone() *PassiveHealthCheck {
	if chk == nil {
		return nil
	}
	chk2 := *chk
	return &chk2
}

func (chk *PassiveHealthCheck) IsZero() bool {
	zeroVal := PassiveHealthCheck{}
	return *chk == zeroVal
}

func (chk PassiveHealthCheck) Validate() error {
	if chk.Interval < 0*time.Second {
		return fmt.Errorf("passive health check interval cannot be negative")
	}
	if chk.EnforcingConsecutive5xx != nil && *chk.EnforcingConsecutive5xx > 100 {
		return fmt.Errorf("passive health check enforcing_consecutive_5xx must be a percentage between 0 and 100")
	}
	if chk.MaxEjectionPercent != nil && *chk.MaxEjectionPercent > 100 {
		return fmt.Errorf("passive health check max_ejection_percent must be a percentage between 0 and 100")
	}
	if chk.BaseEjectionTime != nil && *chk.BaseEjectionTime < 0*time.Second {
		return fmt.Errorf("passive health check base_ejection_time cannot be negative")
	}
	return nil
}

// UpstreamLimits describes the limits that are associated with a specific
// upstream of a service instance.
type UpstreamLimits struct {
	// MaxConnections is the maximum number of connections the local proxy can
	// make to the upstream service.
	MaxConnections *int `json:",omitempty" alias:"max_connections"`

	// MaxPendingRequests is the maximum number of requests that will be queued
	// waiting for an available connection. This is mostly applicable to HTTP/1.1
	// clusters since all HTTP/2 requests are streamed over a single
	// connection.
	MaxPendingRequests *int `json:",omitempty" alias:"max_pending_requests"`

	// MaxConcurrentRequests is the maximum number of in-flight requests that will be allowed
	// to the upstream cluster at a point in time. This is mostly applicable to HTTP/2
	// clusters since all HTTP/1.1 requests are limited by MaxConnections.
	MaxConcurrentRequests *int `json:",omitempty" alias:"max_concurrent_requests"`
}

func (ul *UpstreamLimits) Clone() *UpstreamLimits {
	if ul == nil {
		return nil
	}
	return &UpstreamLimits{
		MaxConnections:        intPointerCopy(ul.MaxConnections),
		MaxPendingRequests:    intPointerCopy(ul.MaxPendingRequests),
		MaxConcurrentRequests: intPointerCopy(ul.MaxConcurrentRequests),
	}
}

func intPointerCopy(v *int) *int {
	if v == nil {
		return nil
	}
	v2 := *v
	return &v2
}

func (ul *UpstreamLimits) IsZero() bool {
	zeroVal := UpstreamLimits{}
	return *ul == zeroVal
}

func (ul UpstreamLimits) Validate() error {
	if ul.MaxConnections != nil && *ul.MaxConnections < 0 {
		return fmt.Errorf("max connections cannot be negative")
	}
	if ul.MaxPendingRequests != nil && *ul.MaxPendingRequests < 0 {
		return fmt.Errorf("max pending requests cannot be negative")
	}
	if ul.MaxConcurrentRequests != nil && *ul.MaxConcurrentRequests < 0 {
		return fmt.Errorf("max concurrent requests cannot be negative")
	}
	return nil
}

type OpaqueUpstreamConfig struct {
	Upstream PeeredServiceName
	Config   map[string]interface{}
}
type OpaqueUpstreamConfigs []OpaqueUpstreamConfig

type ServiceConfigResponse struct {
	ProxyConfig      map[string]interface{}
	UpstreamConfigs  OpaqueUpstreamConfigs
	MeshGateway      MeshGatewayConfig      `json:",omitempty"`
	Expose           ExposeConfig           `json:",omitempty"`
	TransparentProxy TransparentProxyConfig `json:",omitempty"`
	MutualTLSMode    MutualTLSMode          `json:",omitempty"`
	Mode             ProxyMode              `json:",omitempty"`
	Destination      DestinationConfig      `json:",omitempty"`
	AccessLogs       AccessLogsConfig       `json:",omitempty"`
	RateLimits       RateLimits             `json:",omitempty"`
	Meta             map[string]string      `json:",omitempty"`
	EnvoyExtensions  []EnvoyExtension       `json:",omitempty"`
	QueryMeta
}

// MarshalBinary writes ServiceConfigResponse as msgpack encoded. It's only here
// because we need custom decoding of the raw interface{} values.
func (r *ServiceConfigResponse) MarshalBinary() (data []byte, err error) {
	// bs will grow if needed but allocate enough to avoid reallocation in common
	// case.
	bs := make([]byte, 128)
	enc := codec.NewEncoderBytes(&bs, MsgpackHandle)

	type Alias ServiceConfigResponse

	if err := enc.Encode((*Alias)(r)); err != nil {
		return nil, err
	}

	return bs, nil
}

// UnmarshalBinary decodes msgpack encoded ServiceConfigResponse. It used
// default msgpack encoding but fixes up the uint8 strings and other problems we
// have with encoding map[string]interface{}.
func (r *ServiceConfigResponse) UnmarshalBinary(data []byte) error {
	dec := codec.NewDecoderBytes(data, MsgpackHandle)

	type Alias ServiceConfigResponse
	var a Alias

	if err := dec.Decode(&a); err != nil {
		return err
	}

	*r = ServiceConfigResponse(a)

	var err error

	// Fix strings and maps in the returned maps
	r.ProxyConfig, err = lib.MapWalk(r.ProxyConfig)
	if err != nil {
		return err
	}

	for k := range r.UpstreamConfigs {
		r.UpstreamConfigs[k].Config, err = lib.MapWalk(r.UpstreamConfigs[k].Config)
		if err != nil {
			return err
		}
	}
	return nil
}

// ConfigEntryResponse returns a single ConfigEntry
type ConfigEntryResponse struct {
	Entry ConfigEntry
	QueryMeta
}

func (c *ConfigEntryResponse) MarshalBinary() (data []byte, err error) {
	// bs will grow if needed but allocate enough to avoid reallocation in common
	// case.
	bs := make([]byte, 128)
	enc := codec.NewEncoderBytes(&bs, MsgpackHandle)

	if c.Entry != nil {
		if err := enc.Encode(c.Entry.GetKind()); err != nil {
			return nil, err
		}
		if err := enc.Encode(c.Entry); err != nil {
			return nil, err
		}
	} else {
		if err := enc.Encode(""); err != nil {
			return nil, err
		}
	}

	if err := enc.Encode(c.QueryMeta); err != nil {
		return nil, err
	}

	return bs, nil
}

func (c *ConfigEntryResponse) UnmarshalBinary(data []byte) error {
	dec := codec.NewDecoderBytes(data, MsgpackHandle)

	var kind string
	if err := dec.Decode(&kind); err != nil {
		return err
	}

	if kind != "" {
		entry, err := MakeConfigEntry(kind, "")
		if err != nil {
			return err
		}

		if err := dec.Decode(entry); err != nil {
			return err
		}
		c.Entry = entry
	} else {
		c.Entry = nil
	}

	if err := dec.Decode(&c.QueryMeta); err != nil {
		return err
	}

	return nil
}

func validateOpaqueProxyConfig(config map[string]interface{}) error {
	// This max is chosen to stay under the 104 character limit on OpenBSD, FreeBSD, MacOS.
	// It assumes the socket's filename is fixed at 32 characters.
	const maxSocketDirLen = 70

	if path, _ := config["envoy_hcp_metrics_bind_socket_dir"].(string); len(path) > maxSocketDirLen {
		return fmt.Errorf("envoy_hcp_metrics_bind_socket_dir length %d exceeds max %d", len(path), maxSocketDirLen)
	}
	return nil
}

func validateConfigEntryMeta(meta map[string]string) error {
	var err error
	if len(meta) > metaMaxKeyPairs {
		err = multierror.Append(err, fmt.Errorf(
			"Meta exceeds maximum element count %d", metaMaxKeyPairs))
	}
	for k, v := range meta {
		if len(k) > metaKeyMaxLength {
			err = multierror.Append(err, fmt.Errorf(
				"Meta key %q exceeds maximum length %d", k, metaKeyMaxLength))
		}
		if len(v) > metaValueMaxLength {
			err = multierror.Append(err, fmt.Errorf(
				"Meta value for key %q exceeds maximum length %d", k, metaValueMaxLength))
		}
	}
	return err
}

type ConfigEntryDeleteResponse struct {
	Deleted bool
}

func isValidConnectionBalance(s string) bool {
	return s == "" || s == ConnectionExactBalance
}
