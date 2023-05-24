// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package extensioncommon

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"strings"
)

// MakeUpstreamTLSTransportSocket generates an Envoy transport socket for the given TLS context.
func MakeUpstreamTLSTransportSocket(tlsContext *envoy_tls_v3.UpstreamTlsContext) (*envoy_core_v3.TransportSocket, error) {
	if tlsContext == nil {
		return nil, nil
	}
	return MakeTransportSocket("tls", tlsContext)
}

// MakeTransportSocket generates an Envoy transport socket from the given proto message.
func MakeTransportSocket(name string, config proto.Message) (*envoy_core_v3.TransportSocket, error) {
	any, err := anypb.New(config)
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

// MakeEnvoyHTTPFilter generates an Envoy HTTP filter from the given proto message.
func MakeEnvoyHTTPFilter(name string, cfg proto.Message) (*envoy_http_v3.HttpFilter, error) {
	any, err := anypb.New(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_http_v3.HttpFilter{
		Name:       name,
		ConfigType: &envoy_http_v3.HttpFilter_TypedConfig{TypedConfig: any},
	}, nil
}

// MakeFilter generates an Envoy listener filter from the given proto message.
func MakeFilter(name string, cfg proto.Message) (*envoy_listener_v3.Filter, error) {
	any, err := anypb.New(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_listener_v3.Filter{
		Name:       name,
		ConfigType: &envoy_listener_v3.Filter_TypedConfig{TypedConfig: any},
	}, nil
}

// GetListenerEnvoyID returns the Envoy ID string parsed from the name of the given Listener. If none is found, it
// returns the empty string.
func GetListenerEnvoyID(l *envoy_listener_v3.Listener) string {
	if id, _, found := strings.Cut(l.Name, ":"); found {
		return id
	}
	return ""
}

// IsLocalAppCluster returns true if the given Cluster represents the local Cluster, which receives inbound traffic to
// the local proxy.
func IsLocalAppCluster(c *envoy_cluster_v3.Cluster) bool {
	return c.Name == xdscommon.LocalAppClusterName
}

// IsRouteToLocalAppCluster takes a RouteConfiguration and returns true if all routes within it target the local
// Cluster. Note that because we currently target RouteConfiguration in PatchRoute, we have to check multiple individual
// Route resources.
func IsRouteToLocalAppCluster(r *envoy_route_v3.RouteConfiguration) bool {
	clusterNames := RouteClusterNames(r)
	_, match := clusterNames[xdscommon.LocalAppClusterName]

	return match && len(clusterNames) == 1
}

// IsInboundPublicListener returns true if the given Listener represents the inbound public Listener for the local
// service.
func IsInboundPublicListener(l *envoy_listener_v3.Listener) bool {
	return GetListenerEnvoyID(l) == xdscommon.PublicListenerName
}

// IsOutboundTProxyListener returns true if the given Listener represents the outbound TProxy Listener for the local
// service.
func IsOutboundTProxyListener(l *envoy_listener_v3.Listener) bool {
	return GetListenerEnvoyID(l) == xdscommon.OutboundListenerName
}
