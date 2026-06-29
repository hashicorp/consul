// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"sort"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_ext_proc_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_upstreams_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// inferenceGatewayListenerName is the name of the inference gateway's single
	// inbound listener and its RDS route configuration.
	inferenceGatewayListenerName = "inference-gateway"

	// inferenceExtProcClusterName is the local cluster Envoy uses to reach the
	// co-located policy processor over the loopback/UDS socket.
	inferenceExtProcClusterName = "local_ext_proc"

	// inferenceClusterHeader is the header the policy processor sets to select
	// the resolved model cluster; the gateway routes on it.
	inferenceClusterHeader = "x-ai-cluster"

	// inferenceListenerMetadataNamespace is the FilterMetadata key carrying the
	// discovered model catalog for the policy processor.
	inferenceListenerMetadataNamespace = "consul.ai"
)

// listenersFromSnapshotInferenceGateway builds the inference gateway's single
// inbound listener: mesh mTLS + SPIFFE on the downstream (so the calling agent's
// identity is enforced), an HTTP connection manager with the ext_proc filter
// (dialing the policy processor over UDS) ahead of the router, and listener
// metadata carrying the discovered model catalog.
func (s *ResourceGenerator) listenersFromSnapshotInferenceGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	ig := &cfgSnap.InferenceGateway

	addr := cfgSnap.Address
	if addr == "" {
		addr = "0.0.0.0"
	}

	opts := makeListenerOpts{
		name:       inferenceGatewayListenerName,
		accessLogs: cfgSnap.Proxy.AccessLogs,
		addr:       addr,
		port:       cfgSnap.Port,
		direction:  envoy_core_v3.TrafficDirection_INBOUND,
		logger:     s.Logger,
	}
	l := makeListener(opts)

	extProcFilter, err := makeInferenceExtProcHTTPFilter(ig.GatewayConfig)
	if err != nil {
		return nil, err
	}

	filterOpts := listenerFilterOpts{
		protocol:         "http",
		filterName:       inferenceGatewayListenerName,
		routeName:        inferenceGatewayListenerName,
		useRDS:           true,
		accessLogs:       &cfgSnap.Proxy.AccessLogs,
		logger:           s.Logger,
		httpAuthzFilters: []*envoy_http_v3.HttpFilter{extProcFilter},
		// Forward the downstream client cert details (including the URI SAN /
		// SPIFFE id) into the x-forwarded-client-cert header so the co-located
		// policy processor can extract the caller's identity for routing. This
		// mirrors the connect-proxy public listener (APPEND_FORWARD + Uri:true).
		forwardClientDetails: true,
		forwardClientPolicy:  envoy_http_v3.HttpConnectionManager_APPEND_FORWARD,
	}
	filter, err := makeListenerFilter(filterOpts)
	if err != nil {
		return nil, err
	}
	l.FilterChains = []*envoy_listener_v3.FilterChain{{
		Filters: []*envoy_listener_v3.Filter{filter},
	}}

	// Surface the discovered model catalog to the processor via listener metadata.
	if md := makeInferenceListenerMetadata(ig.Models); md != nil {
		l.Metadata = md
	}

	// Attach the mesh mTLS downstream context (RequireClientCertificate + SPIFFE).
	if err := s.finalizePublicListenerFromConfig(l, cfgSnap, true); err != nil {
		return nil, err
	}

	return []proto.Message{l}, nil
}

// makeInferenceExtProcHTTPFilter builds the ext_proc HTTP filter that streams to
// the co-located policy processor. The request body and the response body both
// default to BUFFERED so the processor transforms a full request and normalizes a
// non-streamed provider response in one pass. AllowModeOverride lets the processor
// flip the response body to STREAMED per-response for server-sent-event responses,
// so streamed completions pass through chunk by chunk while the processor meters
// the stream.
func makeInferenceExtProcHTTPFilter(cfg *structs.AIGatewayConfigEntry) (*envoy_http_v3.HttpFilter, error) {
	failureModeAllow := cfg != nil && cfg.Processor.FailureMode == structs.AIGatewayFailureModeOpen

	extProc := &envoy_http_ext_proc_v3.ExternalProcessor{
		GrpcService: &envoy_core_v3.GrpcService{
			TargetSpecifier: &envoy_core_v3.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &envoy_core_v3.GrpcService_EnvoyGrpc{
					ClusterName: inferenceExtProcClusterName,
				},
			},
		},
		FailureModeAllow:  failureModeAllow,
		AllowModeOverride: true,
		ProcessingMode: &envoy_http_ext_proc_v3.ProcessingMode{
			RequestHeaderMode:   envoy_http_ext_proc_v3.ProcessingMode_SEND,
			RequestBodyMode:     envoy_http_ext_proc_v3.ProcessingMode_BUFFERED,
			ResponseHeaderMode:  envoy_http_ext_proc_v3.ProcessingMode_SEND,
			ResponseBodyMode:    envoy_http_ext_proc_v3.ProcessingMode_BUFFERED,
			RequestTrailerMode:  envoy_http_ext_proc_v3.ProcessingMode_SKIP,
			ResponseTrailerMode: envoy_http_ext_proc_v3.ProcessingMode_SKIP,
		},
	}
	return makeEnvoyHTTPFilter("envoy.filters.http.ext_proc", extProc)
}

// makeInferenceListenerMetadata renders the discovered model catalog (names,
// roles, and catalog labels) into listener FilterMetadata under "consul.ai".
func makeInferenceListenerMetadata(models map[structs.ServiceName]*proxycfg.InferenceGatewayModel) *envoy_core_v3.Metadata {
	if len(models) == 0 {
		return nil
	}

	names := make([]structs.ServiceName, 0, len(models))
	for sn := range models {
		names = append(names, sn)
	}
	sort.Slice(names, func(i, j int) bool { return names[i].String() < names[j].String() })

	modelValues := make([]*structpb.Value, 0, len(names))
	for _, sn := range names {
		m := models[sn]
		labelFields := make(map[string]*structpb.Value, len(m.Labels))
		for k, v := range m.Labels {
			labelFields[k] = structpb.NewStringValue(v)
		}
		modelValues = append(modelValues, structpb.NewStructValue(&structpb.Struct{
			Fields: map[string]*structpb.Value{
				"name":   structpb.NewStringValue(sn.Name),
				"role":   structpb.NewStringValue(m.Role),
				"labels": structpb.NewStructValue(&structpb.Struct{Fields: labelFields}),
			},
		}))
	}

	return &envoy_core_v3.Metadata{
		FilterMetadata: map[string]*structpb.Struct{
			inferenceListenerMetadataNamespace: {
				Fields: map[string]*structpb.Value{
					"models": structpb.NewListValue(&structpb.ListValue{Values: modelValues}),
				},
			},
		},
	}
}

// clustersFromSnapshotInferenceGateway builds the local ext_proc cluster (UDS)
// and an EDS cluster per discovered model upstream.
func (s *ResourceGenerator) clustersFromSnapshotInferenceGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	ig := &cfgSnap.InferenceGateway

	res := make([]proto.Message, 0, len(ig.Models)+1)

	if ig.GatewayConfig != nil && ig.GatewayConfig.Processor.UDSPath != "" {
		res = append(res, makeInferenceExtProcCluster(ig.GatewayConfig.Processor.UDSPath))
	}

	for _, sn := range sortedModelNames(ig.Models) {
		res = append(res, s.makeGatewayCluster(cfgSnap, clusterOpts{name: sn.Name}))
	}

	return res, nil
}

// makeInferenceExtProcCluster builds a STATIC HTTP/2 cluster pointing at the
// processor's Unix domain socket (loopback transport).
func makeInferenceExtProcCluster(path string) *envoy_cluster_v3.Cluster {
	httpProtoOpts := &envoy_upstreams_http_v3.HttpProtocolOptions{
		UpstreamProtocolOptions: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
					Http2ProtocolOptions: &envoy_core_v3.Http2ProtocolOptions{},
				},
			},
		},
	}
	httpProtoOptsAny, _ := anypb.New(httpProtoOpts)

	return &envoy_cluster_v3.Cluster{
		Name:                 inferenceExtProcClusterName,
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC},
		ConnectTimeout:       durationpb.New(5 * time.Second),
		LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: inferenceExtProcClusterName,
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
				LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{{
					HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
						Endpoint: &envoy_endpoint_v3.Endpoint{
							Address: &envoy_core_v3.Address{
								Address: &envoy_core_v3.Address_Pipe{
									Pipe: &envoy_core_v3.Pipe{Path: path},
								},
							},
						},
					},
				}},
			}},
		},
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": httpProtoOptsAny,
		},
	}
}

// endpointsFromSnapshotInferenceGateway builds the EDS load assignments for the
// discovered model upstreams.
func (s *ResourceGenerator) endpointsFromSnapshotInferenceGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	ig := &cfgSnap.InferenceGateway

	res := make([]proto.Message, 0, len(ig.Models))
	for _, sn := range sortedModelNames(ig.Models) {
		m := ig.Models[sn]
		la := makeLoadAssignment(
			s.Logger,
			cfgSnap,
			sn.Name,
			nil,
			[]loadAssignmentEndpointGroup{{Endpoints: m.Nodes}},
			cfgSnap.Locality,
		)
		res = append(res, la)
	}
	return res, nil
}

// routesForInferenceGateway builds the RDS route config: the processor sets the
// x-ai-cluster header to the resolved model and the gateway routes on it, with
// the routing policy's default fallback as the catch-all.
func (s *ResourceGenerator) routesForInferenceGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	ig := &cfgSnap.InferenceGateway

	vh := &envoy_route_v3.VirtualHost{
		Name:    inferenceGatewayListenerName,
		Domains: []string{"*"},
	}

	for _, sn := range sortedModelNames(ig.Models) {
		vh.Routes = append(vh.Routes, &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/"},
				Headers: []*envoy_route_v3.HeaderMatcher{{
					Name: inferenceClusterHeader,
					HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_StringMatch{
						StringMatch: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{Exact: sn.Name},
						},
					},
				}},
			},
			Action: &envoy_route_v3.Route_Route{
				Route: &envoy_route_v3.RouteAction{
					ClusterSpecifier: &envoy_route_v3.RouteAction_Cluster{Cluster: sn.Name},
				},
			},
		})
	}

	// Default catch-all: route to the first configured fallback cluster if one
	// exists, otherwise return 503 so misrouted requests fail closed.
	if def := defaultInferenceCluster(ig.GatewayConfig, ig.Models); def != "" {
		vh.Routes = append(vh.Routes, &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/"}},
			Action: &envoy_route_v3.Route_Route{
				Route: &envoy_route_v3.RouteAction{
					ClusterSpecifier: &envoy_route_v3.RouteAction_Cluster{Cluster: def},
				},
			},
		})
	} else {
		vh.Routes = append(vh.Routes, &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/"}},
			Action: &envoy_route_v3.Route_DirectResponse{
				DirectResponse: &envoy_route_v3.DirectResponseAction{Status: 503},
			},
		})
	}

	return []proto.Message{&envoy_route_v3.RouteConfiguration{
		Name:         inferenceGatewayListenerName,
		VirtualHosts: []*envoy_route_v3.VirtualHost{vh},
	}}, nil
}

// defaultInferenceCluster returns the default fallback cluster for the routing
// policy, if one is both configured and a discovered model.
func defaultInferenceCluster(cfg *structs.AIGatewayConfigEntry, models map[structs.ServiceName]*proxycfg.InferenceGatewayModel) string {
	if cfg == nil {
		return ""
	}
	for _, name := range cfg.Routing.FallbackChain {
		sn := structs.NewServiceName(name, &cfg.EnterpriseMeta)
		if _, ok := models[sn]; ok {
			return name
		}
	}
	return ""
}

func sortedModelNames(models map[structs.ServiceName]*proxycfg.InferenceGatewayModel) []structs.ServiceName {
	names := make([]structs.ServiceName, 0, len(models))
	for sn := range models {
		names = append(names, sn)
	}
	sort.Slice(names, func(i, j int) bool { return names[i].String() < names[j].String() })
	return names
}
