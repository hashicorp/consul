// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"encoding/json"
	"sort"
	"strconv"
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
	envoy_retry_host_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/retry/host/previous_hosts/v3"
	envoy_retry_priority_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/retry/priority/previous_priorities/v3"
	envoy_upstreams_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/response"
)

const (
	// inferenceGatewayListenerName is the name of the inference gateway's single
	// inbound listener and its RDS route configuration.
	inferenceGatewayListenerName = "inference-gateway"

	// inferenceExtProcClusterName is the local cluster Envoy uses to reach the
	// co-located policy processor over the loopback/UDS socket.
	inferenceExtProcClusterName = "local_ext_proc"

	// inferenceStateStoreListenerName is the outbound TCP listener the processor
	// dials on 127.0.0.1:StateStore.LocalBindPort to reach the rate-limit counter
	// store; Envoy proxies it over mTLS (intention-gated) to the store mesh service.
	inferenceStateStoreListenerName = "inference-state-store"

	// inferenceRateLimitMetadataNamespace is the listener FilterMetadata key
	// carrying the rate-limit policy for the policy processor. It is a DISTINCT
	// namespace from the model catalog (inferenceListenerMetadataNamespace): the
	// processor reads it off the xds.listener_metadata request attribute, hashes it,
	// and recompiles its limiter policy on change — decoupled from model discovery.
	inferenceRateLimitMetadataNamespace = "consul.ai.ratelimit"

	// inferenceListenerMetadataAttribute is the ext_proc request attribute carrying
	// the inbound listener's metadata (a text-format core.Metadata string). The
	// downstream filter forwards it so the processor can read the rate-limit policy
	// from the consul.ai.ratelimit namespace pre-route. Static listener metadata is
	// not dynamic metadata, so this attribute — not metadata_context — is its channel.
	inferenceListenerMetadataAttribute = "xds.listener_metadata"

	// inferenceSpecializationHeader is the caller-supplied header naming a
	// required capability. Consul renders a native header-match route per
	// discovered capability, so selection happens in Envoy without the policy
	// processor emitting a cluster. Capability selection is the sole routing
	// dimension: there are no model-name routes (a concrete model name is
	// provider-specific and must not silently fail over to another provider).
	inferenceSpecializationHeader = "x-inference-specialization"

	// inferenceCapabilitiesLabel is the model catalog meta key whose value is the
	// comma-separated SET of capabilities a model advertises (matched against
	// inferenceSpecializationHeader). A capability with two or more members renders
	// as a priority pool; membership order comes from inferencePriorityLabelPrefix.
	inferenceCapabilitiesLabel = "capabilities"

	// inferencePriorityLabelPrefix is the prefix of the per-capability rank meta key
	// ("priority_<capability>"). It carries the model's standing on the (capability,
	// model) edge: lower = higher priority (tier 0 = primary); equal ranks share a
	// tier (active/active); a missing rank sorts last. This is edge-scoped so one
	// model can be primary for one capability and a backup for another. Underscore
	// (not dot) because Consul service-meta keys disallow dots.
	inferencePriorityLabelPrefix = "priority_"

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

	// inferencePoolClusterPrefix is the name prefix of the per-capability "pool"
	// cluster rendered for a capability with two or more discovered members: one
	// priority tier per rank group, each endpoint carrying that member's consul.ai
	// {model, adapter} metadata. The capability route targets it with a retry policy,
	// so a retriable failure fails over to the next tier — a different provider — and
	// the upstream ext_proc filter transforms for whichever endpoint Envoy selected.
	// This is the Envoy AI Gateway pattern (per-endpoint backend metadata); an
	// aggregate cluster cannot be used because Envoy does not run a member cluster's
	// upstream HTTP filters when routing through the aggregate, so the transform would
	// be skipped.
	inferencePoolClusterPrefix = "inference-pool-"
)

// inferencePoolClusterName is the pool cluster name for a capability.
func inferencePoolClusterName(capability string) string {
	return inferencePoolClusterPrefix + sanitizeInferencePoolName(capability)
}

// sanitizeInferencePoolName maps a capability to an Envoy-safe cluster-name suffix
// (lower-case alphanumerics and '-'; other runes become '-').
func sanitizeInferencePoolName(capability string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(capability) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

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

	// Surface the discovered model catalog and (separately) the rate-limit policy
	// to the processor via listener metadata — distinct FilterMetadata namespaces.
	l.Metadata = makeInferenceListenerMetadata(ig.Models)
	if rlMD := makeInferenceRateLimitFilterMetadata(ig.GatewayConfig); rlMD != nil {
		if l.Metadata == nil {
			l.Metadata = &envoy_core_v3.Metadata{FilterMetadata: map[string]*structpb.Struct{}}
		}
		for k, v := range rlMD {
			l.Metadata.FilterMetadata[k] = v
		}
	}

	// Attach the mesh mTLS downstream context (RequireClientCertificate + SPIFFE).
	if err := s.finalizePublicListenerFromConfig(l, cfgSnap, true); err != nil {
		return nil, err
	}

	out := []proto.Message{l}

	// When a rate-limit StateStore is bound, add the outbound TCP listener the
	// processor dials locally; Envoy proxies it over mTLS to the store mesh service.
	storeListener, err := s.makeInferenceStateStoreListener(cfgSnap)
	if err != nil {
		return nil, err
	}
	if storeListener != nil {
		out = append(out, storeListener)
	}

	return out, nil
}

// makeInferenceStateStoreListener builds the outbound TCP listener the processor
// dials on 127.0.0.1:StateStore.LocalBindPort to reach the rate-limit counter
// store. A tcp_proxy forwards to the store's mesh-upstream cluster (mTLS applied
// on the cluster's transport socket). Returns nil when no StateStore with a bind
// port is bound.
func (s *ResourceGenerator) makeInferenceStateStoreListener(cfgSnap *proxycfg.ConfigSnapshot) (*envoy_listener_v3.Listener, error) {
	ig := &cfgSnap.InferenceGateway
	sn := ig.StateStoreService
	if sn.Name == "" || ig.GatewayConfig == nil || ig.GatewayConfig.StateStore == nil {
		return nil, nil
	}
	port := ig.GatewayConfig.StateStore.LocalBindPort
	if port == 0 {
		return nil, nil
	}

	l := makeListener(makeListenerOpts{
		name:       inferenceStateStoreListenerName,
		accessLogs: cfgSnap.Proxy.AccessLogs,
		addr:       "127.0.0.1",
		port:       port,
		direction:  envoy_core_v3.TrafficDirection_OUTBOUND,
		logger:     s.Logger,
	})

	filter, err := makeTCPProxyFilter(listenerFilterOpts{
		accessLogs: &cfgSnap.Proxy.AccessLogs,
		cluster:    inferenceStateStoreSNI(cfgSnap, sn),
		filterName: inferenceStateStoreListenerName,
		statPrefix: "upstream.",
		logger:     s.Logger,
	})
	if err != nil {
		return nil, err
	}
	l.FilterChains = []*envoy_listener_v3.FilterChain{{
		Filters: []*envoy_listener_v3.Filter{filter},
	}}
	return l, nil
}

// makeInferenceRateLimitFilterMetadata renders the bound entry's RateLimit policy
// into a consul.ai.ratelimit FilterMetadata namespace for the processor. The
// policy is carried as a JSON string ("policy") — the same interchange the
// processor already decodes the config entry from, so the full nested block rides
// along without a hand-written struct mirror — plus the store's local bind port so
// the processor knows which loopback port to dial. Returns nil when the entry
// declares no RateLimit block.
func makeInferenceRateLimitFilterMetadata(cfg *structs.AIGatewayConfigEntry) map[string]*structpb.Struct {
	if cfg == nil || cfg.RateLimit == nil {
		return nil
	}
	policyJSON, err := json.Marshal(cfg.RateLimit)
	if err != nil {
		return nil
	}
	fields := map[string]*structpb.Value{
		"policy": structpb.NewStringValue(string(policyJSON)),
	}
	if cfg.StateStore != nil && cfg.StateStore.LocalBindPort != 0 {
		fields["store_local_bind_port"] = structpb.NewNumberValue(float64(cfg.StateStore.LocalBindPort))
	}
	return map[string]*structpb.Struct{
		inferenceRateLimitMetadataNamespace: {Fields: fields},
	}
}

// inferenceStateStoreSNI is the store mesh-upstream cluster/SNI name, following the
// standard Connect-upstream convention so its EDS load assignment (same name) and
// SAN validation line up.
func inferenceStateStoreSNI(cfgSnap *proxycfg.ConfigSnapshot, sn structs.ServiceName) string {
	return connect.ServiceSNI(sn.Name, "", sn.NamespaceOrDefault(), sn.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
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
		// Forward the inbound listener's metadata (which carries the rate-limit policy
		// under consul.ai.ratelimit) to the processor via the xds.listener_metadata
		// request attribute, so the downstream CHECK phase compiles its limiter from
		// live config with no side fetch. Requested only when the entry declares a
		// RateLimit block; the attribute is a text-format core.Metadata string, read
		// the same way as xds.cluster_metadata on the upstream filter. Static listener
		// metadata is NOT dynamic metadata, so metadata_context/forwarding_namespaces
		// cannot carry it — the request attribute is the correct channel.
		RequestAttributes: rateLimitRequestAttributes(cfg),
	}
	return makeEnvoyHTTPFilter("envoy.filters.http.ext_proc", extProc)
}

// rateLimitRequestAttributes asks ext_proc to forward the inbound listener's
// metadata (xds.listener_metadata) so the processor can read the consul.ai.ratelimit
// policy off it. Returns nil when the entry declares no RateLimit block, so the
// filter is byte-identical to before for non-rate-limited gateways.
func rateLimitRequestAttributes(cfg *structs.AIGatewayConfigEntry) []string {
	if cfg == nil || cfg.RateLimit == nil {
		return nil
	}
	return []string{inferenceListenerMetadataAttribute}
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
		// The selected backend's consul.ai metadata (adapter + model) names the
		// transform, and the cluster name attributes the metered usage. Both are
		// evaluated against the post-route selection, which the downstream phase
		// cannot see; this is why the transform must run as an upstream filter. A
		// single-backend cluster carries the metadata on the cluster; a fallback pool
		// carries it on the selected ENDPOINT (each priority tier is a different
		// provider), so upstream_host_metadata is requested too and the processor
		// falls back to it when the cluster metadata is absent.
		RequestAttributes: []string{"xds.cluster_metadata", "xds.cluster_name", "xds.upstream_host_metadata"},
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

	// The rate-limit counter store, reached as a Connect mesh upstream (mTLS).
	if sn := ig.StateStoreService; sn.Name != "" {
		storeCluster, err := s.makeInferenceStateStoreCluster(cfgSnap, sn)
		if err != nil {
			return nil, err
		}
		res = append(res, storeCluster)
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

	// Render one pool cluster per capability that has two or more discovered
	// members. It carries the same upstream ext_proc chain but NO cluster-level
	// consul.ai metadata: the adapter/model live on each endpoint (per priority
	// tier), so the processor resolves them from the selected host's metadata.
	for _, capability := range sortedCapabilities(ig.Models) {
		if tiers := capabilityTiers(ig.Models, capability); tierMemberCount(tiers) >= 2 {
			pool := s.makeGatewayCluster(cfgSnap, clusterOpts{name: inferencePoolClusterName(capability)})
			pool.TypedExtensionProtocolOptions = map[string]*anypb.Any{
				"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": upstreamOpts,
			}
			res = append(res, pool)
		}
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

// makeInferenceStateStoreCluster builds the mesh-upstream EDS cluster for the
// rate-limit counter store. Unlike the terminating-gateway model clusters this is
// a Connect upstream: a plain TCP cluster whose transport socket carries the
// gateway's client cert and validates the store's SPIFFE SAN, so the loopback
// listener → store hop is mTLS and intention-gated. Endpoints arrive via EDS.
func (s *ResourceGenerator) makeInferenceStateStoreCluster(cfgSnap *proxycfg.ConfigSnapshot, sn structs.ServiceName) (*envoy_cluster_v3.Cluster, error) {
	sni := inferenceStateStoreSNI(cfgSnap, sn)
	transportSocket, err := makeMTLSTransportSocket(cfgSnap, proxycfg.NewUpstreamIDFromServiceName(sn), sni)
	if err != nil {
		return nil, err
	}
	return &envoy_cluster_v3.Cluster{
		Name:                 sni,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS},
		EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
			EdsConfig: &envoy_core_v3.ConfigSource{
				InitialFetchTimeout: cfgSnap.GetXDSCommonConfig(s.Logger).GetXDSFetchTimeout(),
				ResourceApiVersion:  envoy_core_v3.ApiVersion_V3,
				ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
					Ads: &envoy_core_v3.AggregatedConfigSource{},
				},
			},
		},
		TransportSocket: transportSocket,
	}, nil
}

// endpointsFromSnapshotInferenceGateway builds the EDS load assignments for the
// discovered model upstreams and (when bound) the rate-limit counter store.
func (s *ResourceGenerator) endpointsFromSnapshotInferenceGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	ig := &cfgSnap.InferenceGateway

	res := make([]proto.Message, 0, len(ig.Models)+1)

	if sn := ig.StateStoreService; sn.Name != "" && len(ig.StateStoreNodes) > 0 {
		res = append(res, makeLoadAssignment(
			s.Logger,
			cfgSnap,
			inferenceStateStoreSNI(cfgSnap, sn),
			nil,
			[]loadAssignmentEndpointGroup{{Endpoints: ig.StateStoreNodes}},
			cfgSnap.Locality,
		))
	}

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

	// Each capability pool's load assignment: one priority tier per rank group with
	// per-endpoint provider metadata (see makeInferencePoolLoadAssignment).
	for _, capability := range sortedCapabilities(ig.Models) {
		if tiers := capabilityTiers(ig.Models, capability); tierMemberCount(tiers) >= 2 {
			res = append(res, makeInferencePoolLoadAssignment(inferencePoolClusterName(capability), ig.Models, tiers))
		}
	}
	return res, nil
}

// routesForInferenceGateway builds the RDS route config. Selection is by capability
// only: the caller's x-inference-specialization header is matched against each
// discovered capability. A capability with one member routes directly to that
// model's cluster; a capability with two or more members routes to its priority
// pool cluster with a retry policy, so a retriable failure fails over to the next
// tier (a different provider). The upstream ext_proc filter on the selected
// cluster/endpoint then transforms to that backend's native schema. A request with
// no matching capability hits the catch-all and fails closed (503).
func (s *ResourceGenerator) routesForInferenceGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	ig := &cfgSnap.InferenceGateway

	vh := &envoy_route_v3.VirtualHost{
		Name:    inferenceGatewayListenerName,
		Domains: []string{"*"},
	}

	fallback := inferenceFallbackConfig(ig.GatewayConfig)
	for _, capability := range sortedCapabilities(ig.Models) {
		tiers := capabilityTiers(ig.Models, capability)
		if tierMemberCount(tiers) == 0 {
			continue
		}

		action := &envoy_route_v3.RouteAction{}
		if tierMemberCount(tiers) == 1 {
			// Single member: route directly to the model's own cluster (its
			// cluster-level consul.ai metadata drives the upstream transform).
			action.ClusterSpecifier = &envoy_route_v3.RouteAction_Cluster{Cluster: tiers[0][0].Name}
		} else {
			// Two or more members: route to the capability's priority pool with the
			// failover retry policy. len(tiers) is the number of priority tiers.
			action.ClusterSpecifier = &envoy_route_v3.RouteAction_Cluster{Cluster: inferencePoolClusterName(capability)}
			action.RetryPolicy = inferenceFallbackRetryPolicy(len(tiers), fallback)
		}

		vh.Routes = append(vh.Routes, &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/"},
				Headers: []*envoy_route_v3.HeaderMatcher{{
					Name: inferenceSpecializationHeader,
					HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_StringMatch{
						StringMatch: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{Exact: capability},
						},
					},
				}},
			},
			Action: &envoy_route_v3.Route_Route{Route: action},
		})
	}

	// Catch-all: no matching capability -> 503, fail closed.
	vh.Routes = append(vh.Routes, &envoy_route_v3.Route{
		Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/"}},
		Action: &envoy_route_v3.Route_DirectResponse{
			DirectResponse: &envoy_route_v3.DirectResponseAction{Status: 503},
		},
	})

	return []proto.Message{&envoy_route_v3.RouteConfiguration{
		Name:         inferenceGatewayListenerName,
		VirtualHosts: []*envoy_route_v3.VirtualHost{vh},
	}}, nil
}

// modelCapabilities returns the model's advertised capability set (the
// comma-separated `capabilities` meta, trimmed; empty entries dropped).
func modelCapabilities(m *proxycfg.InferenceGatewayModel) []string {
	raw := m.Labels[inferenceCapabilitiesLabel]
	if raw == "" {
		return nil
	}
	var caps []string
	for _, c := range strings.Split(raw, ",") {
		if c = strings.TrimSpace(c); c != "" {
			caps = append(caps, c)
		}
	}
	return caps
}

// modelHasCapability reports whether the model advertises the capability.
func modelHasCapability(m *proxycfg.InferenceGatewayModel, capability string) bool {
	for _, c := range modelCapabilities(m) {
		if c == capability {
			return true
		}
	}
	return false
}

// modelPriority returns the model's rank on the (capability, model) edge from the
// `priority.<capability>` meta, and whether it was set. Lower = higher priority.
func modelPriority(m *proxycfg.InferenceGatewayModel, capability string) (int, bool) {
	v, ok := m.Labels[inferencePriorityLabelPrefix+capability]
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, false
	}
	return n, true
}

// sortedCapabilities returns the distinct capabilities advertised across the
// discovered models (capabilities is a set), in deterministic order.
func sortedCapabilities(models map[structs.ServiceName]*proxycfg.InferenceGatewayModel) []string {
	seen := make(map[string]struct{})
	for _, m := range models {
		for _, c := range modelCapabilities(m) {
			seen[c] = struct{}{}
		}
	}
	caps := make([]string, 0, len(seen))
	for c := range seen {
		caps = append(caps, c)
	}
	sort.Strings(caps)
	return caps
}

// capabilityTiers groups the discovered models advertising `capability` into
// priority tiers. Models are ordered by their priority.<capability> rank (a missing
// rank sorts last); models sharing a rank form one tier (active/active, load
// balanced), and each distinct rank is a lower-priority failover tier. tiers[0] is
// the primary tier. A model can be primary for one capability and a backup for
// another because the rank is read per capability.
func capabilityTiers(models map[structs.ServiceName]*proxycfg.InferenceGatewayModel, capability string) [][]structs.ServiceName {
	type ranked struct {
		sn   structs.ServiceName
		rank int
		set  bool
	}
	var members []ranked
	for _, sn := range sortedModelNames(models) {
		if !modelHasCapability(models[sn], capability) {
			continue
		}
		rank, set := modelPriority(models[sn], capability)
		members = append(members, ranked{sn: sn, rank: rank, set: set})
	}
	if len(members) == 0 {
		return nil
	}
	sort.SliceStable(members, func(i, j int) bool {
		a, b := members[i], members[j]
		if a.set != b.set {
			return a.set // ranked before unranked
		}
		if a.set && a.rank != b.rank {
			return a.rank < b.rank
		}
		return a.sn.String() < b.sn.String()
	})

	sameTier := func(a, b ranked) bool {
		return a.set == b.set && (!a.set || a.rank == b.rank)
	}
	var tiers [][]structs.ServiceName
	for i, m := range members {
		if i == 0 || !sameTier(members[i-1], m) {
			tiers = append(tiers, nil)
		}
		tiers[len(tiers)-1] = append(tiers[len(tiers)-1], m.sn)
	}
	return tiers
}

// tierMemberCount is the total number of members across all tiers.
func tierMemberCount(tiers [][]structs.ServiceName) int {
	n := 0
	for _, t := range tiers {
		n += len(t)
	}
	return n
}

// makeInferencePoolLoadAssignment builds a capability pool's endpoints: one Envoy
// priority tier per rank group (tier i → priority i), each endpoint tagged with its
// member's consul.ai {model, adapter} so the upstream ext_proc filter (reading
// xds.upstream_host_metadata) transforms for whichever provider Envoy selected.
// Normal traffic goes to priority 0; the route's retry policy (previous_priorities)
// moves a retried request onto the next tier.
func makeInferencePoolLoadAssignment(clusterName string, models map[structs.ServiceName]*proxycfg.InferenceGatewayModel, tiers [][]structs.ServiceName) *envoy_endpoint_v3.ClusterLoadAssignment {
	cla := &envoy_endpoint_v3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   make([]*envoy_endpoint_v3.LocalityLbEndpoints, 0, len(tiers)),
	}
	for i, tier := range tiers {
		var es []*envoy_endpoint_v3.LbEndpoint
		for _, sn := range tier {
			m := models[sn]
			md := makeInferenceClusterMetadata(m)
			for _, ep := range m.Nodes {
				_, addr, port := ep.BestAddress(false)
				healthStatus, weight := calculateEndpointHealthAndWeight(ep, false)
				es = append(es, &envoy_endpoint_v3.LbEndpoint{
					HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
						Endpoint: &envoy_endpoint_v3.Endpoint{Address: response.MakeAddress(addr, port)},
					},
					HealthStatus:        healthStatus,
					LoadBalancingWeight: response.MakeUint32Value(weight),
					Metadata:            md,
				})
			}
		}
		cla.Endpoints = append(cla.Endpoints, &envoy_endpoint_v3.LocalityLbEndpoints{
			Priority:    uint32(i),
			LbEndpoints: es,
		})
	}
	return cla
}

// inferenceFallbackConfig returns the entry's fallback behavior block, or nil.
func inferenceFallbackConfig(cfg *structs.AIGatewayConfigEntry) *structs.AIGatewayFallback {
	if cfg == nil {
		return nil
	}
	return cfg.Routing.Fallback
}

// inferenceFallbackRetryPolicy builds the route retry policy that drives failover
// across a capability pool's priority tiers. A retriable upstream response is
// retried; the previous_priorities predicate excludes already-tried priorities
// (moving the retry to the next provider tier) and previous_hosts excludes
// already-tried hosts — so 401 is safe to retry (it lands on a different provider).
// numTiers-1 retries let a request walk every tier once, capped by MaxTiers.
// Behavior (retriable conditions, tier cap, per-try timeout) comes from the entry's
// Routing.Fallback block; an absent block uses the built-in defaults.
func inferenceFallbackRetryPolicy(numTiers int, fb *structs.AIGatewayFallback) *envoy_route_v3.RetryPolicy {
	retryOn, codes := inferenceRetryOn(fb)

	numRetries := numTiers - 1
	if fb != nil && fb.MaxTiers > 0 && fb.MaxTiers-1 < numRetries {
		numRetries = fb.MaxTiers - 1
	}
	if numRetries < 0 {
		numRetries = 0
	}

	prevPriorities, _ := anypb.New(&envoy_retry_priority_v3.PreviousPrioritiesConfig{UpdateFrequency: 1})
	prevHosts, _ := anypb.New(&envoy_retry_host_v3.PreviousHostsPredicate{})
	rp := &envoy_route_v3.RetryPolicy{
		RetryOn:                       retryOn,
		RetriableStatusCodes:          codes,
		NumRetries:                    wrapperspb.UInt32(uint32(numRetries)),
		HostSelectionRetryMaxAttempts: 5,
		RetryHostPredicate: []*envoy_route_v3.RetryPolicy_RetryHostPredicate{{
			Name:       "envoy.retry_host_predicates.previous_hosts",
			ConfigType: &envoy_route_v3.RetryPolicy_RetryHostPredicate_TypedConfig{TypedConfig: prevHosts},
		}},
		RetryPriority: &envoy_route_v3.RetryPolicy_RetryPriority{
			Name:       "envoy.retry_priorities.previous_priorities",
			ConfigType: &envoy_route_v3.RetryPolicy_RetryPriority_TypedConfig{TypedConfig: prevPriorities},
		},
	}
	if fb != nil && fb.PerTryTimeout != "" {
		if d, err := time.ParseDuration(fb.PerTryTimeout); err == nil {
			rp.PerTryTimeout = durationpb.New(d)
		}
	}
	return rp
}

// inferenceRetryOn translates the Fallback.RetryOn tokens into Envoy's retry_on
// string plus explicit retriable status codes. Numeric tokens ("401") become status
// codes (and imply "retriable-status-codes"); named tokens ("5xx", "reset",
// "connect-failure") pass through. An absent/empty block uses the default set.
func inferenceRetryOn(fb *structs.AIGatewayFallback) (string, []uint32) {
	if fb == nil || len(fb.RetryOn) == 0 {
		return "retriable-status-codes,5xx,reset,connect-failure", []uint32{401}
	}
	var tokens []string
	var codes []uint32
	hasCodes := false
	for _, t := range fb.RetryOn {
		if t = strings.TrimSpace(t); t == "" {
			continue
		}
		if n, err := strconv.Atoi(t); err == nil {
			codes = append(codes, uint32(n))
			hasCodes = true
			continue
		}
		tokens = append(tokens, t)
	}
	if hasCodes {
		found := false
		for _, t := range tokens {
			if t == "retriable-status-codes" {
				found = true
				break
			}
		}
		if !found {
			tokens = append([]string{"retriable-status-codes"}, tokens...)
		}
	}
	return strings.Join(tokens, ","), codes
}

func sortedModelNames(models map[structs.ServiceName]*proxycfg.InferenceGatewayModel) []structs.ServiceName {
	names := make([]structs.ServiceName, 0, len(models))
	for sn := range models {
		names = append(names, sn)
	}
	sort.Slice(names, func(i, j int) bool { return names[i].String() < names[j].String() })
	return names
}
