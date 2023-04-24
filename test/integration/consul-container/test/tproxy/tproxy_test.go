// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tproxy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// TestTProxyService
// This test makes sure two services in the same datacenter have connectivity
// with transparent proxy enabled.
//
// Steps:
//   - Create a single server cluster.
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an example static-client sidecar, then register both the service and sidecar with Consul
//   - Make sure a request from static-client to the virtual address (<svc>.virtual.consul) returns a
//     response from the upstream.
func TestTProxyService(t *testing.T) {
	t.Parallel()

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                2,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			// TODO(rb): fix the test to not need the service/envoy stack to use :8500
			AllowHTTPAnyway: true,
		},
	})

	clientService := createServices(t, cluster)
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.AssertContainerState(t, clientService, "running")
	assertHTTPRequestToVirtualAddress(t, clientService)
}

func assertHTTPRequestToVirtualAddress(t *testing.T, clientService libservice.Service) {
	timer := &retry.Timer{Timeout: 120 * time.Second, Wait: 500 * time.Millisecond}

	retry.RunWith(timer, t, func(r *retry.R) {
		// Test that we can make a request to the virtual ip to reach the upstream.
		//
		// This uses a workaround for DNS because I had trouble modifying
		// /etc/resolv.conf. There is a --dns option to docker run, but it
		// didn't seem to be exposed via testcontainers. I'm not sure if it would
		// do what I want. In any case, Docker sets up /etc/resolv.conf for certain
		// functionality so it seems better to leave DNS alone.
		//
		// But, that means DNS queries aren't redirected to Consul out of the box.
		// As a workaround, we `dig @localhost:53` which is iptables-redirected to
		// localhost:8600 where the Consul client responds with the virtual ip.
		//
		// In tproxy tests, Envoy is not configured with a unique listener for each
		// upstream. This means the usual approach for non-tproxy tests doesn't
		// work - where we send the request to a host address mapped in to Envoy's
		// upstream listener. Instead, we exec into the container and run curl.
		//
		// We must make this request with a non-envoy user. The envoy and consul
		// users are excluded from traffic redirection rules, so instead we
		// make the request as root.
		out, err := clientService.Exec(
			context.Background(),
			[]string{"sudo", "sh", "-c", `
			set -e
			VIRTUAL=$(dig @localhost +short static-server.virtual.consul)
			echo "Virtual IP: $VIRTUAL"
			curl -s "$VIRTUAL/debug?env=dump"
			`,
			},
		)
		t.Logf("making call to upstream\nerr = %v\nout = %s", err, out)
		require.NoError(r, err)
		require.Regexp(r, `Virtual IP: 240.0.0.\d+`, out)
		require.Contains(r, out, "FORTIO_NAME=static-server")
	})
}

func createServices(t *testing.T, cluster *libcluster.Cluster) libservice.Service {
	{
		node := cluster.Agents[1]
		client := node.GetClient()
		// Create a service and proxy instance
		serviceOpts := &libservice.ServiceOpts{
			Name:     libservice.StaticServerServiceName,
			ID:       "static-server",
			HTTPPort: 8080,
			GRPCPort: 8079,
			Connect: libservice.SidecarService{
				Proxy: libservice.ConnectProxy{
					Mode: "transparent",
				},
			},
		}

		// Create a service and proxy instance
		_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
		require.NoError(t, err)

		libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", nil)
		libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)
	}

	{
		node := cluster.Agents[2]
		client := node.GetClient()

		// Create a client proxy instance with the server as an upstream
		clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, true)
		require.NoError(t, err)

		libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)
		return clientConnectProxy
	}
}
