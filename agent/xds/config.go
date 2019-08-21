package xds

import (
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/mapstructure"
)

// ProxyConfig describes the keys we understand from Connect.Proxy.Config. Note
// that this only includes config keys that affects runtime config delivered by
// xDS. For Envoy config keys that affect bootstrap generation see
// command/connect/envoy/bootstrap_config.go.
type ProxyConfig struct {
	// PublicListenerJSON is a complete override ("escape hatch") for the
	// upstream's public listener. The Connect server TLS certificate and
	// validation context will be injected overriding any TLS settings present. An
	// AuthZ filter will also be prepended to each filterChain provided to enforce
	// Connect's access control.
	//
	// Note: This escape hatch is compatible with the discovery chain.
	PublicListenerJSON string `mapstructure:"envoy_public_listener_json"`

	// LocalClusterJSON is a complete override ("escape hatch") for the
	// local application cluster.
	//
	// Note: This escape hatch is compatible with the discovery chain.
	LocalClusterJSON string `mapstructure:"envoy_local_cluster_json"`

	// LocalConnectTimeoutMs is the number of milliseconds to timeout making a new
	// connection to the local app instance. Defaults to 5000 (5 seconds) if not
	// set.
	LocalConnectTimeoutMs int `mapstructure:"local_connect_timeout_ms"`

	// Protocol describes the service's protocol. Valid values are "tcp",
	// "http" and "grpc". Anything else is treated as tcp. This enables
	// protocol aware features like per-request metrics and connection
	// pooling, tracing, routing etc.
	Protocol string `mapstructure:"protocol"`

	// BindAddress overrides the address the proxy's listener binds to. This
	// enables proxies in network namespaces to bind to a different address
	// than the host address.
	BindAddress string `mapstructure:"bind_address"`

	// BindPort overrides the port the proxy's listener binds to. This
	// enable proxies in network namespaces to bind to a different port
	// than the host port being advertised.
	BindPort int `mapstructure:"bind_port"`
}

// ParseProxyConfig returns the ProxyConfig parsed from the an opaque map. If an
// error occurs during parsing it is returned along with the default config this
// allows caller to choose whether and how to report the error.
func ParseProxyConfig(m map[string]interface{}) (ProxyConfig, error) {
	var cfg ProxyConfig
	err := mapstructure.WeakDecode(m, &cfg)
	// Set defaults (even if error is returned)
	if cfg.Protocol == "" {
		cfg.Protocol = "tcp"
	} else {
		cfg.Protocol = strings.ToLower(cfg.Protocol)
	}
	if cfg.LocalConnectTimeoutMs < 1 {
		cfg.LocalConnectTimeoutMs = 5000
	}
	return cfg, err
}

type MeshGatewayConfig struct {
	// BindTaggedAddresses when set will cause all of the services tagged
	// addresses to have listeners bound to them in addition to the main service
	// address listener. This is only suitable when the tagged addresses are IP
	// addresses of network interfaces Envoy can see. i.e. When using DNS names
	// for those addresses or where an external entity maps that IP to the Envoy
	// (like AWS EC2 mapping a public IP to the private interface) then this
	// cannot be used. See the BindAddresses config instead
	//
	// TODO - wow this is a verbose setting name. Maybe shorten this
	BindTaggedAddresses bool `mapstructure:"envoy_mesh_gateway_bind_tagged_addresses"`

	// BindAddresses additional bind addresses to configure listeners for
	BindAddresses map[string]structs.ServiceAddress `mapstructure:"envoy_mesh_gateway_bind_addresses"`

	// NoDefaultBind indicates that we should not bind to the default address of the
	// gateway service
	NoDefaultBind bool `mapstructure:"envoy_mesh_gateway_no_default_bind"`

	// ConnectTimeoutMs is the number of milliseconds to timeout making a new
	// connection to this upstream. Defaults to 5000 (5 seconds) if not set.
	ConnectTimeoutMs int `mapstructure:"connect_timeout_ms"`
}

// ParseMeshGatewayConfig returns the MeshGatewayConfig parsed from an opaque map. If an
// error occurs during parsing, it is returned along with the default config. This
// allows the caller to choose whether and how to report the error
func ParseMeshGatewayConfig(m map[string]interface{}) (MeshGatewayConfig, error) {
	var cfg MeshGatewayConfig
	err := mapstructure.WeakDecode(m, &cfg)

	if cfg.ConnectTimeoutMs < 1 {
		cfg.ConnectTimeoutMs = 5000
	}
	return cfg, err
}

// UpstreamConfig describes the keys we understand from
// Connect.Proxy.Upstream[*].Config.
type UpstreamConfig struct {
	// ListenerJSON is a complete override ("escape hatch") for the upstream's
	// listener.
	//
	// Note: This escape hatch is NOT compatible with the discovery chain and
	// will be ignored if a discovery chain is active.
	ListenerJSON string `mapstructure:"envoy_listener_json"`

	// ClusterJSON is a complete override ("escape hatch") for the upstream's
	// cluster. The Connect client TLS certificate and context will be injected
	// overriding any TLS settings present.
	//
	// Note: This escape hatch is NOT compatible with the discovery chain and
	// will be ignored if a discovery chain is active.
	ClusterJSON string `mapstructure:"envoy_cluster_json"`

	// Protocol describes the upstream's service protocol. Valid values are "tcp",
	// "http" and "grpc". Anything else is treated as tcp. The enables protocol
	// aware features like per-request metrics and connection pooling, tracing,
	// routing etc.
	Protocol string `mapstructure:"protocol"`

	// ConnectTimeoutMs is the number of milliseconds to timeout making a new
	// connection to this upstream. Defaults to 5000 (5 seconds) if not set.
	ConnectTimeoutMs int `mapstructure:"connect_timeout_ms"`
}

func ParseUpstreamConfigNoDefaults(m map[string]interface{}) (UpstreamConfig, error) {
	var cfg UpstreamConfig
	err := mapstructure.WeakDecode(m, &cfg)
	return cfg, err
}

// ParseUpstreamConfig returns the UpstreamConfig parsed from the an opaque map.
// If an error occurs during parsing it is returned along with the default
// config this allows caller to choose whether and how to report the error.
func ParseUpstreamConfig(m map[string]interface{}) (UpstreamConfig, error) {
	cfg, err := ParseUpstreamConfigNoDefaults(m)
	// Set defaults (even if error is returned)
	if cfg.Protocol == "" {
		cfg.Protocol = "tcp"
	} else {
		cfg.Protocol = strings.ToLower(cfg.Protocol)
	}
	if cfg.ConnectTimeoutMs < 1 {
		cfg.ConnectTimeoutMs = 5000
	}
	return cfg, err
}
