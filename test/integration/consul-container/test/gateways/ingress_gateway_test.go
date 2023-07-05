// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package gateways

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// TestIngressGateway Summary
// This test makes sure a cluster service can be reached via and ingress gateway.
//
// Steps:
//   - Create a cluster (1 server and 1 client).
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an ingress gateway and register it with Consul on the client agent
//   - Create a config entry that binds static-server to a new listener on the ingress gateway
//   - Verify that static-service is accessible through the ingress gateway port
func TestIngressGateway(t *testing.T) {
	t.Parallel()

	// Ingress gateways must have a listener other than 8443, which is used for health checks.
	// 9999 is already exposed from consul agents
	gatewayListenerPort := 9999

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
	apiClient := cluster.APIClient(0)
	clientNode := cluster.Clients()[0]

	// Set up the "static-server" backend
	serverService, _ := topology.CreateServices(t, cluster)

	// Create the ingress gateway service
	// We expose this on the client node, which already has port 9999 exposed as part of it's pause "pod"
	gwCfg := libservice.GatewayConfig{
		Name: api.IngressGateway,
		Kind: "ingress",
	}
	ingressService, err := libservice.NewGatewayService(context.Background(), gwCfg, clientNode)
	require.NoError(t, err)

	// this is deliberate
	// internally, ingress gw have a 15s timeout before the /ready endpoint is available,
	// then we need to wait for the health check to re-execute and propagate.
	time.Sleep(45 * time.Second)

	// We check this is healthy here because in the case of bringing up a new kube cluster,
	// it is not possible to create the config entry in advance.
	// The health checks must pass so the pod can start up.
	libassert.CatalogServiceIsHealthy(t, apiClient, api.IngressGateway, nil)

	// Register a service to the ingress gateway
	// **NOTE**: We intentionally wait until after the gateway starts to create the config entry.
	// This was a regression that can cause errors when starting up consul-k8s before you have the resource defined.
	ingressGwConfig := &api.IngressGatewayConfigEntry{
		Kind: api.IngressGateway,
		Name: api.IngressGateway,
		Listeners: []api.IngressListener{
			{
				Port:     gatewayListenerPort,
				Protocol: "http",
				Services: []api.IngressService{
					{
						Name: libservice.StaticServerServiceName,
					},
				},
			},
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(ingressGwConfig))

	// Wait for the request to persist
	checkIngressConfigEntry(t, apiClient, api.IngressGateway, nil)

	_, adminPort := ingressService.GetAdminAddr()
	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	//libassert.GetEnvoyListenerTCPFilters(t, adminPort) // This won't succeed because the dynamic listener is delayed

	libassert.AssertContainerState(t, ingressService, "running")
	libassert.AssertContainerState(t, serverService, "running")

	//time.Sleep(3600 * time.Second)
	mappedPort, err := clientNode.GetPod().MappedPort(context.Background(), nat.Port(fmt.Sprintf("%d/tcp", gatewayListenerPort)))
	require.NoError(t, err)

	// by default, ingress routes are set per <service>.ingress.*
	headers := map[string]string{"Host": fmt.Sprintf("%s.ingress.com", libservice.StaticServerServiceName)}
	libassert.HTTPServiceEchoesWithHeaders(t, "localhost", mappedPort.Int(), "", headers)
}

func checkIngressConfigEntry(t *testing.T, client *api.Client, gatewayName string, opts *api.QueryOptions) {
	t.Helper()

	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.IngressGateway, gatewayName, opts)
		if err != nil {
			t.Log("error constructing request", err)
			return false
		}
		if entry == nil {
			t.Log("returned entry is nil")
			return false
		}
		return true
	}, time.Second*10, time.Second*1)
}
