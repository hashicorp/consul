package structs

import (
	"encoding/json"
	"fmt"

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

	// MeshGatewayModeLocal represents that the Upstrea Connect connections
	// should be made to a mesh gateway in the local datacenter. This is
	MeshGatewayModeLocal MeshGatewayMode = "local"

	// MeshGatewayModeRemote represents that the Upstream Connect connections
	// should be made to a mesh gateway in a remote datacenter.
	MeshGatewayModeRemote MeshGatewayMode = "remote"
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

func (base *MeshGatewayConfig) OverlayWith(overlay MeshGatewayConfig) MeshGatewayConfig {
	out := *base
	if overlay.Mode != MeshGatewayModeDefault {
		out.Mode = overlay.Mode
	}
	return out
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

// ConnectProxyConfig describes the configuration needed for any proxy managed
// or unmanaged. It describes a single logical service's listener and optionally
// upstreams and sidecar-related config for a single instance. To describe a
// centralized proxy that routed traffic for multiple services, a different one
// of these would be needed for each, sharing the same LogicalProxyID.
type ConnectProxyConfig struct {
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

	// TransparentProxy toggles whether inbound and outbound traffic is being
	// redirected to the proxy.
	TransparentProxy bool `json:",omitempty" alias:"transparent_proxy"`
}

func (t *ConnectProxyConfig) UnmarshalJSON(data []byte) (err error) {
	type Alias ConnectProxyConfig
	aux := &struct {
		DestinationServiceNameSnake string            `json:"destination_service_name"`
		DestinationServiceIDSnake   string            `json:"destination_service_id"`
		LocalServiceAddressSnake    string            `json:"local_service_address"`
		LocalServicePortSnake       int               `json:"local_service_port"`
		MeshGatewaySnake            MeshGatewayConfig `json:"mesh_gateway"`
		TransparentProxySnake       bool              `json:"transparent_proxy"`

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
	if t.MeshGateway.Mode == "" {
		t.MeshGateway.Mode = aux.MeshGatewaySnake.Mode
	}
	if !t.TransparentProxy {
		t.TransparentProxy = aux.TransparentProxySnake
	}

	return nil

}

func (c *ConnectProxyConfig) MarshalJSON() ([]byte, error) {
	type typeCopy ConnectProxyConfig
	copy := typeCopy(*c)

	proxyConfig, err := lib.MapWalk(copy.Config)
	if err != nil {
		return nil, err
	}
	copy.Config = proxyConfig

	return json.Marshal(&copy)
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
		Config:                 c.Config,
		Upstreams:              c.Upstreams.ToAPI(),
		MeshGateway:            c.MeshGateway.ToAPI(),
		Expose:                 c.Expose.ToAPI(),
		TransparentProxy:       c.TransparentProxy,
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
	LocalBindPort int `alias:"local_bind_port"`

	// Config is an opaque config that is specific to the proxy process being run.
	// It can be used to pass arbitrary configuration for this specific upstream
	// to the proxy.
	Config map[string]interface{} `json:",omitempty" bexpr:"-"`

	// MeshGateway is the configuration for mesh gateway usage of this upstream
	MeshGateway MeshGatewayConfig `json:",omitempty" alias:"mesh_gateway"`

	// IngressHosts are a list of hosts that should route to this upstream from
	// an ingress gateway. This cannot and should not be set by a user, it is
	// used internally to store the association of hosts to an upstream service.
	IngressHosts []string `json:"-" bexpr:"-"`
}

func (t *Upstream) UnmarshalJSON(data []byte) (err error) {
	type Alias Upstream
	aux := &struct {
		DestinationTypeSnake      string `json:"destination_type"`
		DestinationNamespaceSnake string `json:"destination_namespace"`
		DestinationNameSnake      string `json:"destination_name"`

		LocalBindAddressSnake string `json:"local_bind_address"`
		LocalBindPortSnake    int    `json:"local_bind_port"`

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
	if t.DestinationName == "" {
		t.DestinationName = aux.DestinationNameSnake
	}
	if t.LocalBindAddress == "" {
		t.LocalBindAddress = aux.LocalBindAddressSnake
	}
	if t.LocalBindPort == 0 {
		t.LocalBindPort = aux.LocalBindPortSnake
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

	if u.LocalBindPort == 0 {
		return fmt.Errorf("upstream local bind port cannot be zero")
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
		DestinationName:      u.DestinationName,
		Datacenter:           u.Datacenter,
		LocalBindAddress:     u.LocalBindAddress,
		LocalBindPort:        u.LocalBindPort,
		Config:               u.Config,
		MeshGateway:          u.MeshGateway.ToAPI(),
	}
}

// ToKey returns a value-type representation that uniquely identifies the
// upstream in a canonical way. Set and unset values are deliberately handled
// differently.
//
// These fields should be user-specificed explicit values and not inferred
// values.
func (u *Upstream) ToKey() UpstreamKey {
	return UpstreamKey{
		DestinationType:      u.DestinationType,
		DestinationNamespace: u.DestinationNamespace,
		DestinationName:      u.DestinationName,
		Datacenter:           u.Datacenter,
	}
}

type UpstreamKey struct {
	DestinationType      string
	DestinationName      string
	DestinationNamespace string
	Datacenter           string
}

func (k UpstreamKey) String() string {
	return fmt.Sprintf(
		"[type=%q, name=%q, namespace=%q, datacenter=%q]",
		k.DestinationType,
		k.DestinationName,
		k.DestinationNamespace,
		k.Datacenter,
	)
}

// Identifier returns a string representation that uniquely identifies the
// upstream in a canonical but human readable way.
func (u *Upstream) Identifier() string {
	name := u.DestinationName
	typ := u.DestinationType

	if typ != UpstreamDestTypePreparedQuery && u.DestinationNamespace != "" && u.DestinationNamespace != IntentionDefaultNamespace {
		name = u.DestinationNamespace + "/" + u.DestinationName
	}
	if u.Datacenter != "" {
		name += "?dc=" + u.Datacenter
	}

	// Service is default type so never prefix it. This is more readable and long
	// term it is the only type that matters so we can drop the prefix and have
	// nicer naming in metrics etc.
	if typ == "" || typ == UpstreamDestTypeService {
		return name
	}
	return typ + ":" + name
}

// String implements Stringer by returning the Identifier.
func (u *Upstream) String() string {
	return u.Identifier()
}

// UpstreamFromAPI is a helper for converting api.Upstream to Upstream.
func UpstreamFromAPI(u api.Upstream) Upstream {
	return Upstream{
		DestinationType:      string(u.DestinationType),
		DestinationNamespace: u.DestinationNamespace,
		DestinationName:      u.DestinationName,
		Datacenter:           u.Datacenter,
		LocalBindAddress:     u.LocalBindAddress,
		LocalBindPort:        u.LocalBindPort,
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
