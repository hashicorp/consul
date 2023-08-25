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

	// ServiceName is the name of the service the EnvoyExtension is being applied to. It is typically the local service
	// (IsSourcedFromUpstream = false), but can also be an upstream of the local service (IsSourcedFromUpstream = true).
	ServiceName api.CompoundServiceName

	// Upstreams represent the upstreams of the local service. This is consistent regardless of the value of
	// IsSourcedFromUpstream, which refers to the Envoy extension source.
	Upstreams map[api.CompoundServiceName]*UpstreamData

	// IsSourcedFromUpstream is set to true only in the exceptional cases where upstream service config contains
	// extensions that apply to the configured service's downstreams. In those cases, this value will be true when such
	// a downstream is the local service. In all other cases, IsSourcedFromUpstream will be false.
	//
	// This is used exclusively for specific extensions (currently, only AWS Lambda and Validate) in which we
	// intentionally apply the extension to downstreams rather than the local proxy of the configured service itself.
	// This is generally dangerous, since it circumvents ACLs for the affected downstream services (the upstream owner
	// may not have `service:write` for the downstreams).
	//
	// Extensions used this way MUST be designed to allow only trusted modifications of downstream proxies that impact
	// their ability to call the upstream service. Remote configurations MUST NOT be allowed to otherwise modify local
	// proxies until we support explicit extension capability controls or require privileges higher than the typical
	// `service:write` required to configure extensions.
	//
	// See UpstreamEnvoyExtender for the code that applies RuntimeConfig with this flag set.
	IsSourcedFromUpstream bool

	// Kind is mode the local Envoy proxy is running in. For now, only connect proxy and
	// terminating gateways are supported.
	Kind api.ServiceKind
}

// MatchesUpstreamServiceSNI indicates if the extension configuration is for an upstream service
// that matches the given SNI, if the RuntimeConfig corresponds to an upstream of the local service.
// Only used when IsSourcedFromUpstream is true.
func (c RuntimeConfig) MatchesUpstreamServiceSNI(sni string) bool {
	u := c.Upstreams[c.ServiceName]
	_, match := u.SNI[sni]
	return match
}

// UpstreamEnvoyID returns the unique Envoy identifier of the upstream service, if the RuntimeConfig corresponds to an
// upstream of the local service. Note that this could be the local service if it targets itself as an upstream.
// Only used when IsSourcedFromUpstream is true.
func (c RuntimeConfig) UpstreamEnvoyID() string {
	u := c.Upstreams[c.ServiceName]
	return u.EnvoyID
}

// UpstreamOutgoingProxyKind returns the service kind for the outgoing listener of the upstream service, if the
// RuntimeConfig corresponds to an upstream of the local service.
// Only used when IsSourcedFromUpstream is true.
func (c RuntimeConfig) UpstreamOutgoingProxyKind() api.ServiceKind {
	u := c.Upstreams[c.ServiceName]
	return u.OutgoingProxyKind
}
