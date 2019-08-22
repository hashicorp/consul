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

// ParseUpstreamConfig returns the UpstreamConfig parsed from an opaque map.
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

// ExposeConfig describes HTTP paths to expose through Envoy outside of Connect.
// Users can expose individual paths and/or all HTTP/GRPC paths for checks.
type ExposeConfig struct {
	// Checks defines whether paths associated with Consul checks will be exposed.
	// This flag triggers exposing all HTTP and GRPC check paths registered for the service.
	Checks bool `mapstructure:"checks"`

	// Port defines the port of the proxy's listener for exposed paths.
	Port int `mapstructure:"port"`

	// Paths is the list of paths exposed through the proxy.
	Paths []Path `mapstructure:"paths"`
}

type Path struct {
	// Path is the path to expose through the proxy, ie. "/metrics."
	Path string `mapstructure:"path"`

	// Port is the port that the service is listening on for the given path.
	Port int `mapstructure:"port"`

	// Protocol describes the upstream's service protocol.
	// Valid values are "http1.1", "http2" and "grpc". Defaults to "http1.1".
	Protocol string `mapstructure:"protocol"`

	// TLSSkipVerify defines whether incoming requests should be authenticated with TLS.
	TLSSkipVerify bool `mapstructure:"tls_skip_verify"`

	// CAFile is the path to the PEM encoded CA cert used to verify client certificates.
	CAFile string `mapstructure:"ca_file"`
}

// ParseExposeConfig returns the ExposeConfig parsed from an opaque map.
// If an error occurs during parsing it is returned along with the default
// config this allows caller to choose whether and how to report the error.
func ParseExposeConfig(m map[string]interface{}) (ExposeConfig, error) {
	var cfg ExposeConfig
	err := mapstructure.WeakDecode(m, &cfg)
	if cfg.Port == 0 {
		cfg.Port = 21500
	}
	for _, path := range cfg.Paths {
		if path.Protocol == "" {
			path.Protocol = "http1.1"
		} else {
			path.Protocol = strings.ToLower(path.Protocol)
		}
	}
	return cfg, err
}
