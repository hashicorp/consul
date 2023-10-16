// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package observability

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TestAccessLogs Summary
// This test ensures that when enabled through `proxy-defaults`, Envoy will emit access logs.
// Philosophically, we are trying to ensure the config options make their way to Envoy more than
// trying to test Envoy's behavior. For this reason and simplicity file operations are not tested.
//
// Steps:
//   - Create a single agent cluster.
//   - Enable default access logs. We do this so Envoy's admin interface inherits the configuration on startup
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an example static-client sidecar, then register both the service and sidecar with Consul
//   - Make sure a call to the client sidecar emits an access log at the client-sidecar (outbound) and
//     server-sidecar (inbound).
//   - Make sure hitting the Envoy admin interface generates an access log
//   - Change access log configuration to use custom text format and disable Listener logs
//   - Make sure a call to the client sidecar emits an access log at the client-sidecar (outbound) and
//     server-sidecar (inbound).
//
// Notes:
//   - Does not test disabling listener logs. In practice, it's hard to get them to emit. The best chance would
//     be running a service that throws a 404 on a random path or maybe use some path-based disco chains
//   - JSON keys have no guaranteed ordering, so simple key-value pairs are tested
//   - Because it takes a while for xDS updates to make it to envoy, it's not obvious when turning off access logs
//     will actually cause the proxies to update. Testing this proved difficult.
func TestAccessLogs(t *testing.T) {
	if semver.IsValid(utils.TargetVersion) && semver.Compare(utils.TargetVersion, "v1.15") < 0 {
		t.Skip()
	}

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	})

	// Turn on access logs. Do this before starting the sidecars so that they inherit the configuration
	// for their admin interface
	proxyDefault := &api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: api.ProxyConfigGlobal,
		AccessLogs: &api.AccessLogsConfig{
			Enabled:    true,
			JSONFormat: "{\"banana_path\":\"%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%\"}",
		},
	}
	set, _, err := cluster.Agents[0].GetClient().ConfigEntries().Set(proxyDefault, nil)
	require.NoError(t, err)
	require.True(t, set)

	serverService, clientService := topology.CreateServices(t, cluster, "http")
	_, port := clientService.GetAddr()

	// Validate Custom JSON
	require.Eventually(t, func() bool {
		libassert.HTTPServiceEchoes(t, "localhost", port, "banana")
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")
		client := libassert.ServiceLogContains(t, clientService, "\"banana_path\":\"/banana\"")
		server := libassert.ServiceLogContains(t, serverService, "\"banana_path\":\"/banana\"")
		return client && server
	}, 60*time.Second, 1*time.Second)

	// Validate Logs on the Admin Interface
	serverSidecar, ok := serverService.(*libservice.ConnectContainer)
	require.True(t, ok)
	ip, port := serverSidecar.GetAdminAddr()

	httpClient := cleanhttp.DefaultClient()
	url := fmt.Sprintf("http://%s:%d/clusters?fruit=bananas", ip, port)
	_, err = httpClient.Get(url)
	require.NoError(t, err, "error making call to Envoy admin interface")

	require.Eventually(t, func() bool {
		return libassert.ServiceLogContains(t, serverService, "\"banana_path\":\"/clusters?fruit=bananas\"")
	}, 15*time.Second, 1*time.Second)

	// TODO: add a test to check that connections without a matching filter chain are logged

	// Validate Listener Logs
	proxyDefault = &api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: api.ProxyConfigGlobal,
		AccessLogs: &api.AccessLogsConfig{
			Enabled:             true,
			DisableListenerLogs: true,
			TextFormat:          "Orange you glad I didn't say banana: %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%, %RESPONSE_FLAGS%",
		},
	}

	set, _, err = cluster.Agents[0].GetClient().ConfigEntries().Set(proxyDefault, nil)
	require.NoError(t, err)
	require.True(t, set)
	time.Sleep(5 * time.Second) // time for xDS to propagate

	// Validate Custom Text
	_, port = clientService.GetAddr()
	require.Eventually(t, func() bool {
		libassert.HTTPServiceEchoes(t, "localhost", port, "orange")
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")
		client := libassert.ServiceLogContains(t, clientService, "Orange you glad I didn't say banana: /orange, -")
		server := libassert.ServiceLogContains(t, serverService, "Orange you glad I didn't say banana: /orange, -")
		return client && server
	}, 60*time.Second, 500*time.Millisecond) // For some reason it takes a long time for the server sidecar to update

	// TODO: add a test to check that connections without a matching filter chain are NOT logged

}
