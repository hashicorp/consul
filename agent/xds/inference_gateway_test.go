// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
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
