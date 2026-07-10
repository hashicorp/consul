// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"sort"
	"strings"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_ext_proc_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	envoy_upstream_codec_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/upstream_codec/v3"
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

	// inferenceModelHeader is the header the downstream (pre-route) ext_proc phase
	// promotes the request body's model onto; the gateway routes on it (model →
	// backend cluster) so Envoy owns cluster selection. The upstream (post-route)
	// ext_proc phase then transforms to the selected backend's native schema.
	inferenceModelHeader = "x-ai-model"

	// inferenceSpecializationHeader is the caller-supplied header naming a
	// required capability. Consul renders a native header-match route per
	// discovered model capability, so selection happens in Envoy without the
	// policy processor emitting a cluster.
	inferenceSpecializationHeader = "x-inference-specialization"

	// inferenceCapabilitiesLabel is the model catalog meta key whose value is the
	// capability a model advertises (matched against inferenceSpecializationHeader).
	inferenceCapabilitiesLabel = "capabilities"

	// inferenceModelFamilyLabel is the model catalog meta key naming the concrete
	// upstream model. The processor stamps it onto a request whose model is "auto".
	inferenceModelFamilyLabel = "model_family"

	// inferenceModelAPILabel is the model catalog meta key naming the wire/adapter
	// dialect the terminating gateway exposes for this model (e.g. "openai",
	// "gemini"). It is deliberately distinct from the vendor: a model may be a
	// Google model reached through an OpenAI-compatible endpoint.
	inferenceModelAPILabel = "model_api"

	// inferenceListenerMetadataNamespace is the FilterMetadata key carrying the
	// discovered model catalog for the policy processor. It is also the route
	// metadata namespace for a capability route's per-model routing facts.
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

// makeInferenceExtProcHTTPFilter builds the DOWNSTREAM (pre-route) ext_proc HTTP
// filter on the HCM chain. It streams the request to the co-located policy
// processor, which resolves the caller's identity, enforces request PII on the
// canonical body, and promotes the body model to x-ai-model so Envoy routes on it.
// The request body is BUFFERED so the processor sees the whole canonical body. The
// response is not processed here — the upstream (post-route) filter owns response
// transform, PII, and metering — so response processing is skipped, and no cluster
// metadata is requested (the downstream phase runs before a backend is selected).
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
		FailureModeAllow: failureModeAllow,
		ProcessingMode: &envoy_http_ext_proc_v3.ProcessingMode{
			RequestHeaderMode:   envoy_http_ext_proc_v3.ProcessingMode_SEND,
			RequestBodyMode:     envoy_http_ext_proc_v3.ProcessingMode_BUFFERED,
			ResponseHeaderMode:  envoy_http_ext_proc_v3.ProcessingMode_SKIP,
			ResponseBodyMode:    envoy_http_ext_proc_v3.ProcessingMode_NONE,
			RequestTrailerMode:  envoy_http_ext_proc_v3.ProcessingMode_SKIP,
			ResponseTrailerMode: envoy_http_ext_proc_v3.ProcessingMode_SKIP,
		},
	}
	return makeEnvoyHTTPFilter("envoy.filters.http.ext_proc", extProc)
}

// makeInferenceUpstreamExtProcHTTPFilter builds the UPSTREAM (post-route) ext_proc
// HTTP filter attached to each backend model cluster. It streams to the same
// processor over the same UDS, but runs after Envoy selected the backend, so it
// receives the chosen cluster's metadata (adapter + model) and name via request
// attributes. The processor uses that to transform the canonical request into the
// backend's native schema and to normalize/meter the response. Request and response
// bodies are BUFFERED for full-body transform; AllowModeOverride lets the processor
// flip the response to STREAMED per-response for server-sent-event completions.
func makeInferenceUpstreamExtProcHTTPFilter(cfg *structs.AIGatewayConfigEntry) (*envoy_http_v3.HttpFilter, error) {
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
		// The selected backend cluster's consul.ai metadata (adapter + model) names
		// the transform, and the cluster name attributes the metered usage. Both are
		// evaluated against the post-route selection, which the downstream phase
		// cannot see; this is why the transform must run as an upstream filter.
		RequestAttributes: []string{"xds.cluster_metadata", "xds.cluster_name"},
	}
	return makeEnvoyHTTPFilter("envoy.filters.http.ext_proc", extProc)
}

// makeInferenceUpstreamCodecHTTPFilter builds the terminal upstream_codec filter.
// An upstream HTTP filter chain must end in the codec (it replaces the router,
// which does not run in the upstream chain), so it follows the ext_proc filter.
func makeInferenceUpstreamCodecHTTPFilter() (*envoy_http_v3.HttpFilter, error) {
	return makeEnvoyHTTPFilter("envoy.filters.http.upstream_codec", &envoy_upstream_codec_v3.UpstreamCodec{})
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

// makeInferenceClusterMetadata renders a backend model cluster's routing facts —
// the wire adapter to transform/normalize with and the concrete model to stamp —
// into cluster FilterMetadata under "consul.ai". The upstream ext_proc filter
// receives this as the xds.cluster_metadata request attribute once Envoy selects
// the cluster, so the processor learns which adapter to transform with without
// ever routing itself.
func makeInferenceClusterMetadata(m *proxycfg.InferenceGatewayModel) *envoy_core_v3.Metadata {
	return &envoy_core_v3.Metadata{
		FilterMetadata: map[string]*structpb.Struct{
			inferenceListenerMetadataNamespace: {
				Fields: map[string]*structpb.Value{
					"model":   structpb.NewStringValue(m.Labels[inferenceModelFamilyLabel]),
					"adapter": structpb.NewStringValue(m.Labels[inferenceModelAPILabel]),
				},
			},
		},
	}
}

// clustersFromSnapshotInferenceGateway builds the local ext_proc cluster (UDS)
// and an EDS cluster per discovered model upstream. Each model cluster carries its
// consul.ai metadata (adapter + model) and an upstream HTTP filter chain
// (ext_proc → upstream_codec) so the processor transforms to the backend's native
// schema after Envoy routes to it.
func (s *ResourceGenerator) clustersFromSnapshotInferenceGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	ig := &cfgSnap.InferenceGateway

	res := make([]proto.Message, 0, len(ig.Models)+1)

	if ig.GatewayConfig != nil && ig.GatewayConfig.Processor.UDSPath != "" {
		res = append(res, makeInferenceExtProcCluster(ig.GatewayConfig.Processor.UDSPath))
	}

	upstreamOpts, err := makeInferenceUpstreamHTTPOptions(ig.GatewayConfig)
	if err != nil {
		return nil, err
	}

	for _, sn := range sortedModelNames(ig.Models) {
		cluster := s.makeGatewayCluster(cfgSnap, clusterOpts{name: sn.Name})
		cluster.Metadata = makeInferenceClusterMetadata(ig.Models[sn])
		cluster.TypedExtensionProtocolOptions = map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": upstreamOpts,
		}
		res = append(res, cluster)
	}

	return res, nil
}

// makeInferenceUpstreamHTTPOptions builds the HttpProtocolOptions attached to each
// backend model cluster: an explicit HTTP/1.1 upstream with the upstream HTTP
// filter chain ext_proc → upstream_codec. The ext_proc filter runs the provider
// transform post-route; the terminal upstream_codec encodes the request onto the
// upstream connection (it replaces the router, which does not run upstream). The
// options are identical across model clusters, so they are built once and shared.
func makeInferenceUpstreamHTTPOptions(cfg *structs.AIGatewayConfigEntry) (*anypb.Any, error) {
	extProc, err := makeInferenceUpstreamExtProcHTTPFilter(cfg)
	if err != nil {
		return nil, err
	}
	codec, err := makeInferenceUpstreamCodecHTTPFilter()
	if err != nil {
		return nil, err
	}

	opts := &envoy_upstreams_http_v3.HttpProtocolOptions{
		UpstreamProtocolOptions: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{
					HttpProtocolOptions: &envoy_core_v3.Http1ProtocolOptions{},
				},
			},
		},
		HttpFilters: []*envoy_http_v3.HttpFilter{extProc, codec},
	}
	return anypb.New(opts)
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

// routesForInferenceGateway builds the RDS route config: the downstream ext_proc
// phase promotes the request body's model to x-ai-model, and the gateway routes
// that model to the serving backend cluster, with the routing policy's default
// fallback as the catch-all. The upstream ext_proc filter on the selected cluster
// then transforms to that backend's native schema.
func (s *ResourceGenerator) routesForInferenceGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	ig := &cfgSnap.InferenceGateway

	vh := &envoy_route_v3.VirtualHost{
		Name:    inferenceGatewayListenerName,
		Domains: []string{"*"},
	}

	// Model routes: match the x-ai-model header (the body model the downstream
	// phase promoted) against each model's `model_family` catalog meta and route to
	// that model's cluster. A trailing "*" in model_family is a prefix match
	// (e.g. "gpt-4*"); a model without model_family falls back to its service name.
	for _, sn := range sortedModelNames(ig.Models) {
		vh.Routes = append(vh.Routes, &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/"},
				Headers: []*envoy_route_v3.HeaderMatcher{{
					Name: inferenceModelHeader,
					HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_StringMatch{
						StringMatch: inferenceModelMatcher(ig.Models[sn], sn),
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

	// Capability routes: match the caller's x-inference-specialization header
	// against each model's `capabilities` catalog meta and route to that model's
	// cluster. Selection is Envoy's; the upstream ext_proc filter on the cluster
	// reads that cluster's consul.ai metadata to transform. These come after the
	// x-ai-model routes and before the catch-all.
	for _, cap := range sortedCapabilities(ig.Models) {
		sn := capabilityModel(ig.Models, cap)
		vh.Routes = append(vh.Routes, &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/"},
				Headers: []*envoy_route_v3.HeaderMatcher{{
					Name: inferenceSpecializationHeader,
					HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_StringMatch{
						StringMatch: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{Exact: cap},
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

// inferenceModelMatcher builds the x-ai-model header matcher for a model route. It
// matches the model's `model_family` catalog meta: a trailing "*" is a prefix
// match (e.g. "gpt-4*" serves gpt-4, gpt-4o, ...), otherwise an exact match. A
// model without model_family falls back to an exact match on its service name so
// it stays reachable.
func inferenceModelMatcher(m *proxycfg.InferenceGatewayModel, sn structs.ServiceName) *envoy_matcher_v3.StringMatcher {
	family := m.Labels[inferenceModelFamilyLabel]
	if family == "" {
		return &envoy_matcher_v3.StringMatcher{
			MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{Exact: sn.Name},
		}
	}
	if strings.HasSuffix(family, "*") {
		return &envoy_matcher_v3.StringMatcher{
			MatchPattern: &envoy_matcher_v3.StringMatcher_Prefix{Prefix: strings.TrimSuffix(family, "*")},
		}
	}
	return &envoy_matcher_v3.StringMatcher{
		MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{Exact: family},
	}
}

// sortedCapabilities returns the distinct `capabilities` meta values advertised
// by the discovered models, in deterministic order.
func sortedCapabilities(models map[structs.ServiceName]*proxycfg.InferenceGatewayModel) []string {
	seen := make(map[string]struct{})
	for _, m := range models {
		if cap := m.Labels[inferenceCapabilitiesLabel]; cap != "" {
			seen[cap] = struct{}{}
		}
	}
	caps := make([]string, 0, len(seen))
	for cap := range seen {
		caps = append(caps, cap)
	}
	sort.Strings(caps)
	return caps
}

// capabilityModel returns the model chosen to serve a capability. When more than
// one model advertises it, the first by sorted service name wins (multi-member
// pools / failover are out of scope for the demo).
func capabilityModel(models map[structs.ServiceName]*proxycfg.InferenceGatewayModel, cap string) structs.ServiceName {
	for _, sn := range sortedModelNames(models) {
		if models[sn].Labels[inferenceCapabilitiesLabel] == cap {
			return sn
		}
	}
	return structs.ServiceName{}
}

func sortedModelNames(models map[structs.ServiceName]*proxycfg.InferenceGatewayModel) []structs.ServiceName {
	names := make([]structs.ServiceName, 0, len(models))
	for sn := range models {
		names = append(names, sn)
	}
	sort.Slice(names, func(i, j int) bool { return names[i].String() < names[j].String() })
	return names
}
