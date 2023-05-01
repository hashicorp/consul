// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tproxy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

var requestRetryTimer = &retry.Timer{Timeout: 120 * time.Second, Wait: 500 * time.Millisecond}

// TestTProxyService makes sure two services in the same datacenter have connectivity
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

	cluster := createCluster(t, 2) // 2 client agent pods

	clientService := createServices(t, cluster)
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.AssertContainerState(t, clientService, "running")
	assertHTTPRequestToStaticServerVirtual(t, clientService)
}

// TestTProxyPermissiveMTLS makes sure that a service in permissive mTLS mode accepts
// non-mesh traffic on the upstream service's port.
//
// Steps:
//   - Create a single server cluster
//   - Create the static-server and static-client services in the mesh
//   - Create an additional non-mesh container without a mesh service
//   - In default/strict mTLS mode, make sure requests from static-client to
//     static-server's virtual address succeed, but requests from the non-mesh
//     service fail
//   - In permissive mTLS mode, make sure requests from static-client to
//     static-server's virtual addresss succeed, and requests from the non-mesh
//     service to the static-server's normal address/port succeed.
func TestTProxyPermissiveMTLS(t *testing.T) {
	t.Parallel()

	// Three client containers: static-client, static-server, and a non-mesh service.
	// To simulate requests from outside the mesh, we use another client agent container
	// without a mesh service, and run curl to simulate requests from outside the mesh.
	cluster := createCluster(t, 3)

	staticServerPod := cluster.Agents[1]
	nonMeshPod := cluster.Agents[3]

	clientService := createServices(t, cluster)
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.AssertContainerState(t, clientService, "running")

	// Validate mesh traffic to the virtual address succeeds in strict/default mTLS mode.
	assertHTTPRequestToStaticServerVirtual(t, clientService)
	// Validate non-mesh is blocked in strict/default mTLS mode.
	assertHTTPRequestToStaticServerServiceAddress(t, nonMeshPod, staticServerPod, false)

	// Put the service in permissive mTLS mode
	require.NoError(t, cluster.ConfigEntryWrite(&api.MeshConfigEntry{
		AllowEnablingPermissiveMutualTLS: true,
	}))
	require.NoError(t, cluster.ConfigEntryWrite(&api.ServiceConfigEntry{
		Kind:          api.ServiceDefaults,
		Name:          libservice.StaticServerServiceName,
		MutualTLSMode: api.MutualTLSModePermissive,
	}))

	// Validate mesh traffic to the virtual address succeeds in permissive mTLS mode.
	assertHTTPRequestToStaticServerVirtual(t, clientService)
	// Validate non-mesh traffic succeeds in permissive mode.
	assertHTTPRequestToStaticServerServiceAddress(t, nonMeshPod, staticServerPod, true)
}

// assertHTTPRequestToStaticServerVirtual checks that a request to the static-server's
// virtual address succeeds.
func assertHTTPRequestToStaticServerVirtual(t *testing.T, clientService libservice.Service) {
	retry.RunWith(requestRetryTimer, t, func(r *retry.R) {
		// Test that we can make a request to the virtual ip to reach the upstream.
		//
		// NOTE(pglass): This uses a workaround for DNS because I had trouble modifying
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
		t.Logf("curl request to upstream virtual address\nerr = %v\nout = %s", err, out)
		require.NoError(r, err)
		require.Regexp(r, `Virtual IP: 240.0.0.\d+`, out)
		require.Contains(r, out, "FORTIO_NAME=static-server")
	})
}

// assertHTTPRequestToStaticServerServiceAddress checks the result of a request
// from a non-mesh service to the static-server service. If expSuccess is true,
// we check for a successful request, and otherwise check for a failed request.
func assertHTTPRequestToStaticServerServiceAddress(t *testing.T, client, server libcluster.Agent, expSuccess bool) {
	upstreamURL := fmt.Sprintf("http://%s:8080/debug?env=dump", server.GetIP())
	retry.RunWith(requestRetryTimer, t, func(r *retry.R) {
		out, err := client.Exec(context.Background(), []string{"curl", "-s", upstreamURL})
		t.Logf("curl request to upstream service address: url=%s\nerr = %v\nout = %s", upstreamURL, err, out)

		if expSuccess {
			require.NoError(r, err)
			require.Contains(r, out, "FORTIO_NAME=static-server")
		} else {
			require.Error(r, err)
			require.Contains(r, err.Error(), "exit code 52")
		}
	})
}

func createCluster(t *testing.T, numClients int) *libcluster.Cluster {
	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                numClients,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			// TODO(rb): fix the test to not need the service/envoy stack to use :8500
			AllowHTTPAnyway: true,
		},
	})
	return cluster
}

// createServices creates the static-client and static-server services with
// transparent proxy enabled.
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
