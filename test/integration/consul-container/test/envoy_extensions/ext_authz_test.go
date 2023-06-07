// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package envoyextensions

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// TestExtAuthz Summary
// This test makes sure two services in the same datacenter have connectivity.
// A simulated client (a direct HTTP call) talks to it's upstream proxy through the mesh.
// The upstream (static-server) is configured with a `builtin/ext-authz` extension that
// calls an OPA external authorization service to authorize incoming HTTP requests.
//
// Steps:
//   - Create a single agent cluster.
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an example static-client sidecar, then register both the service and sidecar with Consul
//   - Create an OPA external authorization container on the local network, this doesn't need to be registered with Consul.
//   - Configure the static-server service with a `builtin/ext-authz` EnvoyExtension.
//   - Make sure a call to the client sidecar local bind port returns the expected response from the upstream, static-server:
//   - A call to `/allow` returns 200 OK.
//   - A call to any other endpoint returns 403 Forbidden.
func TestExtAuthz(t *testing.T) {
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

	// wire up ext-authz envoy extension for the static-server
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
	entry, _, _ := consul.ConfigEntries().Get("service-defaults", "static-server", nil)
	fmt.Printf("\n\n!!! entry = %#v\n\n", entry)
	baseURL := fmt.Sprintf("http://localhost:%d/", port)
	doRequest(t, baseURL, http.StatusForbidden)
	doRequest(t, baseURL+"/allow", http.StatusOK)
	wait()
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
			"/testdata/policies/bundle.tar.gz",
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
	var errs []string
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Log(err)
			t.FailNow()
		}
		req.Header.Set("user-agent", "foo")
		client := &http.Client{}
		res, err := client.Do(req)
		if err == nil {
			if res.StatusCode == expStatus {
				return
			} else {
				fmt.Printf("\n\n!!! response from %s: %#v\n\nrequest = %#v\n\n", url, *res, res.Request)
				errs = append(errs, fmt.Sprintf("%s unexpected status code: want: %d, have: %d", time.Now().Format(time.RFC3339), expStatus, res.StatusCode))
			}
		} else {
			errs = append(errs, fmt.Sprintf("unexpected error: %s", err.Error()))
		}
		if res != nil {
			res.Body.Close()
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	t.Logf("request failed:    \n%s", strings.Join(errs, "    \n"))
	t.Fail()
}

func wait() {
	for {
		_, err := os.Stat("continue")
		if err == nil {
			_ = os.Remove("continue")
			break
		}
		time.Sleep(time.Second)
	}
}
