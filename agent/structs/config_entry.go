package structs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/hashstructure"
	"github.com/mitchellh/mapstructure"
)

const (
	ServiceDefaults string = "service-defaults"
	ProxyDefaults   string = "proxy-defaults"
	ServiceRouter   string = "service-router"
	ServiceSplitter string = "service-splitter"
	ServiceResolver string = "service-resolver"

	ProxyConfigGlobal string = "global"

	DefaultServiceProtocol = "tcp"
)

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
	CanRead(acl.Authorizer) bool
	CanWrite(acl.Authorizer) bool

	GetRaftIndex() *RaftIndex
}

// ServiceConfiguration is the top-level struct for the configuration of a service
// across the entire cluster.
type ServiceConfigEntry struct {
	Kind        string
	Name        string
	Protocol    string
	MeshGateway MeshGatewayConfig `json:",omitempty"`

	ExternalSNI string `json:",omitempty"`

	// TODO(banks): enable this once we have upstreams supported too. Enabling
	// sidecars actually makes no sense and adds complications when you don't
	// allow upstreams to be specified centrally too.
	//
	// Connect ConnectConfiguration

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
	e.Protocol = strings.ToLower(e.Protocol)

	return nil
}

func (e *ServiceConfigEntry) Validate() error {
	return nil
}

func (e *ServiceConfigEntry) CanRead(rule acl.Authorizer) bool {
	return rule.ServiceRead(e.Name)
}

func (e *ServiceConfigEntry) CanWrite(rule acl.Authorizer) bool {
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
	Kind        string
	Name        string
	Config      map[string]interface{}
	MeshGateway MeshGatewayConfig `json:",omitempty"`

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

func (e *ProxyConfigEntry) CanRead(rule acl.Authorizer) bool {
	return true
}

func (e *ProxyConfigEntry) CanWrite(rule acl.Authorizer) bool {
	return rule.OperatorWrite()
}

func (e *ProxyConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
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
	enc := codec.NewEncoderBytes(&bs, msgpackHandle)
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
	dec := codec.NewDecoderBytes(data, msgpackHandle)
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
// command/config/write/config_write.go:newDecodeConfigEntry
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

	skipWhenPatching, translateKeysDict, err := ConfigEntryDecodeRulesForKind(entry.GetKind())
	if err != nil {
		return nil, err
	}

	// lib.TranslateKeys doesn't understand []map[string]interface{} so we have
	// to do this part first.
	raw = lib.PatchSliceOfMaps(raw, skipWhenPatching, nil)

	lib.TranslateKeys(raw, translateKeysDict)

	var md mapstructure.Metadata
	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
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

	for _, k := range md.Unused {
		switch k {
		case "CreateIndex", "ModifyIndex":
		default:
			err = multierror.Append(err, fmt.Errorf("invalid config key %q", k))
		}
	}
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// ConfigEntryDecodeRulesForKind returns rules for 'fixing' config entry key
// formats by kind. This is shared between the 'structs' and 'api' variations
// of config entries.
func ConfigEntryDecodeRulesForKind(kind string) (skipWhenPatching []string, translateKeysDict map[string]string, err error) {
	switch kind {
	case ProxyDefaults:
		return nil, map[string]string{
			"mesh_gateway": "meshgateway",
			"config":       "",
		}, nil
	case ServiceDefaults:
		return nil, map[string]string{
			"mesh_gateway": "meshgateway",
			"external_sni": "externalsni",
		}, nil
	case ServiceRouter:
		return []string{
				"routes",
				"Routes",
				"routes.match.http.header",
				"Routes.Match.HTTP.Header",
				"routes.match.http.query_param",
				"Routes.Match.HTTP.QueryParam",
			}, map[string]string{
				"num_retries":              "numretries",
				"path_exact":               "pathexact",
				"path_prefix":              "pathprefix",
				"path_regex":               "pathregex",
				"prefix_rewrite":           "prefixrewrite",
				"query_param":              "queryparam",
				"request_timeout":          "requesttimeout",
				"retry_on_connect_failure": "retryonconnectfailure",
				"retry_on_status_codes":    "retryonstatuscodes",
				"service_subset":           "servicesubset",
			}, nil
	case ServiceSplitter:
		return []string{
				"splits",
				"Splits",
			}, map[string]string{
				"service_subset": "servicesubset",
			}, nil
	case ServiceResolver:
		return nil, map[string]string{
			"connect_timeout": "connecttimeout",
			"default_subset":  "defaultsubset",
			"only_passing":    "onlypassing",
			"service_subset":  "servicesubset",
		}, nil
	default:
		return nil, nil, fmt.Errorf("kind %q should be explicitly handled here", kind)
	}
}

type ConfigEntryOp string

const (
	ConfigEntryUpsert    ConfigEntryOp = "upsert"
	ConfigEntryUpsertCAS ConfigEntryOp = "upsert-cas"
	ConfigEntryDelete    ConfigEntryOp = "delete"
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
	case ServiceRouter:
		return &ServiceRouterConfigEntry{Name: name}, nil
	case ServiceSplitter:
		return &ServiceSplitterConfigEntry{Name: name}, nil
	case ServiceResolver:
		return &ServiceResolverConfigEntry{Name: name}, nil
	default:
		return nil, fmt.Errorf("invalid config entry kind: %s", kind)
	}
}

func ValidateConfigEntryKind(kind string) bool {
	switch kind {
	case ServiceDefaults, ProxyDefaults:
		return true
	case ServiceRouter, ServiceSplitter, ServiceResolver:
		return true
	default:
		return false
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
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

// ServiceConfigRequest is used when requesting the resolved configuration
// for a service.
type ServiceConfigRequest struct {
	Name       string
	Datacenter string
	Upstreams  []string

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
		Name      string
		Upstreams []string `hash:"set"`
	}{
		Name:      r.Name,
		Upstreams: r.Upstreams,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

type ServiceConfigResponse struct {
	ProxyConfig     map[string]interface{}
	UpstreamConfigs map[string]map[string]interface{}
	MeshGateway     MeshGatewayConfig `json:",omitempty"`
	QueryMeta
}

func (r *ServiceConfigResponse) Reset() {
	r.ProxyConfig = nil
	r.UpstreamConfigs = nil
	r.MeshGateway = MeshGatewayConfig{}
}

// MarshalBinary writes ServiceConfigResponse as msgpack encoded. It's only here
// because we need custom decoding of the raw interface{} values.
func (r *ServiceConfigResponse) MarshalBinary() (data []byte, err error) {
	// bs will grow if needed but allocate enough to avoid reallocation in common
	// case.
	bs := make([]byte, 128)
	enc := codec.NewEncoderBytes(&bs, msgpackHandle)

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
	dec := codec.NewDecoderBytes(data, msgpackHandle)

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
		r.UpstreamConfigs[k], err = lib.MapWalk(r.UpstreamConfigs[k])
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
	enc := codec.NewEncoderBytes(&bs, msgpackHandle)

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
	dec := codec.NewDecoderBytes(data, msgpackHandle)

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

// ConfigEntryKindName is a value type useful for maps. You can use:
//     map[ConfigEntryKindName]Payload
// instead of:
//     map[string]map[string]Payload
type ConfigEntryKindName struct {
	Kind string
	Name string
}
