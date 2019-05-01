package agent

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestServiceManager_RegisterService(t *testing.T) {
	require := require.New(t)

	a := NewTestAgent(t, t.Name(), "enable_central_service_config = true")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a global proxy and service config
	{
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ProxyConfigEntry{
				Config: map[string]interface{}{
					"foo": 1,
				},
			},
		}
		var out struct{}
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}
	{
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "redis",
				Protocol: "tcp",
				Connect: structs.ConnectConfiguration{
					SidecarProxy: true,
				},
			},
		}
		var out struct{}
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}

	// Now register a service locally to make sure we get a sidecar proxy
	// according to the central config above.
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
	}
	require.NoError(a.AddService(svc, nil, false, "", ConfigSourceLocal))

	// Verify both the service and sidecar.
	redisService := a.State.Service("redis")
	require.NotNil(redisService)
	require.Equal(&structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}, redisService)
	sidecarService := a.State.Service("redis-sidecar-proxy")
	require.NotNil(sidecarService)
	require.Equal(&structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "redis-sidecar-proxy",
		Service: "redis-sidecar-proxy",
		Port:    21000,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "redis",
			DestinationServiceID:   "redis",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8000,
			Config: map[string]interface{}{
				"foo":      int64(1),
				"protocol": "tcp",
			},
		},
		LocallyRegisteredAsSidecar: true,
	}, sidecarService)
}

func TestServiceManager_Disabled(t *testing.T) {
	require := require.New(t)

	a := NewTestAgent(t, t.Name(), "enable_central_service_config = false")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register some global proxy config
	args := &structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Entry: &structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
	}
	out := false
	require.NoError(a.RPC("ConfigEntry.Apply", args, &out))

	// Now register a service locally and make sure the resulting State entry
	// has the global config in it.
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
	}
	require.NoError(a.AddService(svc, nil, false, "", ConfigSourceLocal))
	mergedService := a.State.Service("redis")
	require.NotNil(mergedService)
	// The proxy config map shouldn't be present; the agent should ignore global
	// defaults here.
	require.Equal(&structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}, mergedService)
}
