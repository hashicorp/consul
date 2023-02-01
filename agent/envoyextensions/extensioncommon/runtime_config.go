package extensioncommon

import "github.com/hashicorp/consul/api"

// UpstreamData has the SNI, EnvoyID, and OutgoingProxyKind of the upstream services for the local proxy and this data
// is used to choose which Envoy resources to patch.
type UpstreamData struct {
	// SNI is the SNI header used to reach an upstream service.
	SNI map[string]struct{}

	// EnvoyID is the envoy ID of an upstream service, structured <service> or <partition>/<ns>/<service> when using a
	// non-default namespace or partition.
	EnvoyID string

	// OutgoingProxyKind is the type of proxy of the upstream service. However, if the upstream is "typical" this will
	// be set to "connect-proxy" instead.
	OutgoingProxyKind api.ServiceKind

	// VIP is the tproxy virtual IP used to reach an upstream service.
	VIP string
}

// RuntimeConfig is the configuration for an extension attached to a service on the local proxy. Currently, it
// is only created for the local proxy's upstream service if the upstream service has an extension configured. In the
// future it will also include information about the service local to the local proxy as well. It should depend on the
// API client rather than the structs package because the API client is meant to be public.
type RuntimeConfig struct {
	// EnvoyExtension is the extension that will patch Envoy resources.
	EnvoyExtension api.EnvoyExtension

	// ServiceName is the name of the service the EnvoyExtension is being applied to. It could be the local service or
	// an upstream of the local service.
	ServiceName api.CompoundServiceName

	// Upstreams will only be configured if the EnvoyExtension is being applied to an upstream.
	// If there are no Upstreams, then EnvoyExtension is being applied to the local service's resources.
	Upstreams map[api.CompoundServiceName]UpstreamData

	// Kind is mode the local Envoy proxy is running in. For now, only connect proxy and
	// terminating gateways are supported.
	Kind api.ServiceKind
}

func (ec RuntimeConfig) IsUpstream() bool {
	_, ok := ec.Upstreams[ec.ServiceName]
	return ok
}

func (ec RuntimeConfig) MatchesUpstreamServiceSNI(sni string) bool {
	u := ec.Upstreams[ec.ServiceName]
	_, match := u.SNI[sni]
	return match
}

func (ec RuntimeConfig) EnvoyID() string {
	u := ec.Upstreams[ec.ServiceName]
	return u.EnvoyID
}

func (ec RuntimeConfig) OutgoingProxyKind() api.ServiceKind {
	u := ec.Upstreams[ec.ServiceName]
	return u.OutgoingProxyKind
}
