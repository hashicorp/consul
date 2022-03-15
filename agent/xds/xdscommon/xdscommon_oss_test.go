//go:build !consulent
// +build !consulent

package xdscommon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/api"
)

func TestMakePluginConfiguration_TerminatingGateway(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTerminatingGatewayWithServiceDefaultsMeta(t)

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
				Meta: map[string]string{"a": "b"},
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
			"api.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":   apiService,
			"cache.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": cacheService,
			"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":    dbService,
			"web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":   webService,
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
