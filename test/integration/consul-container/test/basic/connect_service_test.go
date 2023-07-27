// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package basic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// TestBasicConnectService Summary
// This test makes sure two services in the same datacenter have connectivity.
// A simulated client (a direct HTTP call) talks to it's upstream proxy through the
//
// Steps:
//   - Create a single agent cluster.
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an example static-client sidecar, then register both the service and sidecar with Consul
//   - Make sure a call to the client sidecar local bind port returns a response from the upstream, static-server
func TestBasicConnectService(t *testing.T) {
	t.Parallel()

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			// TODO(rb): fix the test to not need the service/envoy stack to use :8500
			AllowHTTPAnyway: true,
		},
	})

	_, clientService := topology.CreateServices(t, cluster, "http")
	_, port := clientService.GetAddr()
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.GetEnvoyListenerTCPFilters(t, adminPort)

	libassert.AssertContainerState(t, clientService, "running")
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server", "")
}

func TestConnectGRPCService_WithInputConfig(t *testing.T) {
	serverHclConfig := `
datacenter = "dc2"
data_dir = "/non-existent/conssul-data-dir"
node_name = "server-1"

bind_addr = "0.0.0.0"
max_query_time = "800s"
	`

	clientHclConfig := `
datacenter = "dc2"
data_dir = "/non-existent/conssul-data-dir"
node_name = "client-1"

bind_addr = "0.0.0.0"
max_query_time = "900s"
	`

	cluster, _, _ := topology.NewClusterWithConfig(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			AllowHTTPAnyway:        true,
		},
	},
		serverHclConfig,
		clientHclConfig,
	)

	// Verify the provided server config is merged to agent config
	serverConfig := cluster.Agents[0].GetConfig()
	require.Contains(t, serverConfig.JSON, "\"max_query_time\":\"800s\"")

	clientConfig := cluster.Agents[1].GetConfig()
	require.Contains(t, clientConfig.JSON, "\"max_query_time\":\"900s\"")

	_, clientService := topology.CreateServices(t, cluster, "grpc")
	_, port := clientService.GetAddr()
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.GRPCPing(t, fmt.Sprintf("localhost:%d", port))

	// time.Sleep(9999 * time.Second)
}
