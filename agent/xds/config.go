package xds

import (
	"google.golang.org/protobuf/types/known/wrapperspb"
	"strings"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/decode"
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

	// ListenerTracingJSON is a complete override ("escape hatch") for the
	// listeners tracing configuration.
	//
	// Note: This escape hatch is compatible with the discovery chain.
	ListenerTracingJSON string `mapstructure:"envoy_listener_tracing_json"`

	// LocalClusterJSON is a complete override ("escape hatch") for the
	// local application cluster.
	//
	// Note: This escape hatch is compatible with the discovery chain.
	LocalClusterJSON string `mapstructure:"envoy_local_cluster_json"`

	// LocalConnectTimeoutMs is the number of milliseconds to timeout making a new
	// connection to the local app instance. Defaults to 5000 (5 seconds) if not
	// set.
	LocalConnectTimeoutMs int `mapstructure:"local_connect_timeout_ms"`

	// LocalRequestTimeoutMs is the number of milliseconds to timeout HTTP requests
	// to the local app instance. If not set, no value is set, Envoy defaults are
	// respected (15s)
	LocalRequestTimeoutMs *int `mapstructure:"local_request_timeout_ms"`

	// LocalIdleTimeoutMs is the number of milliseconds to timeout HTTP streams
	// to the local app instance. If not set, no value is set, Envoy defaults are
	// respected (300s)
	LocalIdleTimeoutMs *int `mapstructure:"local_idle_timeout_ms"`

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

	// MaxInboundConnections is the maximum number of inbound connections to
	// the proxy. If not set, the default is 0 (no limit).
	MaxInboundConnections int `mapstructure:"max_inbound_connections"`

	// BalanceInboundConnections indicates how the proxy should attempt to distribute
	// connections across worker threads. Only used by envoy proxies.
	BalanceInboundConnections string `json:",omitempty" alias:"balance_inbound_connections"`
}

// ParseProxyConfig returns the ProxyConfig parsed from the an opaque map. If an
// error occurs during parsing it is returned along with the default config this
// allows caller to choose whether and how to report the error.
func ParseProxyConfig(m map[string]interface{}) (ProxyConfig, error) {
	var cfg ProxyConfig
	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			decode.HookWeakDecodeFromSlice,
			decode.HookTranslateKeys,
		),
		Result:           &cfg,
		WeaklyTypedInput: true,
	}
	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return cfg, err
	}
	if err := decoder.Decode(m); err != nil {
		return cfg, err
	}

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

type GatewayConfig struct {
	// BindTaggedAddresses when set will cause all of the services tagged
	// addresses to have listeners bound to them in addition to the main service
	// address listener. This is only suitable when the tagged addresses are IP
	// addresses of network interfaces Envoy can see. i.e. When using DNS names
	// for those addresses or where an external entity maps that IP to the Envoy
	// (like AWS EC2 mapping a public IP to the private interface) then this
	// cannot be used. See the BindAddresses config instead
	BindTaggedAddresses bool `mapstructure:"envoy_gateway_bind_tagged_addresses" alias:"envoy_mesh_gateway_bind_tagged_addresses"`

	// BindAddresses additional bind addresses to configure listeners for
	BindAddresses map[string]structs.ServiceAddress `mapstructure:"envoy_gateway_bind_addresses" alias:"envoy_mesh_gateway_bind_addresses"`

	// NoDefaultBind indicates that we should not bind to the default address of the
	// gateway service
	NoDefaultBind bool `mapstructure:"envoy_gateway_no_default_bind" alias:"envoy_mesh_gateway_no_default_bind"`

	// DNSDiscoveryType indicates the DNS service discovery type.
	// See: https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/service_discovery#arch-overview-service-discovery-types
	DNSDiscoveryType string `mapstructure:"envoy_dns_discovery_type"`

	// ConnectTimeoutMs is the number of milliseconds to timeout making a new
	// connection to this upstream. Defaults to 5000 (5 seconds) if not set.
	ConnectTimeoutMs int `mapstructure:"connect_timeout_ms"`

	// TCP keepalive settings for remote gateway upstreams (mesh gateways and terminating gateway upstreams).
	// See: https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/core/v3/address.proto#envoy-v3-api-msg-config-core-v3-tcpkeepalive
	TcpKeepaliveEnable   bool `mapstructure:"envoy_gateway_remote_tcp_enable_keepalive"`
	TcpKeepaliveTime     int  `mapstructure:"envoy_gateway_remote_tcp_keepalive_time"`
	TcpKeepaliveInterval int  `mapstructure:"envoy_gateway_remote_tcp_keepalive_interval"`
	TcpKeepaliveProbes   int  `mapstructure:"envoy_gateway_remote_tcp_keepalive_probes"`
}

// ParseGatewayConfig returns the GatewayConfig parsed from an opaque map. If an
// error occurs during parsing, it is returned along with the default config. This
// allows the caller to choose whether and how to report the error
func ParseGatewayConfig(m map[string]interface{}) (GatewayConfig, error) {
	var cfg GatewayConfig
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			decode.HookWeakDecodeFromSlice,
			decode.HookTranslateKeys,
		),
		Result:           &cfg,
		WeaklyTypedInput: true,
	})
	if err != nil {
		return cfg, err
	}
	if err := d.Decode(m); err != nil {
		return cfg, err
	}

	if cfg.ConnectTimeoutMs < 1 {
		cfg.ConnectTimeoutMs = 5000
	}

	cfg.DNSDiscoveryType = strings.ToLower(cfg.DNSDiscoveryType)

	return cfg, err
}

// Return an envoy.OutlierDetection populated by the values from this struct.
// If all values are zero a default empty OutlierDetection will be returned to
// enable outlier detection with default values.
func ToOutlierDetection(p *structs.PassiveHealthCheck) *envoy_cluster_v3.OutlierDetection {
	od := &envoy_cluster_v3.OutlierDetection{}
	if p == nil {
		return od
	}

	if p.Interval != 0 {
		od.Interval = durationpb.New(p.Interval)
	}
	if p.MaxFailures != 0 {
		od.Consecutive_5Xx = &wrappers.UInt32Value{Value: p.MaxFailures}
	}

	if p.EnforcingConsecutive5xx != nil {
		// NOTE: EnforcingConsecutive5xx must be greater than 0 for ingress-gateway
		od.EnforcingConsecutive_5Xx = &wrappers.UInt32Value{Value: *p.EnforcingConsecutive5xx}
	}
	if p.MaxEjectionPercent != nil {
		od.MaxEjectionPercent = &wrapperspb.UInt32Value{Value: *p.MaxEjectionPercent}
	}
	if p.BaseEjectionTime != nil {
		od.BaseEjectionTime = durationpb.New(*p.BaseEjectionTime)
	}

	return od
}
