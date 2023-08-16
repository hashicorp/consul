// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package envoyextensions

import (
	"context"
	"fmt"
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
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/go-cleanhttp"
)

// TestExtAuthzLocal Summary
// This test makes sure two services in the same datacenter have connectivity.
// A simulated client (a direct HTTP call) talks to it's upstream proxy through the mesh.
// The upstream (static-server) is configured with a `builtin/ext-authz` extension that
// calls an OPA external authorization service to authorize incoming HTTP requests.
// The external authorization service is deployed as a container on the local network.
//
// Steps:
// - Create a single agent cluster.
// - Create the example static-server and sidecar containers, then register them both with Consul
// - Create an example static-client sidecar, then register both the service and sidecar with Consul
// - Create an OPA external authorization container on the local network, this doesn't need to be registered with Consul.
// - Configure the static-server service with a `builtin/ext-authz` EnvoyExtension targeting the OPA ext-authz service.
// - Make sure a call to the client sidecar local bind port returns the expected response from the upstream static-server:
//   - A call to `/allow` returns 200 OK.
//   - A call to any other endpoint returns 403 Forbidden.
func TestExtAuthzLocal(t *testing.T) {
	t.Parallel()

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

	createLocalAuthzService(t, cluster)

	clientService := createServices(t, cluster)
	_, port := clientService.GetAddr()
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.GetEnvoyListenerTCPFilters(t, adminPort)

	libassert.AssertContainerState(t, clientService, "running")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server", "")

	// Wire up the ext-authz envoy extension for the static-server
	consul := cluster.APIClient(0)
	defaults := api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "static-server",
		Protocol: "http",
		EnvoyExtensions: []api.EnvoyExtension{{
			Name: "builtin/ext-authz",
			Arguments: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "127.0.0.1:9191"},
					},
				},
			},
		}},
	}
	consul.ConfigEntries().Set(&defaults, nil)

	// Make requests to the static-server. We expect that all requests are rejected with 403 Forbidden
	// unless they are to the /allow path.
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	doRequest(t, baseURL, http.StatusForbidden)
	doRequest(t, baseURL+"/allow", http.StatusOK)
}

func createServices(t *testing.T, cluster *libcluster.Cluster) libservice.Service {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}

	// Create a service and proxy instance
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", nil)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)

	return clientConnectProxy
}

func createLocalAuthzService(t *testing.T, cluster *libcluster.Cluster) {
	node := cluster.Agents[0]

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image:      "openpolicyagent/opa:0.53.0-envoy-3",
		AutoRemove: true,
		Name:       "ext-authz",
		Env:        make(map[string]string),
		Cmd: []string{
			"run",
			"--server",
			"--addr=localhost:8181",
			"--diagnostic-addr=0.0.0.0:8282",
			"--set=plugins.envoy_ext_authz_grpc.addr=:9191",
			"--set=plugins.envoy_ext_authz_grpc.path=envoy/authz/allow",
			"--set=decision_logs.console=true",
			"--set=status.console=true",
			"--ignore=.*",
			"/testdata/policies/policy.rego",
		},
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
	_, err = libcluster.LaunchContainerOnNode(ctx, node, req, exposedPorts)
	if err != nil {
		t.Fatal(err)
	}
}

func doRequest(t *testing.T, url string, expStatus int) {
	retry.RunWith(&retry.Timer{Timeout: 5 * time.Second, Wait: time.Second}, t, func(r *retry.R) {
		resp, err := cleanhttp.DefaultClient().Get(url)
		require.NoError(r, err)
		require.Equal(r, expStatus, resp.StatusCode)
	})
}
