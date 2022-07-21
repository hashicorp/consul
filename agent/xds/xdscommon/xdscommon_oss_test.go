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

func TestMakePluginConfiguration_TerminatingGateway(t *testing.T) {
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

	expected := PluginConfiguration{
		Kind: api.ServiceKindTerminatingGateway,
		ServiceConfigs: map[api.CompoundServiceName]ServiceConfig{
			webService: {
				Kind: api.ServiceKindTerminatingGateway,
				Meta: map[string]string{
					"serverless.consul.hashicorp.com/v1alpha1/lambda/enabled":             "true",
					"serverless.consul.hashicorp.com/v1alpha1/lambda/arn":                 "lambda-arn",
					"serverless.consul.hashicorp.com/v1alpha1/lambda/payload-passthrough": "true",
					"serverless.consul.hashicorp.com/v1alpha1/lambda/region":              "us-east-1",
				},
			},
			apiService: {
				Kind: api.ServiceKindTerminatingGateway,
			},
			cacheService: {
				Kind: api.ServiceKindTerminatingGateway,
			},
			dbService: {
				Kind: api.ServiceKindTerminatingGateway,
			},
		},
		SNIToServiceName: map[string]api.CompoundServiceName{
			"api.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":         apiService,
			"cache.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":       cacheService,
			"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":          dbService,
			"web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":         webService,
			"canary1.web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": webService,
			"canary2.web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": webService,
		},
		EnvoyIDToServiceName: map[string]api.CompoundServiceName{
			"web":   webService,
			"db":    dbService,
			"cache": cacheService,
			"api":   apiService,
		},
	}

	require.Equal(t, expected, MakePluginConfiguration(snap))
}

func TestMakePluginConfiguration_ConnectProxy(t *testing.T) {
	dbService := api.CompoundServiceName{
		Name:      "db",
		Partition: "default",
		Namespace: "default",
	}
	lambdaMeta := map[string]string{
		"serverless.consul.hashicorp.com/v1alpha1/lambda/enabled":             "true",
		"serverless.consul.hashicorp.com/v1alpha1/lambda/arn":                 "lambda-arn",
		"serverless.consul.hashicorp.com/v1alpha1/lambda/payload-passthrough": "true",
		"serverless.consul.hashicorp.com/v1alpha1/lambda/region":              "us-east-1",
	}
	serviceDefaults := &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db",
		Protocol: "http",
		Meta:     lambdaMeta,
	}

	snap := proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil, serviceDefaults)
	expected := PluginConfiguration{
		Kind: api.ServiceKindConnectProxy,
		ServiceConfigs: map[api.CompoundServiceName]ServiceConfig{
			dbService: {
				Kind: api.ServiceKindConnectProxy,
				Meta: lambdaMeta,
			},
		},
		SNIToServiceName: map[string]api.CompoundServiceName{
			"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": dbService,
		},
		EnvoyIDToServiceName: map[string]api.CompoundServiceName{
			"db": dbService,
		},
	}

	require.Equal(t, expected, MakePluginConfiguration(snap))
}
