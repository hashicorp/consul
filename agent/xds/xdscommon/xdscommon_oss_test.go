//go:build !consulent
// +build !consulent

package xdscommon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func TestGetExtensionConfigurations_TerminatingGateway(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTerminatingGatewayWithLambdaServiceAndServiceResolvers(t)

	webService := api.CompoundServiceName{
		Name:      "web",
		Namespace: "default",
		Partition: "default",
	}
	dbService := api.CompoundServiceName{
		Name:      "db",
		Namespace: "default",
		Partition: "default",
	}
	cacheService := api.CompoundServiceName{
		Name:      "cache",
		Namespace: "default",
		Partition: "default",
	}
	apiService := api.CompoundServiceName{
		Name:      "api",
		Namespace: "default",
		Partition: "default",
	}

	expected := map[api.CompoundServiceName][]ExtensionConfiguration{
		apiService:   {},
		cacheService: {},
		dbService:    {},
		webService: {
			{
				EnvoyExtension: api.EnvoyExtension{
					Name: structs.BuiltinAWSLambdaExtension,
					Arguments: map[string]interface{}{
						"ARN":                "lambda-arn",
						"PayloadPassthrough": true,
						"Region":             "us-east-1",
					},
				},
				ServiceName: webService,
				Upstreams: map[api.CompoundServiceName]UpstreamData{
					apiService: {
						SNI: map[string]struct{}{
							"api.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
						},
						EnvoyID:           "api",
						OutgoingProxyKind: "terminating-gateway",
					},
					cacheService: {
						SNI: map[string]struct{}{
							"cache.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
						},
						EnvoyID:           "cache",
						OutgoingProxyKind: "terminating-gateway",
					},
					dbService: {
						SNI: map[string]struct{}{
							"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
						},
						EnvoyID:           "db",
						OutgoingProxyKind: "terminating-gateway",
					},
					webService: {
						SNI: map[string]struct{}{
							"canary1.web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
							"canary2.web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
							"web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":         {},
						},
						EnvoyID:           "web",
						OutgoingProxyKind: "terminating-gateway",
					},
				},
				Kind: api.ServiceKindTerminatingGateway,
			},
		},
	}

	require.Equal(t, expected, GetExtensionConfigurations(snap))
}

func TestGetExtensionConfigurations_ConnectProxy(t *testing.T) {
	dbService := api.CompoundServiceName{
		Name:      "db",
		Partition: "default",
		Namespace: "default",
	}
	envoyExtensions := []structs.EnvoyExtension{
		{
			Name: structs.BuiltinAWSLambdaExtension,
			Arguments: map[string]interface{}{
				"ARN":                "lambda-arn",
				"PayloadPassthrough": true,
				"Region":             "us-east-1",
			},
		},
	}

	serviceDefaults := &structs.ServiceConfigEntry{
		Kind:            structs.ServiceDefaults,
		Name:            "db",
		Protocol:        "http",
		EnvoyExtensions: envoyExtensions,
	}

	snapConnect := proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil, serviceDefaults)
	snapTermGw := proxycfg.TestConfigSnapshotDiscoveryChain(t, "register-to-terminating-gateway", nil, nil, serviceDefaults)

	type testCase struct {
		snapshot *proxycfg.ConfigSnapshot
		expected map[api.CompoundServiceName][]ExtensionConfiguration
	}
	cases := map[string]testCase{
		"connect proxy upstream": {
			snapshot: snapConnect,
			expected: map[api.CompoundServiceName][]ExtensionConfiguration{
				dbService: {
					{
						EnvoyExtension: api.EnvoyExtension{
							Name: structs.BuiltinAWSLambdaExtension,
							Arguments: map[string]interface{}{
								"ARN":                "lambda-arn",
								"PayloadPassthrough": true,
								"Region":             "us-east-1",
							},
						},
						ServiceName: dbService,
						Upstreams: map[api.CompoundServiceName]UpstreamData{
							dbService: {
								SNI: map[string]struct{}{
									"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
								},
								EnvoyID:           "db",
								OutgoingProxyKind: "connect-proxy",
							},
						},
						Kind: api.ServiceKindConnectProxy,
					},
				},
			},
		},
		"terminating gateway upstream": {
			snapshot: snapTermGw,
			expected: map[api.CompoundServiceName][]ExtensionConfiguration{
				dbService: {
					{
						EnvoyExtension: api.EnvoyExtension{
							Name: structs.BuiltinAWSLambdaExtension,
							Arguments: map[string]interface{}{
								"ARN":                "lambda-arn",
								"PayloadPassthrough": true,
								"Region":             "us-east-1",
							},
						},
						ServiceName: dbService,
						Upstreams: map[api.CompoundServiceName]UpstreamData{
							dbService: {
								SNI: map[string]struct{}{
									"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
								},
								EnvoyID:           "db",
								OutgoingProxyKind: "terminating-gateway",
							},
						},
						Kind: api.ServiceKindConnectProxy,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, GetExtensionConfigurations(tc.snapshot))
		})
	}
}
