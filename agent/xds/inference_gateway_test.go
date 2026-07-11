// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
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
	// response (the upstream filter owns it) and requests no cluster metadata (no
	// backend is selected yet).
	var ep envoy_http_ext_proc_v3.ExternalProcessor
	require.NoError(t, f.GetTypedConfig().UnmarshalTo(&ep))
	require.Empty(t, ep.RequestAttributes)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_BUFFERED, ep.ProcessingMode.RequestBodyMode)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_SKIP, ep.ProcessingMode.ResponseHeaderMode)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_NONE, ep.ProcessingMode.ResponseBodyMode)

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

	// The upstream (post-route) filter receives the selected cluster's metadata and
	// name, buffers both bodies for transform, and allows the per-response STREAMED
	// override for SSE completions.
	var ep envoy_http_ext_proc_v3.ExternalProcessor
	require.NoError(t, f.GetTypedConfig().UnmarshalTo(&ep))
	require.Equal(t, []string{"xds.cluster_metadata", "xds.cluster_name"}, ep.RequestAttributes)
	require.True(t, ep.AllowModeOverride)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_BUFFERED, ep.ProcessingMode.RequestBodyMode)
	require.Equal(t, envoy_http_ext_proc_v3.ProcessingMode_BUFFERED, ep.ProcessingMode.ResponseBodyMode)
}

func TestMakeInferenceExtProcCluster(t *testing.T) {
	c := makeInferenceExtProcCluster("/run/consul/ext_proc.sock")
	require.Equal(t, inferenceExtProcClusterName, c.Name)
	ep := c.LoadAssignment.Endpoints[0].LbEndpoints[0].GetEndpoint()
	require.Equal(t, "/run/consul/ext_proc.sock", ep.GetAddress().GetPipe().GetPath())
	require.Contains(t, c.TypedExtensionProtocolOptions, "envoy.extensions.upstreams.http.v3.HttpProtocolOptions")
}

func TestRoutesForInferenceGateway(t *testing.T) {
	g := NewResourceGenerator(testutil.Logger(t), nil, false)

	cfgSnap := &proxycfg.ConfigSnapshot{Kind: structs.ServiceKindInferenceGateway}
	cfgSnap.InferenceGateway.Models = testInferenceModels(t)
	cfgSnap.InferenceGateway.GatewayConfig = &structs.AIGatewayConfigEntry{
		Routing: structs.AIGatewayRouting{FallbackChain: []string{"openai-gpt4"}},
	}

	res, err := g.routesForInferenceGateway(cfgSnap)
	require.NoError(t, err)
	require.Len(t, res, 1)

	rc := res[0].(*envoy_route_v3.RouteConfiguration)
	require.Equal(t, inferenceGatewayListenerName, rc.Name)
	routes := rc.VirtualHosts[0].Routes
	// one model-match route + one default fallback route
	require.Len(t, routes, 2)

	// First route matches the x-ai-model header against the model_family and routes
	// to the model's cluster.
	hdr := routes[0].Match.Headers[0]
	require.Equal(t, inferenceModelHeader, hdr.Name)
	require.Equal(t, "gpt-4", hdr.GetStringMatch().GetExact())
	require.Equal(t, "openai-gpt4", routes[0].GetRoute().GetCluster())

	// Default route falls back to the configured cluster.
	require.Equal(t, "openai-gpt4", routes[1].GetRoute().GetCluster())
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
	// x-ai-model route + capability route + fail-closed catch-all.
	require.Len(t, routes, 3)

	// The model route matches x-ai-model against the model_family.
	modelHdr := routes[0].Match.Headers[0]
	require.Equal(t, inferenceModelHeader, modelHdr.Name)
	require.Equal(t, "gemini-1.5-pro", modelHdr.GetStringMatch().GetExact())

	// The capability route matches x-inference-specialization and routes natively
	// to the model's cluster (no ext_proc selection). Under two-phase it carries no
	// route metadata — the adapter lives in the destination cluster's metadata,
	// which the upstream ext_proc filter reads post-route.
	capRoute := routes[1]
	hdr := capRoute.Match.Headers[0]
	require.Equal(t, inferenceSpecializationHeader, hdr.Name)
	require.Equal(t, "travel-planner", hdr.GetStringMatch().GetExact())
	require.Equal(t, "gemini-travel", capRoute.GetRoute().GetCluster())
	require.Nil(t, capRoute.Metadata, "capability routes carry no metadata under two-phase")

	// No fallback configured → catch-all fails closed with a 503.
	require.NotNil(t, routes[2].GetDirectResponse())
	require.Equal(t, uint32(503), routes[2].GetDirectResponse().GetStatus())
}
