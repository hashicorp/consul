package structs

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
)

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
	DestinationServiceName string `json:",omitempty"`

	// DestinationServiceID is optional and should only be specified for
	// "side-car" style proxies where the proxy is in front of just a single
	// instance of the service. It should be set to the service ID of the instance
	// being represented which must be registered to the same agent. It's valid to
	// provide a service ID that does not yet exist to avoid timing issues when
	// bootstrapping a service with a proxy.
	DestinationServiceID string `json:",omitempty"`

	// LocalServiceAddress is the address of the local service instance. It is
	// optional and should only be specified for "side-car" style proxies. It will
	// default to 127.0.0.1 if the proxy is a "side-car" (DestinationServiceID is
	// set) but otherwise will be ignored.
	LocalServiceAddress string `json:",omitempty"`

	// LocalServicePort is the port of the local service instance. It is optional
	// and should only be specified for "side-car" style proxies. It will default
	// to the registered port for the instance if the proxy is a "side-car"
	// (DestinationServiceID is set) but otherwise will be ignored.
	LocalServicePort int `json:",omitempty"`

	// Config is the arbitrary configuration data provided with the proxy
	// registration.
	Config map[string]interface{} `json:",omitempty" bexpr:"-"`

	// Upstreams describes any upstream dependencies the proxy instance should
	// setup.
	Upstreams Upstreams `json:",omitempty"`

	// MeshGateway defines the mesh gateway configuration for this upstream
	MeshGateway MeshGatewayConfig `json:",omitempty"`
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
	DestinationType      string
	DestinationNamespace string `json:",omitempty"`
	DestinationName      string

	// Datacenter that the service discovery request should be run against. Note
	// for prepared queries, the actual results might be from a different
	// datacenter.
	Datacenter string

	// LocalBindAddress is the ip address a side-car proxy should listen on for
	// traffic destined for this upstream service. Default if empty is 127.0.0.1.
	LocalBindAddress string `json:",omitempty"`

	// LocalBindPort is the ip address a side-car proxy should listen on for traffic
	// destined for this upstream service. Required.
	LocalBindPort int

	// Config is an opaque config that is specific to the proxy process being run.
	// It can be used to pass arbitrary configuration for this specific upstream
	// to the proxy.
	Config map[string]interface{} `bexpr:"-"`

	// MeshGateway is the configuration for mesh gateway usage of this upstream
	MeshGateway MeshGatewayConfig `json:",omitempty"`
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
	if u.DestinationNamespace != "" && u.DestinationNamespace != "default" {
		name = u.DestinationNamespace + "/" + u.DestinationName
	}
	if u.Datacenter != "" {
		name += "?dc=" + u.Datacenter
	}
	typ := u.DestinationType
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
