package structs

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

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
		Upstreams:              Upstreams(c.Upstreams).ToAPI(),
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

// Validate sanity checks the struct is valid
func (u *Upstream) Validate() error {
	if u.DestinationType != UpstreamDestTypeService &&
		u.DestinationType != UpstreamDestTypePreparedQuery {
		return fmt.Errorf("unknown upstream destination type")
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
	}
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
	if typ == "" {
		typ = UpstreamDestTypeService
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
