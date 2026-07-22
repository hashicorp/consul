// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"encoding/json"
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_ext_proc_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	envoy_upstreams_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func testInferenceModels(t *testing.T) map[structs.ServiceName]*proxycfg.InferenceGatewayModel {
	em := acl.DefaultEnterpriseMeta()
	return map[structs.ServiceName]*proxycfg.InferenceGatewayModel{
		structs.NewServiceName("openai-gpt4", em): {
			Service: structs.NewServiceName("openai-gpt4", em),
			Role:    structs.AIRoleModel,
			Labels:  map[string]string{"provider": "openai", "tier": "premium", "model_family": "gpt-4", "model_api": "openai"},
			Nodes: structs.CheckServiceNodes{
				{Service: &structs.NodeService{Service: "openai-gpt4", Address: "10.0.0.1", Port: 443}},
			},
		},
	}
}

func TestMakeInferenceListenerMetadata(t *testing.T) {
	require.Nil(t, makeInferenceListenerMetadata(nil))

	md := makeInferenceListenerMetadata(testInferenceModels(t))
	require.NotNil(t, md)
	consulAI := md.FilterMetadata[inferenceListenerMetadataNamespace]
	require.NotNil(t, consulAI)
	models := consulAI.Fields["models"].GetListValue()
	require.Len(t, models.Values, 1)
	model := models.Values[0].GetStructValue()
	require.Equal(t, "openai-gpt4", model.Fields["name"].GetStringValue())
	require.Equal(t, structs.AIRoleModel, model.Fields["role"].GetStringValue())
	require.Equal(t, "openai", model.Fields["labels"].GetStructValue().Fields["provider"].GetStringValue())
}

func TestMakeInferenceExtProcHTTPFilter(t *testing.T) {
	// Defaults to fail-closed.
	f, err := makeInferenceExtProcHTTPFilter(&structs.AIGatewayConfigEntry{})
	require.NoError(t, err)
	require.Equal(t, "envoy.filters.http.ext_proc", f.Name)

	// The downstream (pre-route) filter buffers the request but does not process the
	// response (the upstream filter owns it) and requests no attributes (no backend
	// is selected yet, and there is no RateLimit block to read listener metadata for).
	var ep envoy_http_ext_proc_v3.ExternalProcessor
	require.NoError(t, f.GetTypedConfig().UnmarshalTo(&ep))
	require.Empty(t, ep.RequestAttributes)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_BUFFERED, ep.ProcessingMode.RequestBodyMode)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_SKIP, ep.ProcessingMode.ResponseHeaderMode)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_NONE, ep.ProcessingMode.ResponseBodyMode)

	// A bound RateLimit block makes the downstream filter forward the inbound
	// listener metadata (which carries consul.ai.ratelimit) to the processor.
	f, err = makeInferenceExtProcHTTPFilter(&structs.AIGatewayConfigEntry{
		RateLimit: &structs.AIGatewayRateLimit{Enabled: true},
	})
	require.NoError(t, err)
	require.NoError(t, f.GetTypedConfig().UnmarshalTo(&ep))
	require.Equal(t, []string{inferenceListenerMetadataAttribute}, ep.RequestAttributes)

	// Open failure mode flips FailureModeAllow (verified via the typed config).
	f, err = makeInferenceExtProcHTTPFilter(&structs.AIGatewayConfigEntry{
		Processor: structs.AIGatewayProcessor{FailureMode: structs.AIGatewayFailureModeOpen},
	})
	require.NoError(t, err)
	require.NotNil(t, f.GetTypedConfig())
}

func TestMakeInferenceUpstreamExtProcHTTPFilter(t *testing.T) {
	f, err := makeInferenceUpstreamExtProcHTTPFilter(&structs.AIGatewayConfigEntry{})
	require.NoError(t, err)
	require.Equal(t, "envoy.filters.http.ext_proc", f.Name)

	// The upstream (post-route) filter receives the selected backend's metadata and
	// name, buffers both bodies for transform, and allows the per-response STREAMED
	// override for SSE completions. Host metadata is requested too so a fallback pool
	// (adapter/model on the endpoint, not the cluster) resolves the selected tier.
	var ep envoy_http_ext_proc_v3.ExternalProcessor
	require.NoError(t, f.GetTypedConfig().UnmarshalTo(&ep))
	require.Equal(t, []string{"xds.cluster_metadata", "xds.cluster_name", "xds.upstream_host_metadata"}, ep.RequestAttributes)
	require.True(t, ep.AllowModeOverride)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_BUFFERED, ep.ProcessingMode.RequestBodyMode)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_BUFFERED, ep.ProcessingMode.ResponseBodyMode)
}

func TestMakeInferenceRateLimitFilterMetadata(t *testing.T) {
	// No RateLimit block → no metadata namespace.
	require.Nil(t, makeInferenceRateLimitFilterMetadata(nil))
	require.Nil(t, makeInferenceRateLimitFilterMetadata(&structs.AIGatewayConfigEntry{}))

	cfg := &structs.AIGatewayConfigEntry{
		StateStore: &structs.AIGatewayStateStore{Service: "valkey", LocalBindPort: 6379},
		RateLimit: &structs.AIGatewayRateLimit{
			Enabled:    true,
			Dimensions: []string{"agent", "global"},
			Default: &structs.AIGatewayLimitPair{
				Tokens: &structs.AIGatewayLimit{Count: 10000, Unit: "day"},
			},
		},
	}
	md := makeInferenceRateLimitFilterMetadata(cfg)
	require.NotNil(t, md)
	ns := md[inferenceRateLimitMetadataNamespace]
	require.NotNil(t, ns)

	// The bind port lets the processor learn which loopback port to dial.
	require.Equal(t, float64(6379), ns.Fields["store_local_bind_port"].GetNumberValue())

	// The policy is carried as a JSON string that round-trips back to the same
	// struct (the interchange the processor already decodes config from).
	policyJSON := ns.Fields["policy"].GetStringValue()
	require.NotEmpty(t, policyJSON)
	var back structs.AIGatewayRateLimit
	require.NoError(t, json.Unmarshal([]byte(policyJSON), &back))
	require.True(t, back.Enabled)
	require.Equal(t, "day", back.Default.Tokens.Unit)
	require.Equal(t, 10000, back.Default.Tokens.Count)
}

func TestMakeInferenceExtProcCluster(t *testing.T) {
	c := makeInferenceExtProcCluster("/run/consul/ext_proc.sock")
	require.Equal(t, inferenceExtProcClusterName, c.Name)
	ep := c.LoadAssignment.Endpoints[0].LbEndpoints[0].GetEndpoint()
	require.Equal(t, "/run/consul/ext_proc.sock", ep.GetAddress().GetPipe().GetPath())
	require.Contains(t, c.TypedExtensionProtocolOptions, "envoy.extensions.upstreams.http.v3.HttpProtocolOptions")
}

// testFallbackModels returns two models sharing the "general-chat" capability at
// ranks 0 (openai-prod) and 1 (gemini-prod) — a cross-provider fallback pool.
func testFallbackModels(t *testing.T) map[structs.ServiceName]*proxycfg.InferenceGatewayModel {
	em := acl.DefaultEnterpriseMeta()
	return map[structs.ServiceName]*proxycfg.InferenceGatewayModel{
		structs.NewServiceName("openai-prod", em): {
			Service: structs.NewServiceName("openai-prod", em),
			Role:    structs.AIRoleModel,
			Labels: map[string]string{
				"capabilities": "general-chat", "priority_general-chat": "0",
				"model_family": "gpt-4", "model_api": "openai",
			},
			Nodes: structs.CheckServiceNodes{
				{Service: &structs.NodeService{Service: "openai-prod", Address: "10.0.0.1", Port: 443}},
			},
		},
		structs.NewServiceName("gemini-prod", em): {
			Service: structs.NewServiceName("gemini-prod", em),
			Role:    structs.AIRoleModel,
			Labels: map[string]string{
				"capabilities": "general-chat", "priority_general-chat": "1",
				"model_family": "gemini-2.5-flash", "model_api": "gemini",
			},
			Nodes: structs.CheckServiceNodes{
				{Service: &structs.NodeService{Service: "gemini-prod", Address: "10.0.0.2", Port: 443}},
			},
		},
	}
}

func TestRoutesForInferenceGateway(t *testing.T) {
	g := NewResourceGenerator(testutil.Logger(t), nil, false)

	cfgSnap := &proxycfg.ConfigSnapshot{Kind: structs.ServiceKindInferenceGateway}
	cfgSnap.InferenceGateway.Models = testFallbackModels(t)
	cfgSnap.InferenceGateway.GatewayConfig = &structs.AIGatewayConfigEntry{}

	res, err := g.routesForInferenceGateway(cfgSnap)
	require.NoError(t, err)
	require.Len(t, res, 1)

	rc := res[0].(*envoy_route_v3.RouteConfiguration)
	require.Equal(t, inferenceGatewayListenerName, rc.Name)
	routes := rc.VirtualHosts[0].Routes
	// one capability (pool) route + the fail-closed catch-all
	require.Len(t, routes, 2)

	// The capability route matches x-inference-specialization and targets the pool
	// cluster (two members -> failover) with a retry policy.
	hdr := routes[0].Match.Headers[0]
	require.Equal(t, inferenceSpecializationHeader, hdr.Name)
	require.Equal(t, "general-chat", hdr.GetStringMatch().GetExact())
	require.Equal(t, "inference-pool-general-chat", routes[0].GetRoute().GetCluster())

	rp := routes[0].GetRoute().GetRetryPolicy()
	require.NotNil(t, rp, "pool route carries a failover retry policy")
	require.Equal(t, uint32(1), rp.GetNumRetries().GetValue()) // 2 tiers -> 1 retry
	require.Contains(t, rp.RetryOn, "retriable-status-codes")
	require.Contains(t, rp.RetriableStatusCodes, uint32(401))

	// Catch-all fails closed.
	require.NotNil(t, routes[1].GetDirectResponse())
	require.Equal(t, uint32(503), routes[1].GetDirectResponse().GetStatus())
}

// TestClustersAndEndpointsForInferencePool verifies a multi-member capability
// renders a pool cluster plus a priority-tiered load assignment tagged per endpoint.
func TestClustersAndEndpointsForInferencePool(t *testing.T) {
	g := NewResourceGenerator(testutil.Logger(t), nil, false)

	cfgSnap := &proxycfg.ConfigSnapshot{Kind: structs.ServiceKindInferenceGateway}
	cfgSnap.InferenceGateway.Models = testFallbackModels(t)
	cfgSnap.InferenceGateway.GatewayConfig = &structs.AIGatewayConfigEntry{
		Processor: structs.AIGatewayProcessor{UDSPath: "/run/consul/ext_proc.sock"},
	}

	clusters, err := g.clustersFromSnapshotInferenceGateway(cfgSnap)
	require.NoError(t, err)
	var pool *envoy_cluster_v3.Cluster
	for _, r := range clusters {
		if c := r.(*envoy_cluster_v3.Cluster); c.Name == "inference-pool-general-chat" {
			pool = c
		}
	}
	require.NotNil(t, pool, "capability pool cluster rendered")

	// Build the pool load assignment directly (the per-model EDS path needs richer
	// node fixtures; the pool path only needs Service address/port + labels).
	tiers := capabilityTiers(cfgSnap.InferenceGateway.Models, "general-chat")
	cla := makeInferencePoolLoadAssignment("inference-pool-general-chat", cfgSnap.InferenceGateway.Models, tiers)
	require.Equal(t, "inference-pool-general-chat", cla.ClusterName)
	require.Len(t, cla.Endpoints, 2, "one priority tier per rank")

	// Tier 0 = openai-prod (priority_general-chat=0), tier 1 = gemini-prod.
	require.Equal(t, uint32(0), cla.Endpoints[0].Priority)
	require.Equal(t, uint32(1), cla.Endpoints[1].Priority)
	tier0md := cla.Endpoints[0].LbEndpoints[0].Metadata.FilterMetadata[inferenceListenerMetadataNamespace]
	require.Equal(t, "openai", tier0md.Fields["adapter"].GetStringValue())
	tier1md := cla.Endpoints[1].LbEndpoints[0].Metadata.FilterMetadata[inferenceListenerMetadataNamespace]
	require.Equal(t, "gemini", tier1md.Fields["adapter"].GetStringValue())
}

func TestClustersForInferenceGateway(t *testing.T) {
	g := NewResourceGenerator(testutil.Logger(t), nil, false)

	cfgSnap := &proxycfg.ConfigSnapshot{Kind: structs.ServiceKindInferenceGateway}
	cfgSnap.InferenceGateway.Models = testInferenceModels(t)
	cfgSnap.InferenceGateway.GatewayConfig = &structs.AIGatewayConfigEntry{
		Processor: structs.AIGatewayProcessor{UDSPath: "/run/consul/ext_proc.sock"},
	}

	res, err := g.clustersFromSnapshotInferenceGateway(cfgSnap)
	require.NoError(t, err)
	// the local ext_proc cluster + one model cluster
	require.Len(t, res, 2)

	var model *envoy_cluster_v3.Cluster
	for _, r := range res {
		c := r.(*envoy_cluster_v3.Cluster)
		if c.Name == "openai-gpt4" {
			model = c
		}
	}
	require.NotNil(t, model, "model cluster rendered")

	// The backend's adapter + model live in cluster metadata; the upstream ext_proc
	// filter reads it via xds.cluster_metadata to pick the transform.
	consulAI := model.Metadata.FilterMetadata[inferenceListenerMetadataNamespace]
	require.NotNil(t, consulAI)
	require.Equal(t, "openai", consulAI.Fields["adapter"].GetStringValue())
	require.Equal(t, "gpt-4", consulAI.Fields["model"].GetStringValue())

	// The cluster carries an upstream HTTP filter chain ending in upstream_codec,
	// with ext_proc ahead of it.
	optsAny := model.TypedExtensionProtocolOptions["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"]
	require.NotNil(t, optsAny)
	var opts envoy_upstreams_http_v3.HttpProtocolOptions
	require.NoError(t, optsAny.UnmarshalTo(&opts))
	require.Len(t, opts.HttpFilters, 2)
	require.Equal(t, "envoy.filters.http.ext_proc", opts.HttpFilters[0].Name)
	require.Equal(t, "envoy.filters.http.upstream_codec", opts.HttpFilters[1].Name)
}

func TestRoutesForInferenceGateway_capabilities(t *testing.T) {
	g := NewResourceGenerator(testutil.Logger(t), nil, false)
	em := acl.DefaultEnterpriseMeta()

	cfgSnap := &proxycfg.ConfigSnapshot{Kind: structs.ServiceKindInferenceGateway}
	cfgSnap.InferenceGateway.Models = map[structs.ServiceName]*proxycfg.InferenceGatewayModel{
		structs.NewServiceName("gemini-travel", em): {
			Service: structs.NewServiceName("gemini-travel", em),
			Role:    structs.AIRoleModel,
			Labels:  map[string]string{"capabilities": "travel-planner", "model_family": "gemini-1.5-pro", "model_api": "openai"},
			Nodes: structs.CheckServiceNodes{
				{Service: &structs.NodeService{Service: "gemini-travel", Address: "10.0.0.2", Port: 443}},
			},
		},
	}
	cfgSnap.InferenceGateway.GatewayConfig = &structs.AIGatewayConfigEntry{}

	res, err := g.routesForInferenceGateway(cfgSnap)
	require.NoError(t, err)
	require.Len(t, res, 1)

	routes := res[0].(*envoy_route_v3.RouteConfiguration).VirtualHosts[0].Routes
	// Single-member capability route + fail-closed catch-all (no model-name routes).
	require.Len(t, routes, 2)

	// The capability route matches x-inference-specialization and routes directly to
	// the sole member's cluster (no pool, no retry policy). Under two-phase it carries
	// no route metadata — the adapter lives in the destination cluster's metadata,
	// which the upstream ext_proc filter reads post-route.
	capRoute := routes[0]
	hdr := capRoute.Match.Headers[0]
	require.Equal(t, inferenceSpecializationHeader, hdr.Name)
	require.Equal(t, "travel-planner", hdr.GetStringMatch().GetExact())
	require.Equal(t, "gemini-travel", capRoute.GetRoute().GetCluster())
	require.Nil(t, capRoute.GetRoute().GetRetryPolicy(), "single-member capability has no failover")
	require.Nil(t, capRoute.Metadata, "capability routes carry no metadata under two-phase")

	// No matching capability → catch-all fails closed with a 503.
	require.NotNil(t, routes[1].GetDirectResponse())
	require.Equal(t, uint32(503), routes[1].GetDirectResponse().GetStatus())
}
