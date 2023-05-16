// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

// RuntimeConfig is the configuration for an extension attached to a service on the local proxy.
// It should depend on the API client rather than the structs package because the API client is
// meant to be public.
type RuntimeConfig struct {
	// EnvoyExtension is the extension that will patch Envoy resources.
	EnvoyExtension api.EnvoyExtension

	// ServiceName is the name of the service the EnvoyExtension is being applied to. It could be the local service or
	// an upstream of the local service.
	ServiceName api.CompoundServiceName

	// Upstreams will only be configured if the EnvoyExtension is being applied to an upstream.
	// If there are no Upstreams, then EnvoyExtension is being applied to the local service's resources.
	Upstreams map[api.CompoundServiceName]*UpstreamData

	// LocalUpstreams will only be configured if the EnvoyExtension is being applied to the local service.
	LocalUpstreams map[api.CompoundServiceName]*UpstreamData

	// Kind is mode the local Envoy proxy is running in. For now, only connect proxy and
	// terminating gateways are supported.
	Kind api.ServiceKind

	// Protocol is the protocol configured for the local service. It may be empty which implies tcp.
	Protocol string
}

// IsLocal indicates if the extension configuration is for the proxy's local service.
func (ec RuntimeConfig) IsLocal() bool {
	return !ec.IsUpstream()
}

// IsUpstream indicates if the extension configuration is for an upstream service.
func (ec RuntimeConfig) IsUpstream() bool {
	_, ok := ec.Upstreams[ec.ServiceName]
	return ok
}

// MatchesUpstreamServiceSNI indicates if the extension configuration is for an upstream service
// that matches the given SNI.
func (ec RuntimeConfig) MatchesUpstreamServiceSNI(sni string) bool {
	u := ec.Upstreams[ec.ServiceName]
	_, match := u.SNI[sni]
	return match
}

// EnvoyID returns the unique Envoy identifier of the upstream service.
func (ec RuntimeConfig) EnvoyID() string {
	u := ec.Upstreams[ec.ServiceName]
	return u.EnvoyID
}

// OutgoingProxyKind returns the service kind for the outgoing listener of an upstream service.
func (ec RuntimeConfig) OutgoingProxyKind() api.ServiceKind {
	u := ec.Upstreams[ec.ServiceName]
	return u.OutgoingProxyKind
}
