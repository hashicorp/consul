// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package wanfed

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
)

func TestPeering_WanFedSecondaryDC(t *testing.T) {
	t.Parallel()

	_, c1Agent := createCluster(t, "primary", func(c *libcluster.ConfigBuilder) {
		c.Set("primary_datacenter", "primary")
		// Enable ACLs, since they affect how the peering certificates are generated.
		c.Set("acl.enabled", true)
	})

	c2, c2Agent := createCluster(t, "secondary", func(c *libcluster.ConfigBuilder) {
		c.Set("primary_datacenter", "primary")
		c.Set("retry_join_wan", []string{c1Agent.GetIP()})
		// Enable ACLs, since they affect how the peering certificates are generated.
		c.Set("acl.enabled", true)
	})

	c3, c3Agent := createCluster(t, "alpha", nil)

	t.Run("secondary dc services are visible in primary dc", func(t *testing.T) {
		createConnectService(t, c2)
		libassert.CatalogServiceExists(t, c1Agent.GetClient(), libservice.StaticServerServiceName, &api.QueryOptions{Datacenter: "secondary"})
	})

	t.Run("secondary dc can peer to alpha dc", func(t *testing.T) {
		// Create the gateway
		gwCfg := libservice.GatewayConfig{
			Name: "mesh",
			Kind: "mesh",
		}
		_, err := libservice.NewGatewayService(context.Background(), gwCfg, c3.Servers()[0])
		require.NoError(t, err)

		// Create the peering connection
		require.NoError(t, c3.PeerWithCluster(c2Agent.GetClient(), "secondary-to-alpha", "alpha-to-secondary"))
		libassert.PeeringStatus(t, c2Agent.GetClient(), "secondary-to-alpha", api.PeeringStateActive)
	})

	t.Run("secondary dc can access services in alpha dc", func(t *testing.T) {
		service := createConnectService(t, c3)
		require.NoError(t, service.Export("default", "alpha-to-secondary", c3Agent.GetClient()))

		// Create a testing sidecar to proxy requests through
		clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(c2Agent, "secondary-to-alpha", false, false)
		require.NoError(t, err)
		libassert.CatalogServiceExists(t, c2Agent.GetClient(), "static-client-sidecar-proxy", nil)

		// Ensure envoy is configured for the peer service and healthy.
		_, adminPort := clientConnectProxy.GetAdminAddr()
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default.secondary-to-alpha.external", "HEALTHY", 1)
		libassert.AssertEnvoyMetricAtMost(t, adminPort, "cluster.static-server.default.secondary-to-alpha.external.", "upstream_cx_total", 0)

		// Make a call to the peered service multiple times.
		_, port := clientConnectProxy.GetAddr()
		for i := 0; i < 10; i++ {
			libassert.HTTPServiceEchoes(t, "localhost", port, "")
			libassert.AssertEnvoyMetricAtLeast(t, adminPort, "cluster.static-server.default.secondary-to-alpha.external.", "upstream_cx_total", i)
		}
	})
}

func createCluster(t *testing.T, dc string, f func(c *libcluster.ConfigBuilder)) (*libcluster.Cluster, libcluster.Agent) {
	ctx := libcluster.NewBuildContext(t, libcluster.BuildOptions{Datacenter: dc})
	conf := libcluster.NewConfigBuilder(ctx).Advanced(f)

	cluster, err := libcluster.New(t, []libcluster.Config{*conf.ToAgentConfig(t)})
	require.NoError(t, err)

	client := cluster.Agents[0].GetClient()

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 1)

	agent, err := cluster.Leader()
	require.NoError(t, err)
	return cluster, agent
}

func createConnectService(t *testing.T, cluster *libcluster.Cluster) libservice.Service {
	node := cluster.Agents[0]
	client := node.GetClient()

	// Create a service and proxy instance
	opts := libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       libservice.StaticServerServiceName,
		HTTPPort: 8080,
		GRPCPort: 8079,
	}
	serverConnectProxy, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, &opts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)
	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", nil)

	return serverConnectProxy
}
