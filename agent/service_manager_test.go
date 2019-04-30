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
	require.Equal(&structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{
				"foo": int64(1),
			},
		},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}, mergedService)
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
