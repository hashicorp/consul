package serverlessplugin

import (
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"

	"github.com/golang/protobuf/ptypes"

	"github.com/golang/protobuf/proto"
)

// This is copied from xds and not put into the shared package because I'm not
// convinced it should be shared.

func makeUpstreamTLSTransportSocket(tlsContext *envoy_tls_v3.UpstreamTlsContext) (*envoy_core_v3.TransportSocket, error) {
	if tlsContext == nil {
		return nil, nil
	}
	return makeTransportSocket("tls", tlsContext)
}

func makeTransportSocket(name string, config proto.Message) (*envoy_core_v3.TransportSocket, error) {
	any, err := ptypes.MarshalAny(config)
	if err != nil {
		return nil, err
	}
	return &envoy_core_v3.TransportSocket{
		Name: name,
		ConfigType: &envoy_core_v3.TransportSocket_TypedConfig{
			TypedConfig: any,
		},
	}, nil
}

func makeEnvoyHTTPFilter(name string, cfg proto.Message) (*envoy_http_v3.HttpFilter, error) {
	any, err := ptypes.MarshalAny(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_http_v3.HttpFilter{
		Name:       name,
		ConfigType: &envoy_http_v3.HttpFilter_TypedConfig{TypedConfig: any},
	}, nil
}

func makeFilter(name string, cfg proto.Message) (*envoy_listener_v3.Filter, error) {
	any, err := ptypes.MarshalAny(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_listener_v3.Filter{
		Name:       name,
		ConfigType: &envoy_listener_v3.Filter_TypedConfig{TypedConfig: any},
	}, nil
}
