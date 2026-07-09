// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_ext_proc_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
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
			Labels:  map[string]string{"provider": "openai", "tier": "premium"},
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

	// The selected route's metadata is forwarded to the processor so it can pin the
	// pipeline on a native capability route.
	var ep envoy_http_ext_proc_v3.ExternalProcessor
	require.NoError(t, f.GetTypedConfig().UnmarshalTo(&ep))
	require.Equal(t, []string{"xds.route_metadata"}, ep.RequestAttributes)

	// Open failure mode flips FailureModeAllow (verified via the typed config).
	f, err = makeInferenceExtProcHTTPFilter(&structs.AIGatewayConfigEntry{
		Processor: structs.AIGatewayProcessor{FailureMode: structs.AIGatewayFailureModeOpen},
	})
	require.NoError(t, err)
	require.NotNil(t, f.GetTypedConfig())
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
	// one header-match route for the model + one default fallback route
	require.Len(t, routes, 2)

	// First route matches on the x-ai-cluster header.
	hdr := routes[0].Match.Headers[0]
	require.Equal(t, inferenceClusterHeader, hdr.Name)
	require.Equal(t, "openai-gpt4", hdr.GetStringMatch().GetExact())
	require.Equal(t, "openai-gpt4", routes[0].GetRoute().GetCluster())

	// Default route falls back to the configured cluster.
	require.Equal(t, "openai-gpt4", routes[1].GetRoute().GetCluster())
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
	// x-ai-cluster route + capability route + fail-closed catch-all.
	require.Len(t, routes, 3)

	// The capability route matches x-inference-specialization and routes natively
	// to the model's cluster (no ext_proc selection).
	capRoute := routes[1]
	hdr := capRoute.Match.Headers[0]
	require.Equal(t, inferenceSpecializationHeader, hdr.Name)
	require.Equal(t, "travel-planner", hdr.GetStringMatch().GetExact())
	require.Equal(t, "gemini-travel", capRoute.GetRoute().GetCluster())

	// Route metadata carries the model's routing facts for the processor to read via
	// the xds.route_metadata request attribute: the concrete model to stamp and the
	// wire adapter to transform/normalize with.
	consulAI := capRoute.Metadata.FilterMetadata[inferenceListenerMetadataNamespace]
	require.NotNil(t, consulAI)
	require.Equal(t, "gemini-1.5-pro", consulAI.Fields["model"].GetStringValue())
	require.Equal(t, "openai", consulAI.Fields["adapter"].GetStringValue())

	// No fallback configured → catch-all fails closed with a 503.
	require.NotNil(t, routes[2].GetDirectResponse())
	require.Equal(t, uint32(503), routes[2].GetDirectResponse().GetStatus())
}
