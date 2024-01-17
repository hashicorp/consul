// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package envoyextensions

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// TestOTELAccessLogging Summary
// This verifies that the OpenTelemetry access logging Envoy extension works as expected.
// A simulated client (a direct HTTP call) talks to its upstream proxy through the mesh.
// The upstream (static-server) is configured with a `builtin/otel-access-logging` extension that
// sends Envoy access logs to an OpenTelemetry collector for incoming HTTP requests.
// The OpenTelemetry collector is deployed as a container named `otel-collector` on the local network,
// and configured to write Envoy access logs to its stdout log stream.
//
// Steps:
//   - Create a single agent cluster.
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an example static-client sidecar, then register both the service and sidecar with Consul
//   - Create an OpenTelemetry collector container on the local network, this doesn't need to be registered with Consul.
//   - Configure the static-server service with a `builtin/otel-access-logging` EnvoyExtension targeting the
//     otel-collector service.
//   - Make sure a call to the client sidecar local bind port results in Envoy access logs being sent to the
//     otel-collector.
func TestOTELAccessLogging(t *testing.T) {
	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
		},
	})

	launchInfo := createLocalOTELService(t, cluster)

	clientService := createServices(t, cluster)
	_, port := clientService.GetAddr()
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.GetEnvoyListenerTCPFilters(t, adminPort)

	libassert.AssertContainerState(t, clientService, "running")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server", "")

	// Apply the OpenTelemetry Access Logging Envoy extension to the static-server
	consul := cluster.APIClient(0)
	defaults := api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "static-server",
		Protocol: "http",
		EnvoyExtensions: []api.EnvoyExtension{{
			Name: "builtin/otel-access-logging",
			Arguments: map[string]any{
				"Config": map[string]any{
					"LogName": "otel-integration-test",
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "127.0.0.1:4317"},
					},
				},
			},
		}},
	}
	consul.ConfigEntries().Set(&defaults, nil)

	// Make requests from the static-client to the static-server and look for the access logs
	// to show up in the `otel-collector` container logs.
	retry.Run(t, func(r *retry.R) {
		doRequest(r, fmt.Sprintf("http://localhost:%d", port), http.StatusOK)

		reader, err := launchInfo.Container.Logs(context.Background())
		require.NoError(r, err)
		log, err := io.ReadAll(reader)
		require.NoError(r, err)
		require.Contains(r, string(log), `log_name: Str(otel-integration-test)`)
		require.Contains(r, string(log), `cluster_name: Str(static-server)`)
		require.Contains(r, string(log), `node_name: Str(static-server-sidecar-proxy)`)
	},
		retry.WithFullOutput(),
		retry.WithRetryer(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Second}),
	)
}

func createLocalOTELService(t *testing.T, cluster *libcluster.Cluster) *libcluster.LaunchInfo {
	node := cluster.Agents[0]

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image:      "otel/opentelemetry-collector@sha256:7df7482ca6f2f3523cd389ac62c2851a652c67ac3b25fc67cd9248966aa706c1",
		AutoRemove: true,
		Name:       "otel-collector",
		Env:        make(map[string]string),
		Cmd:        []string{"--config", "/testdata/otel/config.yaml"},
		Mounts: []testcontainers.ContainerMount{{
			Source: testcontainers.DockerBindMountSource{
				HostPath: fmt.Sprintf("%s/testdata", cwd),
			},
			Target:   "/testdata",
			ReadOnly: true,
		}},
	}

	ctx := context.Background()

	exposedPorts := []string{}
	li, err := libcluster.LaunchContainerOnNode(ctx, node, req, exposedPorts)
	if err != nil {
		t.Fatal(err)
	}
	return li
}
