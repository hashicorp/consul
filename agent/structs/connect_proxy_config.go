package structs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
)

const (
	defaultExposeProtocol = "http"
)

var allowedExposeProtocols = map[string]bool{"http": true, "http2": true}

type MeshGatewayMode string

const (
	// MeshGatewayModeDefault represents no specific mode and should
	// be used to indicate that a different layer of the configuration
	// chain should take precedence
	MeshGatewayModeDefault MeshGatewayMode = ""

	// MeshGatewayModeNone represents that the Upstream Connect connections
	// should be direct and not flow through a mesh gateway.
	MeshGatewayModeNone MeshGatewayMode = "none"

	// MeshGatewayModeLocal represents that the Upstream Connect connections
	// should be made to a mesh gateway in the local datacenter.
	MeshGatewayModeLocal MeshGatewayMode = "local"

	// MeshGatewayModeRemote represents that the Upstream Connect connections
	// should be made to a mesh gateway in a remote datacenter.
	MeshGatewayModeRemote MeshGatewayMode = "remote"
)

type LogSinkType string

const (
	DefaultLogSinkType LogSinkType = ""
	FileLogSinkType    LogSinkType = "file"
	StdErrLogSinkType  LogSinkType = "stderr"
	StdOutLogSinkType  LogSinkType = "stdout"
)

const (
	// TODO (freddy) Should we have a TopologySourceMixed when there is a mix of proxy reg and tproxy?
	//				 Currently we label as proxy-registration if ANY instance has the explicit upstream definition.
	// TopologySourceRegistration is used to label upstreams or downstreams from explicit upstream definitions.
	TopologySourceRegistration = "proxy-registration"

	// TopologySourceSpecificIntention is used to label upstreams or downstreams from specific intentions.
	TopologySourceSpecificIntention = "specific-intention"

	// TopologySourceWildcardIntention is used to label upstreams or downstreams from wildcard intentions.
	TopologySourceWildcardIntention = "wildcard-intention"

	// TopologySourceDefaultAllow is used to label upstreams or downstreams from default allow ACL policy.
	TopologySourceDefaultAllow = "default-allow"

	// TopologySourceRoutingConfig is used to label upstreams that are not backed by a service instance
	// and are simply used for routing configurations.
	TopologySourceRoutingConfig = "routing-config"
)

// MeshGatewayConfig controls how Mesh Gateways are configured and used
// This is a struct to allow for future additions without having more free-hanging
// configuration items all over the place
type MeshGatewayConfig struct {
	// The Mesh Gateway routing mode
	Mode MeshGatewayMode `json:",omitempty"`
}

func (c *MeshGatewayConfig) IsZero() bool {
	zeroVal := MeshGatewayConfig{}
	return *c == zeroVal
}

func ValidateMeshGatewayMode(mode string) (MeshGatewayMode, error) {
	switch MeshGatewayMode(mode) {
	case MeshGatewayModeNone:
		return MeshGatewayModeNone, nil
	case MeshGatewayModeDefault:
		return MeshGatewayModeDefault, nil
	case MeshGatewayModeLocal:
		return MeshGatewayModeLocal, nil
	case MeshGatewayModeRemote:
		return MeshGatewayModeRemote, nil
	default:
		return MeshGatewayModeDefault, fmt.Errorf("Invalid Mesh Gateway Mode: %q", mode)
	}
}

func (c *MeshGatewayConfig) ToAPI() api.MeshGatewayConfig {
	return api.MeshGatewayConfig{Mode: api.MeshGatewayMode(c.Mode)}
}

type ProxyMode string

const (
	// ProxyModeDefault represents no specific mode and should
	// be used to indicate that a different layer of the configuration
	// chain should take precedence
	ProxyModeDefault ProxyMode = ""

	// ProxyModeTransparent represents that inbound and outbound application
	// traffic is being captured and redirected through the proxy.
	ProxyModeTransparent ProxyMode = "transparent"

	// ProxyModeDirect represents that the proxy's listeners must be dialed directly
	// by the local application and other proxies.
	ProxyModeDirect ProxyMode = "direct"
)

func ValidateProxyMode(mode string) (ProxyMode, error) {
	switch ProxyMode(mode) {
	case ProxyModeDefault:
		return ProxyModeDefault, nil
	case ProxyModeDirect:
		return ProxyModeDirect, nil
	case ProxyModeTransparent:
		return ProxyModeTransparent, nil
	default:
		return ProxyModeDefault, fmt.Errorf("Invalid Proxy Mode: %q", mode)
	}
}

type TransparentProxyConfig struct {
	// The port of the listener where outbound application traffic is being redirected to.
	OutboundListenerPort int `json:",omitempty" alias:"outbound_listener_port"`

	// DialedDirectly indicates whether transparent proxies can dial this proxy instance directly.
	// The discovery chain is not considered when dialing a service instance directly.
	// This setting is useful when addressing stateful services, such as a database cluster with a leader node.
	DialedDirectly bool `json:",omitempty" alias:"dialed_directly"`
}

func (c TransparentProxyConfig) ToAPI() *api.TransparentProxyConfig {
	if c.IsZero() {
		return nil
	}
	return &api.TransparentProxyConfig{
		OutboundListenerPort: c.OutboundListenerPort,
		DialedDirectly:       c.DialedDirectly,
	}
}

func (c *TransparentProxyConfig) IsZero() bool {
	if c == nil {
		return true
	}
	zeroVal := TransparentProxyConfig{}
	return *c == zeroVal
}

// AccessLogsConfig contains the associated default settings for all Envoy instances within the datacenter or partition
type AccessLogsConfig struct {
	// Enabled turns off all access logging
	Enabled bool `json:",omitempty" alias:"enabled"`

	// DisableListenerLogs turns off just listener logs for connections rejected by Envoy because they don't
	// have a matching listener filter.
	DisableListenerLogs bool `json:",omitempty" alias:"disable_listener_logs"`

	// Type selects the output for logs: "file", "stderr". "stdout"
	Type LogSinkType `json:",omitempty" alias:"type"`

	// Path is the output file to write logs
	Path string `json:",omitempty" alias:"path"`

	// The presence of one format string or the other implies the access log string encoding.
	// Defining Both is invalid.
	JSONFormat string `json:",omitempty" alias:"json_format"`
	TextFormat string `json:",omitempty" alias:"text_format"`
}

func (c *AccessLogsConfig) IsZero() bool {
	if c == nil {
		return true
	}
	zeroVal := AccessLogsConfig{}
	return *c == zeroVal
}

func (c *AccessLogsConfig) ToAPI() *api.AccessLogsConfig {
	if c.IsZero() {
		return nil
	}
	return &api.AccessLogsConfig{
		Enabled:             c.Enabled,
		DisableListenerLogs: c.DisableListenerLogs,
		Type:                api.LogSinkType(c.Type),
		Path:                c.Path,
		JSONFormat:          c.JSONFormat,
		TextFormat:          c.TextFormat,
	}
}

func (c *AccessLogsConfig) Validate() error {
	switch c.Type {
	case DefaultLogSinkType, StdErrLogSinkType, StdOutLogSinkType:
		// OK
	case FileLogSinkType:
		if c.Path == "" {
			return errors.New("path must be specified when using file type access logs")
		}
	default:
		return fmt.Errorf("invalid access log type: %s", c.Type)
	}

	if c.JSONFormat != "" && c.TextFormat != "" {
		return errors.New("cannot specify both access log JSONFormat and TextFormat")
	}

	if c.Type != FileLogSinkType && c.Path != "" {
		return errors.New("path is only valid for file type access logs")
	}

	if c.JSONFormat != "" {
		msg := json.RawMessage{}
		if err := json.Unmarshal([]byte(c.JSONFormat), &msg); err != nil {
			return fmt.Errorf("invalid access log json for JSON format: %w", err)
		}
	}
	return nil
}

// ConnectProxyConfig describes the configuration needed for any proxy managed
// or unmanaged. It describes a single logical service's listener and optionally
// upstreams and sidecar-related config for a single instance. To describe a
// centralized proxy that routed traffic for multiple services, a different one
// of these would be needed for each, sharing the same LogicalProxyID.
type ConnectProxyConfig struct {
	// EnvoyExtensions are the list of Envoy extensions configured for the local service.
	EnvoyExtensions []EnvoyExtension `json:",omitempty" alias:"envoy_extensions"`

	// DestinationServiceName is required and is the name of the service to accept
	// traffic for.
	DestinationServiceName string `json:",omitempty" alias:"destination_service_name"`

	// DestinationServiceID is optional and should only be specified for
	// "side-car" style proxies where the proxy is in front of just a single
	// instance of the service. It should be set to the service ID of the instance
	// being represented which must be registered to the same agent. It's valid to
	// provide a service ID that does not yet exist to avoid timing issues when
	// bootstrapping a service with a proxy.
	DestinationServiceID string `json:",omitempty" alias:"destination_service_id"`

	// LocalServiceAddress is the address of the local service instance. It is
	// optional and should only be specified for "side-car" style proxies. It will
	// default to 127.0.0.1 if the proxy is a "side-car" (DestinationServiceID is
	// set) but otherwise will be ignored.
	LocalServiceAddress string `json:",omitempty" alias:"local_service_address"`

	// LocalServicePort is the port of the local service instance. It is optional
	// and should only be specified for "side-car" style proxies. It will default
	// to the registered port for the instance if the proxy is a "side-car"
	// (DestinationServiceID is set) but otherwise will be ignored.
	LocalServicePort int `json:",omitempty" alias:"local_service_port"`

	// LocalServiceSocketPath is the socket of the local service instance. It is optional
	// and should only be specified for "side-car" style proxies.
	LocalServiceSocketPath string `json:",omitempty" alias:"local_service_socket_path"`

	// Mode represents how the proxy's inbound and upstream listeners are dialed.
	Mode ProxyMode

	// Config is the arbitrary configuration data provided with the proxy
	// registration.
	Config map[string]interface{} `json:",omitempty" bexpr:"-"`

	// Upstreams describes any upstream dependencies the proxy instance should
	// setup.
	Upstreams Upstreams `json:",omitempty"`

	// MeshGateway defines the mesh gateway configuration for this upstream
	MeshGateway MeshGatewayConfig `json:",omitempty" alias:"mesh_gateway"`

	// Expose defines whether checks or paths are exposed through the proxy
	Expose ExposeConfig `json:",omitempty"`

	// TransparentProxy defines configuration for when the proxy is in
	// transparent mode.
	TransparentProxy TransparentProxyConfig `json:",omitempty" alias:"transparent_proxy"`

	// AccessLogs configures the output and format of Envoy access logs
	AccessLogs AccessLogsConfig `json:",omitempty" alias:"access_logs"`
}

func (t *ConnectProxyConfig) UnmarshalJSON(data []byte) (err error) {
	type Alias ConnectProxyConfig
	aux := &struct {
		DestinationServiceNameSnake string                 `json:"destination_service_name"`
		DestinationServiceIDSnake   string                 `json:"destination_service_id"`
		LocalServiceAddressSnake    string                 `json:"local_service_address"`
		LocalServicePortSnake       int                    `json:"local_service_port"`
		LocalServiceSocketPathSnake string                 `json:"local_service_socket_path"`
		MeshGatewaySnake            MeshGatewayConfig      `json:"mesh_gateway"`
		TransparentProxySnake       TransparentProxyConfig `json:"transparent_proxy"`
		AccessLogsSnake             AccessLogsConfig       `json:"access_logs"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if t.DestinationServiceName == "" {
		t.DestinationServiceName = aux.DestinationServiceNameSnake
	}
	if t.DestinationServiceID == "" {
		t.DestinationServiceID = aux.DestinationServiceIDSnake
	}
	if t.LocalServiceAddress == "" {
		t.LocalServiceAddress = aux.LocalServiceAddressSnake
	}
	if t.LocalServicePort == 0 {
		t.LocalServicePort = aux.LocalServicePortSnake
	}
	if t.LocalServiceSocketPath == "" {
		t.LocalServiceSocketPath = aux.LocalServiceSocketPathSnake
	}
	if t.MeshGateway.Mode == "" {
		t.MeshGateway.Mode = aux.MeshGatewaySnake.Mode
	}
	if t.TransparentProxy.OutboundListenerPort == 0 {
		t.TransparentProxy.OutboundListenerPort = aux.TransparentProxySnake.OutboundListenerPort
	}
	if !t.TransparentProxy.DialedDirectly {
		t.TransparentProxy.DialedDirectly = aux.TransparentProxySnake.DialedDirectly
	}
	if !t.AccessLogs.Enabled {
		t.AccessLogs.Enabled = aux.AccessLogsSnake.Enabled
	}
	if !t.AccessLogs.DisableListenerLogs {
		t.AccessLogs.DisableListenerLogs = aux.AccessLogsSnake.DisableListenerLogs
	}
	if t.AccessLogs.Type == "" {
		t.AccessLogs.Type = aux.AccessLogsSnake.Type
	}
	if t.AccessLogs.Path == "" {
		t.AccessLogs.Path = aux.AccessLogsSnake.Path
	}
	if t.AccessLogs.JSONFormat == "" {
		t.AccessLogs.JSONFormat = aux.AccessLogsSnake.JSONFormat
	}
	if t.AccessLogs.TextFormat == "" {
		t.AccessLogs.TextFormat = aux.AccessLogsSnake.TextFormat
	}
	return nil
}

func (c *ConnectProxyConfig) MarshalJSON() ([]byte, error) {
	type Alias ConnectProxyConfig
	out := struct {
		TransparentProxy *TransparentProxyConfig `json:",omitempty"`
		AccessLogs       *AccessLogsConfig       `json:",omitempty"`
		Alias
	}{
		Alias: (Alias)(*c),
	}

	proxyConfig, err := lib.MapWalk(c.Config)
	if err != nil {
		return nil, err
	}
	out.Alias.Config = proxyConfig

	if !c.TransparentProxy.IsZero() {
		out.TransparentProxy = &out.Alias.TransparentProxy
	}

	if !c.AccessLogs.IsZero() {
		out.AccessLogs = &out.Alias.AccessLogs
	}

	return json.Marshal(&out)
}

// ToAPI returns the api struct with the same fields. We have duplicates to
// avoid the api package depending on this one which imports a ton of Consul's
// core which you don't want if you are just trying to use our client in your
// app.
func (c *ConnectProxyConfig) ToAPI() *api.AgentServiceConnectProxyConfig {
	return &api.AgentServiceConnectProxyConfig{
		DestinationServiceName: c.DestinationServiceName,
		DestinationServiceID:   c.DestinationServiceID,
		LocalServiceAddress:    c.LocalServiceAddress,
		LocalServicePort:       c.LocalServicePort,
		LocalServiceSocketPath: c.LocalServiceSocketPath,
		Mode:                   api.ProxyMode(c.Mode),
		TransparentProxy:       c.TransparentProxy.ToAPI(),
		Config:                 c.Config,
		Upstreams:              c.Upstreams.ToAPI(),
		MeshGateway:            c.MeshGateway.ToAPI(),
		Expose:                 c.Expose.ToAPI(),
		AccessLogs:             c.AccessLogs.ToAPI(),
	}
}

const (
	UpstreamDestTypeService       = "service"
	UpstreamDestTypePreparedQuery = "prepared_query"
)

// Upstreams is a list of upstreams. Aliased to allow ToAPI method.
type Upstreams []Upstream

// ToAPI returns the api structs with the same fields. We have duplicates to
// avoid the api package depending on this one which imports a ton of Consul's
// core which you don't want if you are just trying to use our client in your
// app.
func (us Upstreams) ToAPI() []api.Upstream {
	a := make([]api.Upstream, len(us))
	for i, u := range us {
		a[i] = u.ToAPI()
	}
	return a
}

// UpstreamsFromAPI is a helper for converting api.Upstream to Upstream.
func UpstreamsFromAPI(us []api.Upstream) Upstreams {
	a := make([]Upstream, len(us))
	for i, u := range us {
		a[i] = UpstreamFromAPI(u)
	}
	return a
}

// Upstream represents a single upstream dependency for a service or proxy. It
// describes the mechanism used to discover instances to communicate with (the
// Target) as well as any potential client configuration that may be useful such
// as load balancer options, timeouts etc.
type Upstream struct {
	// Destination fields are the required ones for determining what this upstream
	// points to. Depending on DestinationType some other fields below might
	// further restrict the set of instances allowable.
	//
	// DestinationType would be better as an int constant but even with custom
	// JSON marshallers it causes havoc with all the mapstructure mangling we do
	// on service definitions in various places.
	DestinationType      string `alias:"destination_type"`
	DestinationNamespace string `json:",omitempty" alias:"destination_namespace"`
	DestinationPartition string `json:",omitempty" alias:"destination_partition"`
	DestinationPeer      string `json:",omitempty" alias:"destination_peer"`
	DestinationName      string `alias:"destination_name"`

	// Datacenter that the service discovery request should be run against. Note
	// for prepared queries, the actual results might be from a different
	// datacenter.
	Datacenter string

	// LocalBindAddress is the ip address a side-car proxy should listen on for
	// traffic destined for this upstream service. Default if empty is 127.0.0.1.
	LocalBindAddress string `json:",omitempty" alias:"local_bind_address"`

	// LocalBindPort is the ip address a side-car proxy should listen on for traffic
	// destined for this upstream service. Required.
	LocalBindPort int `json:",omitempty" alias:"local_bind_port"`

	// These are exclusive with LocalBindAddress/LocalBindPort
	LocalBindSocketPath string `json:",omitempty" alias:"local_bind_socket_path"`
	// This might be represented as an int, but because it's octal outputs can be a bit strange.
	LocalBindSocketMode string `json:",omitempty" alias:"local_bind_socket_mode"`

	// Config is an opaque config that is specific to the proxy process being run.
	// It can be used to pass arbitrary configuration for this specific upstream
	// to the proxy.
	Config map[string]interface{} `json:",omitempty" bexpr:"-"`

	// MeshGateway is the configuration for mesh gateway usage of this upstream
	MeshGateway MeshGatewayConfig `json:",omitempty" alias:"mesh_gateway"`

	// IngressHosts are a list of hosts that should route to this upstream from an
	// ingress gateway. This cannot and should not be set by a user, it is used
	// internally to store the association of hosts to an upstream service.
	// TODO(banks): we shouldn't need this any more now we pass through full
	// listener config in the ingress snapshot.
	IngressHosts []string `json:"-" bexpr:"-"`

	// CentrallyConfigured indicates whether the upstream was defined in a proxy
	// instance registration or whether it was generated from a config entry.
	CentrallyConfigured bool `json:",omitempty" bexpr:"-"`
}

func (t *Upstream) UnmarshalJSON(data []byte) (err error) {
	type Alias Upstream
	aux := &struct {
		DestinationTypeSnake      string `json:"destination_type"`
		DestinationPartitionSnake string `json:"destination_partition"`
		DestinationNamespaceSnake string `json:"destination_namespace"`
		DestinationPeerSnake      string `json:"destination_peer"`
		DestinationNameSnake      string `json:"destination_name"`

		LocalBindAddressSnake string `json:"local_bind_address"`
		LocalBindPortSnake    int    `json:"local_bind_port"`

		LocalBindSocketPathSnake string `json:"local_bind_socket_path"`
		LocalBindSocketModeSnake string `json:"local_bind_socket_mode"`

		MeshGatewaySnake MeshGatewayConfig `json:"mesh_gateway"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if t.DestinationType == "" {
		t.DestinationType = aux.DestinationTypeSnake
	}
	if t.DestinationNamespace == "" {
		t.DestinationNamespace = aux.DestinationNamespaceSnake
	}
	if t.DestinationPartition == "" {
		t.DestinationPartition = aux.DestinationPartitionSnake
	}
	if t.DestinationPeer == "" {
		t.DestinationPeer = aux.DestinationPeerSnake
	}
	if t.DestinationName == "" {
		t.DestinationName = aux.DestinationNameSnake
	}
	if t.LocalBindAddress == "" {
		t.LocalBindAddress = aux.LocalBindAddressSnake
	}
	if t.LocalBindPort == 0 {
		t.LocalBindPort = aux.LocalBindPortSnake
	}
	if t.LocalBindSocketPath == "" {
		t.LocalBindSocketPath = aux.LocalBindSocketPathSnake
	}
	if t.LocalBindSocketMode == "" {
		t.LocalBindSocketMode = aux.LocalBindSocketModeSnake
	}
	if t.MeshGateway.Mode == "" {
		t.MeshGateway.Mode = aux.MeshGatewaySnake.Mode
	}

	return nil
}

// Validate sanity checks the struct is valid
func (u *Upstream) Validate() error {
	switch u.DestinationType {
	case UpstreamDestTypePreparedQuery:
	case UpstreamDestTypeService, "":
	default:
		return fmt.Errorf("unknown upstream destination type: %q", u.DestinationType)
	}

	if u.DestinationName == "" {
		return fmt.Errorf("upstream destination name cannot be empty")
	}
	if u.DestinationName == WildcardSpecifier && !u.CentrallyConfigured {
		return fmt.Errorf("upstream destination name cannot be a wildcard")
	}
	if u.DestinationPeer != "" && u.Datacenter != "" {
		return fmt.Errorf("upstream cannot specify both destination peer and datacenter")
	}

	if u.LocalBindPort == 0 && u.LocalBindSocketPath == "" && !u.CentrallyConfigured {
		return fmt.Errorf("upstream local bind port or local socket path must be defined and nonzero")
	}
	if u.LocalBindPort != 0 && u.LocalBindSocketPath != "" && !u.CentrallyConfigured {
		return fmt.Errorf("only one of upstream local bind port or local socket path can be defined and nonzero")
	}

	return nil
}

// ToAPI returns the api structs with the same fields. We have duplicates to
// avoid the api package depending on this one which imports a ton of Consul's
// core which you don't want if you are just trying to use our client in your
// app.
func (u *Upstream) ToAPI() api.Upstream {
	return api.Upstream{
		DestinationType:      api.UpstreamDestType(u.DestinationType),
		DestinationNamespace: u.DestinationNamespace,
		DestinationPartition: u.DestinationPartition,
		DestinationPeer:      u.DestinationPeer,
		DestinationName:      u.DestinationName,
		Datacenter:           u.Datacenter,
		LocalBindAddress:     u.LocalBindAddress,
		LocalBindPort:        u.LocalBindPort,
		LocalBindSocketPath:  u.LocalBindSocketPath,
		LocalBindSocketMode:  u.LocalBindSocketMode,
		Config:               u.Config,
		MeshGateway:          u.MeshGateway.ToAPI(),
	}
}

// ToKey returns a value-type representation that uniquely identifies the
// upstream in a canonical way. Set and unset values are deliberately handled
// differently.
//
// These fields should be user-specified explicit values and not inferred
// values.
func (u *Upstream) ToKey() UpstreamKey {
	return UpstreamKey{
		DestinationType:      u.DestinationType,
		DestinationPartition: u.DestinationPartition,
		DestinationNamespace: u.DestinationNamespace,
		DestinationPeer:      u.DestinationPeer,
		DestinationName:      u.DestinationName,
		Datacenter:           u.Datacenter,
	}
}

func (u *Upstream) HasLocalPortOrSocket() bool {
	if u == nil {
		return false
	}
	return (u.LocalBindPort != 0 || u.LocalBindSocketPath != "")
}

func (u *Upstream) UpstreamIsUnixSocket() bool {
	if u == nil {
		return false
	}
	return (u.LocalBindPort == 0 && u.LocalBindAddress == "" && u.LocalBindSocketPath != "")
}

func (u *Upstream) UpstreamAddressToString() string {
	if u == nil {
		return ""
	}
	if u.UpstreamIsUnixSocket() {
		return u.LocalBindSocketPath
	}

	addr := u.LocalBindAddress
	if addr == "" {
		addr = "127.0.0.1"
	}
	return net.JoinHostPort(addr, fmt.Sprintf("%d", u.LocalBindPort))
}

type UpstreamKey struct {
	DestinationType      string
	DestinationName      string
	DestinationPartition string
	DestinationNamespace string
	DestinationPeer      string
	Datacenter           string
}

func (k UpstreamKey) String() string {
	return fmt.Sprintf(
		"[type=%q, name=%q, partition=%q, namespace=%q, peer=%q, datacenter=%q]",
		k.DestinationType,
		k.DestinationName,
		k.DestinationPartition,
		k.DestinationNamespace,
		k.DestinationPeer,
		k.Datacenter,
	)
}

// String returns a representation of this upstream suitable for debugging
// purposes but nothing relies upon this format.
func (us *Upstream) String() string {
	name := us.enterpriseStringPrefix() + us.DestinationName
	typ := us.DestinationType

	if us.DestinationPeer != "" {
		name += "?peer=" + us.DestinationPeer
	} else if us.Datacenter != "" {
		name += "?dc=" + us.Datacenter
	}

	// Service is default type so never prefix it.
	if typ == "" || typ == UpstreamDestTypeService {
		return name
	}
	return typ + ":" + name
}

// UpstreamFromAPI is a helper for converting api.Upstream to Upstream.
func UpstreamFromAPI(u api.Upstream) Upstream {
	return Upstream{
		DestinationType:      string(u.DestinationType),
		DestinationPartition: u.DestinationPartition,
		DestinationNamespace: u.DestinationNamespace,
		DestinationPeer:      u.DestinationPeer,
		DestinationName:      u.DestinationName,
		Datacenter:           u.Datacenter,
		LocalBindAddress:     u.LocalBindAddress,
		LocalBindPort:        u.LocalBindPort,
		LocalBindSocketPath:  u.LocalBindSocketPath,
		LocalBindSocketMode:  u.LocalBindSocketMode,
		Config:               u.Config,
	}
}

// ExposeConfig describes HTTP paths to expose through Envoy outside of Connect.
// Users can expose individual paths and/or all HTTP/GRPC paths for checks.
type ExposeConfig struct {
	// Checks defines whether paths associated with Consul checks will be exposed.
	// This flag triggers exposing all HTTP and GRPC check paths registered for the service.
	Checks bool `json:",omitempty"`

	// Paths is the list of paths exposed through the proxy.
	Paths []ExposePath `json:",omitempty"`
}

func (e ExposeConfig) Clone() ExposeConfig {
	e2 := e
	if len(e.Paths) > 0 {
		e2.Paths = make([]ExposePath, 0, len(e.Paths))
		for _, p := range e.Paths {
			e2.Paths = append(e2.Paths, p)
		}
	}
	return e2
}

type ExposePath struct {
	// ListenerPort defines the port of the proxy's listener for exposed paths.
	ListenerPort int `json:",omitempty" alias:"listener_port"`

	// Path is the path to expose through the proxy, ie. "/metrics."
	Path string `json:",omitempty"`

	// LocalPathPort is the port that the service is listening on for the given path.
	LocalPathPort int `json:",omitempty" alias:"local_path_port"`

	// Protocol describes the upstream's service protocol.
	// Valid values are "http" and "http2", defaults to "http"
	Protocol string `json:",omitempty"`

	// ParsedFromCheck is set if this path was parsed from a registered check
	ParsedFromCheck bool `json:",omitempty" alias:"parsed_from_check"`
}

func (t *ExposePath) UnmarshalJSON(data []byte) (err error) {
	type Alias ExposePath
	aux := &struct {
		ListenerPortSnake    int  `json:"listener_port"`
		LocalPathPortSnake   int  `json:"local_path_port"`
		ParsedFromCheckSnake bool `json:"parsed_from_check"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if t.LocalPathPort == 0 {
		t.LocalPathPort = aux.LocalPathPortSnake
	}
	if t.ListenerPort == 0 {
		t.ListenerPort = aux.ListenerPortSnake
	}
	if aux.ParsedFromCheckSnake {
		t.ParsedFromCheck = true
	}

	return nil
}

func (e *ExposeConfig) ToAPI() api.ExposeConfig {
	paths := make([]api.ExposePath, 0)
	for _, p := range e.Paths {
		paths = append(paths, p.ToAPI())
	}
	if e.Paths == nil {
		paths = nil
	}

	return api.ExposeConfig{
		Checks: e.Checks,
		Paths:  paths,
	}
}

func (p *ExposePath) ToAPI() api.ExposePath {
	return api.ExposePath{
		ListenerPort:    p.ListenerPort,
		Path:            p.Path,
		LocalPathPort:   p.LocalPathPort,
		Protocol:        p.Protocol,
		ParsedFromCheck: p.ParsedFromCheck,
	}
}

// Finalize validates ExposeConfig and sets default values
func (e *ExposeConfig) Finalize() {
	for i := 0; i < len(e.Paths); i++ {
		path := &e.Paths[i]

		if path.Protocol == "" {
			path.Protocol = defaultExposeProtocol
		}
	}
}
