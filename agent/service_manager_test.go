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
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}
	{
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "redis",
				Protocol: "tcp",
			},
		}
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}

	// Now register a service locally with no sidecar, it should be a no-op.
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
}

func TestServiceManager_RegisterSidecar(t *testing.T) {
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
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}
	{
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "web",
				Protocol: "http",
			},
		}
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}
	{
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "redis",
				Protocol: "tcp",
			},
		}
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}

	// Now register a sidecar proxy. Note we don't use SidecarService here because
	// that gets resolved earlier in config handling than the AddService call
	// here.
	svc := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-sidecar-proxy",
		Service: "web-sidecar-proxy",
		Port:    21000,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8000,
			Upstreams: structs.Upstreams{
				{
					DestinationName: "redis",
					LocalBindPort:   5000,
				},
			},
		},
	}
	require.NoError(a.AddService(svc, nil, false, "", ConfigSourceLocal))

	// Verify sidecar got global config loaded
	sidecarService := a.State.Service("web-sidecar-proxy")
	require.NotNil(sidecarService)
	require.Equal(&structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-sidecar-proxy",
		Service: "web-sidecar-proxy",
		Port:    21000,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8000,
			Config: map[string]interface{}{
				"foo":      int64(1),
				"protocol": "http",
			},
			Upstreams: structs.Upstreams{
				{
					DestinationName: "redis",
					LocalBindPort:   5000,
					Config: map[string]interface{}{
						"protocol": "tcp",
					},
				},
			},
		},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}, sidecarService)
}

func TestServiceManager_RegisterMeshGateway(t *testing.T) {
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
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}
	{
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "mesh-gateway",
				Protocol: "http",
			},
		}
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}

	// Now register a mesh-gateway.
	svc := &structs.NodeService{
		Kind:    structs.ServiceKindMeshGateway,
		ID:      "mesh-gateway",
		Service: "mesh-gateway",
		Port:    443,
	}

	require.NoError(a.AddService(svc, nil, false, "", ConfigSourceLocal))

	// Verify gateway got global config loaded
	gateway := a.State.Service("mesh-gateway")
	require.NotNil(gateway)
	require.Equal(&structs.NodeService{
		Kind:    structs.ServiceKindMeshGateway,
		ID:      "mesh-gateway",
		Service: "mesh-gateway",
		Port:    443,
		Proxy: structs.ConnectProxyConfig{
			Config: map[string]interface{}{
				"foo":      int64(1),
				"protocol": "http",
			},
		},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}, gateway)
}

func TestServiceManager_Disabled(t *testing.T) {
	require := require.New(t)

	a := NewTestAgent(t, t.Name(), "enable_central_service_config = false")
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
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}
	{
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "web",
				Protocol: "http",
			},
		}
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}
	{
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "redis",
				Protocol: "tcp",
			},
		}
		var out bool
		require.NoError(a.RPC("ConfigEntry.Apply", args, &out))
	}

	// Now register a sidecar proxy. Note we don't use SidecarService here because
	// that gets resolved earlier in config handling than the AddService call
	// here.
	svc := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-sidecar-proxy",
		Service: "web-sidecar-proxy",
		Port:    21000,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8000,
			Upstreams: structs.Upstreams{
				{
					DestinationName: "redis",
					LocalBindPort:   5000,
				},
			},
		},
	}
	require.NoError(a.AddService(svc, nil, false, "", ConfigSourceLocal))

	// Verify sidecar got global config loaded
	sidecarService := a.State.Service("web-sidecar-proxy")
	require.NotNil(sidecarService)
	require.Equal(&structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-sidecar-proxy",
		Service: "web-sidecar-proxy",
		Port:    21000,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			DestinationServiceID:   "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8000,
			// No config added
			Upstreams: structs.Upstreams{
				{
					DestinationName: "redis",
					LocalBindPort:   5000,
					// No config added
				},
			},
		},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}, sidecarService)
}
