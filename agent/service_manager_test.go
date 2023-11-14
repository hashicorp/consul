// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestServiceManager_RegisterService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a global proxy and service config
	testApplyConfigEntries(t, a,
		&structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "redis",
			Protocol: "tcp",
		},
	)

	// Now register a service locally with no sidecar, it should be a no-op.
	svc := &structs.NodeService{
		ID:             "redis",
		Service:        "redis",
		Port:           8000,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	require.NoError(t, a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal))

	// Verify both the service and sidecar.
	redisService := a.State.Service(structs.NewServiceID("redis", nil))
	require.NotNil(t, redisService)
	require.Equal(t, &structs.NodeService{
		ID:              "redis",
		Service:         "redis",
		Port:            8000,
		TaggedAddresses: map[string]structs.ServiceAddress{},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}, redisService)
}

func TestServiceManager_RegisterSidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a global proxy and service config
	testApplyConfigEntries(t, a,
		&structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "redis",
			Protocol: "tcp",
		},
	)

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
					DestinationName:      "redis",
					DestinationNamespace: "default",
					DestinationPartition: "default",
					LocalBindPort:        5000,
				},
			},
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	require.NoError(t, a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal))

	// Verify sidecar got global config loaded
	sidecarService := a.State.Service(structs.NewServiceID("web-sidecar-proxy", nil))
	require.NotNil(t, sidecarService)
	require.Equal(t, &structs.NodeService{
		Kind:            structs.ServiceKindConnectProxy,
		ID:              "web-sidecar-proxy",
		Service:         "web-sidecar-proxy",
		Port:            21000,
		TaggedAddresses: map[string]structs.ServiceAddress{},
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
					DestinationName:      "redis",
					DestinationNamespace: "default",
					DestinationPartition: "default",
					LocalBindPort:        5000,
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}, sidecarService)
}

func TestServiceManager_RegisterMeshGateway(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a global proxy and service config
	testApplyConfigEntries(t, a,
		&structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "mesh-gateway",
			Protocol: "http",
		},
	)

	// Now register a mesh-gateway.
	svc := &structs.NodeService{
		Kind:           structs.ServiceKindMeshGateway,
		ID:             "mesh-gateway",
		Service:        "mesh-gateway",
		Port:           443,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	require.NoError(t, a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal))

	// Verify gateway got global config loaded
	gateway := a.State.Service(structs.NewServiceID("mesh-gateway", nil))
	require.NotNil(t, gateway)
	require.Equal(t, &structs.NodeService{
		Kind:            structs.ServiceKindMeshGateway,
		ID:              "mesh-gateway",
		Service:         "mesh-gateway",
		Port:            443,
		TaggedAddresses: map[string]structs.ServiceAddress{},
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}, gateway)
}

func TestServiceManager_RegisterTerminatingGateway(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a global proxy and service config
	testApplyConfigEntries(t, a,
		&structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "terminating-gateway",
			Protocol: "http",
		},
	)

	// Now register a terminating-gateway.
	svc := &structs.NodeService{
		Kind:           structs.ServiceKindTerminatingGateway,
		ID:             "terminating-gateway",
		Service:        "terminating-gateway",
		Port:           443,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	require.NoError(t, a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal))

	// Verify gateway got global config loaded
	gateway := a.State.Service(structs.NewServiceID("terminating-gateway", nil))
	require.NotNil(t, gateway)
	require.Equal(t, &structs.NodeService{
		Kind:            structs.ServiceKindTerminatingGateway,
		ID:              "terminating-gateway",
		Service:         "terminating-gateway",
		Port:            443,
		TaggedAddresses: map[string]structs.ServiceAddress{},
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}, gateway)
}

func TestServiceManager_PersistService_API(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This is the ServiceManager version of TestAgent_PersistService  and
	// TestAgent_PurgeService.
	t.Parallel()

	// Launch a server to manage the config entries.
	serverAgent := NewTestAgent(t, "")
	defer serverAgent.Shutdown()
	testrpc.WaitForLeader(t, serverAgent.RPC, "dc1")

	// Register a global proxy and service config
	testApplyConfigEntries(t, serverAgent,
		&structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "redis",
			Protocol: "tcp",
		},
	)

	// Now launch a single client agent
	cfg := `
		server = false
		bootstrap = false
	`
	a := StartTestAgent(t, TestAgent{HCL: cfg})
	defer a.Shutdown()

	// Join first
	_, err := a.JoinLAN([]string{
		fmt.Sprintf("127.0.0.1:%d", serverAgent.Config.SerfPortLAN),
	}, nil)
	require.NoError(t, err)

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	newNodeService := func() *structs.NodeService {
		return &structs.NodeService{
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
						DestinationName:      "redis",
						DestinationNamespace: "default",
						DestinationPartition: "default",
						LocalBindPort:        5000,
					},
				},
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		}
	}

	expectState := &structs.NodeService{
		Kind:            structs.ServiceKindConnectProxy,
		ID:              "web-sidecar-proxy",
		Service:         "web-sidecar-proxy",
		Port:            21000,
		TaggedAddresses: map[string]structs.ServiceAddress{},
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
					DestinationName:      "redis",
					DestinationNamespace: "default",
					DestinationPartition: "default",
					LocalBindPort:        5000,
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	svc := newNodeService()
	svcID := svc.CompoundServiceID()

	svcFile := filepath.Join(a.Config.DataDir, servicesDir, svcID.StringHashSHA256())
	configFile := filepath.Join(a.Config.DataDir, serviceConfigDir, svcID.StringHashSHA256())

	// Service is not persisted unless requested, but we always persist service configs.
	err = a.AddService(AddServiceRequest{Service: svc, Source: ConfigSourceRemote})
	require.NoError(t, err)
	requireFileIsAbsent(t, svcFile)
	requireFileIsPresent(t, configFile)

	// Persists to file if requested
	err = a.AddService(AddServiceRequest{
		Service: svc,
		persist: true,
		token:   "mytoken",
		Source:  ConfigSourceRemote,
	})
	require.NoError(t, err)
	requireFileIsPresent(t, svcFile)
	requireFileIsPresent(t, configFile)

	// Service definition file is reasonable.
	expectJSONFile(t, svcFile, persistedService{
		Token:   "mytoken",
		Service: svc,
		Source:  "remote",
	}, nil)

	// Service config file is reasonable.
	pcfg := persistedServiceConfig{
		ServiceID: "web-sidecar-proxy",
		Defaults: &structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"foo":      1,
				"protocol": "http",
			},
			UpstreamConfigs: structs.OpaqueUpstreamConfigs{
				{
					Upstream: structs.PeeredServiceName{
						ServiceName: structs.NewServiceName("redis", nil),
					},
					Config: map[string]interface{}{
						"protocol": "tcp",
					},
				},
			},
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	expectJSONFile(t, configFile, pcfg, fixPersistedServiceConfigForTest)

	// Verify in memory state.
	{
		sidecarService := a.State.Service(structs.NewServiceID("web-sidecar-proxy", nil))
		require.NotNil(t, sidecarService)
		require.Equal(t, expectState, sidecarService)
	}

	// Updates service definition on disk
	svc = newNodeService()
	svc.Proxy.LocalServicePort = 8001
	err = a.AddService(AddServiceRequest{
		Service: svc,
		persist: true,
		token:   "mytoken",
		Source:  ConfigSourceRemote,
	})
	require.NoError(t, err)
	requireFileIsPresent(t, svcFile)
	requireFileIsPresent(t, configFile)

	// Service definition file is updated.
	expectJSONFile(t, svcFile, persistedService{
		Token:   "mytoken",
		Service: svc,
		Source:  "remote",
	}, nil)

	// Service config file is the same.
	pcfg = persistedServiceConfig{
		ServiceID: "web-sidecar-proxy",
		Defaults: &structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"foo":      1,
				"protocol": "http",
			},
			UpstreamConfigs: structs.OpaqueUpstreamConfigs{
				{
					Upstream: structs.PeeredServiceName{
						ServiceName: structs.NewServiceName("redis", nil),
					},
					Config: map[string]interface{}{
						"protocol": "tcp",
					},
				},
			},
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	expectJSONFile(t, configFile, pcfg, fixPersistedServiceConfigForTest)

	// Verify in memory state.
	expectState.Proxy.LocalServicePort = 8001
	{
		sidecarService := a.State.Service(structs.NewServiceID("web-sidecar-proxy", nil))
		require.NotNil(t, sidecarService)
		require.Equal(t, expectState, sidecarService)
	}

	// Kill the agent to restart it.
	a.Shutdown()

	// Kill the server so that it can't phone home and must rely upon the persisted defaults.
	serverAgent.Shutdown()

	// Should load it back during later start.
	a2 := StartTestAgent(t, TestAgent{HCL: cfg, DataDir: a.DataDir})
	defer a2.Shutdown()

	{
		restored := a.State.Service(structs.NewServiceID("web-sidecar-proxy", nil))
		require.NotNil(t, restored)
		require.Equal(t, expectState, restored)
	}

	// Now remove it.
	require.NoError(t, a2.RemoveService(structs.NewServiceID("web-sidecar-proxy", nil)))
	requireFileIsAbsent(t, svcFile)
	requireFileIsAbsent(t, configFile)
}

func TestServiceManager_PersistService_ConfigFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This is the ServiceManager version of TestAgent_PersistService  and
	// TestAgent_PurgeService but for config files.
	t.Parallel()

	// Launch a server to manage the config entries.
	serverAgent := NewTestAgent(t, "")
	defer serverAgent.Shutdown()
	testrpc.WaitForLeader(t, serverAgent.RPC, "dc1")

	// Register a global proxy and service config
	testApplyConfigEntries(t, serverAgent,
		&structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "redis",
			Protocol: "tcp",
		},
	)

	// Now launch a single client agent
	serviceSnippet := `
		service = {
		  kind  = "connect-proxy"
		  id    = "web-sidecar-proxy"
		  name  = "web-sidecar-proxy"
		  port  = 21000
		  token = "mytoken"
		  proxy {
			destination_service_name = "web"
			destination_service_id   = "web"
			local_service_address    = "127.0.0.1"
			local_service_port       = 8000
			upstreams = [{
			  destination_name = "redis"
			  destination_namespace = "default"
              destination_partition = "default"
			  local_bind_port  = 5000
			}]
		  }
		}
	`

	cfg := `
		server = false
		bootstrap = false
	` + serviceSnippet

	a := StartTestAgent(t, TestAgent{HCL: cfg})
	defer a.Shutdown()

	// Join first
	_, err := a.JoinLAN([]string{
		fmt.Sprintf("127.0.0.1:%d", serverAgent.Config.SerfPortLAN),
	}, nil)
	require.NoError(t, err)

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Now register a sidecar proxy via the API.
	svcID := "web-sidecar-proxy"

	expectState := &structs.NodeService{
		Kind:            structs.ServiceKindConnectProxy,
		ID:              "web-sidecar-proxy",
		Service:         "web-sidecar-proxy",
		Port:            21000,
		TaggedAddresses: map[string]structs.ServiceAddress{},
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
					DestinationType:      "service",
					DestinationName:      "redis",
					DestinationNamespace: "default",
					DestinationPartition: "default",
					LocalBindPort:        5000,
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// Now wait until we've re-registered using central config updated data.
	retry.Run(t, func(r *retry.R) {
		a.stateLock.Lock()
		defer a.stateLock.Unlock()
		current := a.State.Service(structs.NewServiceID("web-sidecar-proxy", nil))
		if current == nil {
			r.Fatalf("service is missing")
		}
		require.Equal(r, expectState, current)
	})

	svcFile := filepath.Join(a.Config.DataDir, servicesDir, stringHashSHA256(svcID))
	configFile := filepath.Join(a.Config.DataDir, serviceConfigDir, stringHashSHA256(svcID))

	// Service is never persisted, but we always persist service configs.
	requireFileIsAbsent(t, svcFile)
	requireFileIsPresent(t, configFile)

	// Service config file is reasonable.
	expectJSONFile(t, configFile, persistedServiceConfig{
		ServiceID: "web-sidecar-proxy",
		Defaults: &structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"foo":      1,
				"protocol": "http",
			},
			UpstreamConfigs: structs.OpaqueUpstreamConfigs{
				{
					Upstream: structs.PeeredServiceName{
						ServiceName: structs.NewServiceName("redis", nil),
					},
					Config: map[string]interface{}{
						"protocol": "tcp",
					},
				},
			},
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}, fixPersistedServiceConfigForTest)

	// Verify in memory state.
	{
		sidecarService := a.State.Service(structs.NewServiceID("web-sidecar-proxy", nil))
		require.NotNil(t, sidecarService)
		require.Equal(t, expectState, sidecarService)
	}

	// Kill the agent to restart it.
	a.Shutdown()

	// Kill the server so that it can't phone home and must rely upon the persisted defaults.
	serverAgent.Shutdown()

	// Should load it back during later start.
	a2 := StartTestAgent(t, TestAgent{HCL: cfg, DataDir: a.DataDir})
	defer a2.Shutdown()

	{
		restored := a.State.Service(structs.NewServiceID("web-sidecar-proxy", nil))
		require.NotNil(t, restored)
		require.Equal(t, expectState, restored)
	}

	// Now remove it.
	require.NoError(t, a2.RemoveService(structs.NewServiceID("web-sidecar-proxy", nil)))
	requireFileIsAbsent(t, svcFile)
	requireFileIsAbsent(t, configFile)
}

func TestServiceManager_Disabled(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "enable_central_service_config = false")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a global proxy and service config
	testApplyConfigEntries(t, a,
		&structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		},
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "redis",
			Protocol: "tcp",
		},
	)

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	require.NoError(t, a.addServiceFromSource(svc, nil, false, "", ConfigSourceLocal))

	// Verify sidecar got global config loaded
	sidecarService := a.State.Service(structs.NewServiceID("web-sidecar-proxy", nil))
	require.NotNil(t, sidecarService)
	require.Equal(t, &structs.NodeService{
		Kind:            structs.ServiceKindConnectProxy,
		ID:              "web-sidecar-proxy",
		Service:         "web-sidecar-proxy",
		Port:            21000,
		TaggedAddresses: map[string]structs.ServiceAddress{},
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}, sidecarService)
}

func testApplyConfigEntries(t *testing.T, a *TestAgent, entries ...structs.ConfigEntry) {
	t.Helper()
	for _, entry := range entries {
		args := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry:      entry,
		}
		var out bool
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", args, &out))
	}
}

func requireFileIsAbsent(t *testing.T, file string) {
	t.Helper()
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("should not persist")
	}
}

func requireFileIsPresent(t *testing.T, file string) {
	t.Helper()
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func expectJSONFile(t *testing.T, file string, expect interface{}, fixupContentBeforeCompareFn func([]byte) ([]byte, error)) {
	t.Helper()

	expected, err := json.Marshal(expect)
	require.NoError(t, err)

	content, err := os.ReadFile(file)
	require.NoError(t, err)

	if fixupContentBeforeCompareFn != nil {
		content, err = fixupContentBeforeCompareFn(content)
		require.NoError(t, err)
	}

	require.JSONEq(t, string(expected), string(content))
}

func fixPersistedServiceConfigForTest(content []byte) ([]byte, error) {
	var parsed persistedServiceConfig
	if err := json.Unmarshal(content, &parsed); err != nil {
		return nil, err
	}
	// Sort the output, since it's randomized and causes flaky tests otherwise.
	sort.Slice(parsed.Defaults.UpstreamConfigs, func(i, j int) bool {
		return parsed.Defaults.UpstreamConfigs[i].Upstream.String() < parsed.Defaults.UpstreamConfigs[j].Upstream.String()
	})
	out, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	// Clean the query meta
	return resetDefaultsQueryMeta(out)
}

// resetDefaultsQueryMeta will reset the embedded fields from structs.QueryMeta
// to their zero values in the json object keyed under 'Defaults'.
func resetDefaultsQueryMeta(content []byte) ([]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}
	def, ok := raw["Defaults"]
	if !ok {
		return content, nil
	}

	rawDef, ok := def.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected structure found in 'Defaults' key")
	}

	qmZero, err := convertToMap(structs.QueryMeta{})
	if err != nil {
		return nil, err
	}

	for k, v := range qmZero {
		rawDef[k] = v
	}

	raw["Defaults"] = rawDef

	return json.Marshal(raw)
}

func convertToMap(v interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}

	return raw, nil
}
