// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package gateways

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const externalServerName = libservice.StaticServerServiceName

func requestRetryTimer() *retry.Timer {
	return &retry.Timer{Timeout: 120 * time.Second, Wait: 500 * time.Millisecond}
}

// TestTerminatingGateway Summary
// This test makes sure an external service can be reached via and terminating gateway. External server
// refers to being outside of the consul service mesh but it runs under the same docker network.
//
// Steps:
//   - Create a cluster (1 server and 1 client).
//   - Create the external service static-server (a single container, no proxy).
//   - Register an external node and the external service on that node in Consul.
//   - Create a terminating gateway config entry that includes an entry for the "external" static-server.
//   - Create the terminating gateway and register it with Consul.
//   - Create a static-client proxy (no need for a service container).
//   - Verify that the static-client can communicate with the external static-server through the terminating gateway
func TestTerminatingGatewayBasic(t *testing.T) {
	t.Parallel()
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	t.Logf("creating consul cluster")
	cluster, _, client := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
		},
		ApplyDefaultProxySettings: true,
	})
	node := cluster.Clients()[0]

	// Creates an external server that is not part of consul (no proxy)
	t.Logf("creating external server: %s", externalServerName)
	externalServerPort := 8083
	externalServerGRPCPort := 8079
	externalServer, err := libservice.NewExampleService(context.Background(), externalServerName, externalServerPort, externalServerGRPCPort, node)
	require.NoError(t, err)
	deferClean.Add(func() {
		_ = externalServer.Terminate()
	})

	// Register the external service in the default namespace. Tell consul it is located on an 'external node' and
	// not part of the service mesh. Because of the way that containers are created, the terminating gateway can
	// make a call to the address `localhost:<externalServerPort` and reach the external service.
	registerExternalService(t, client, externalServerName, "", "", "localhost", externalServerPort)

	// Create the config entry for the external service that associates it with the terminating gateway
	createTerminatingGatewayConfigEntry(t, client, "", "", externalServerName)

	// Create the terminating gateway in the default namespace.
	gwCfg := libservice.GatewayConfig{
		Name: api.TerminatingGateway,
		Kind: "terminating",
	}
	gatewayService, err := libservice.NewGatewayService(context.Background(), gwCfg, node)
	require.NoError(t, err)
	libassert.AssertContainerState(t, gatewayService, "running")
	libassert.CatalogServiceExists(t, client, gwCfg.Name, nil)

	// Creates a static client that is part of the mesh
	t.Logf("creating static client proxy")
	staticClient, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false, nil)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)

	// Verify that the static client can reach the external service by going through the terminating gateway
	// The `static-server` upstream is listening on port 5000 by default.
	assertHTTPRequestToServiceAddress(t, staticClient, externalServerName, libcluster.ServiceUpstreamLocalBindPort, true)
}

// registerExternalService registers a service on an external node so that Consul knows
// that the service is not being managed by an agent.
func registerExternalService(t *testing.T, consulClient *api.Client, name, namespace, partition, address string, port int) {
	t.Helper()

	service := &api.AgentService{
		ID:      name,
		Service: name,
		Port:    port,
	}

	part := "default"
	if partition != "" {
		part = partition
	}

	if namespace != "" {
		service.Namespace = namespace

		t.Logf("creating the %s namespace in Consul", namespace)
		_, _, err := consulClient.Namespaces().Create(&api.Namespace{
			Name:      namespace,
			Partition: part,
		}, nil)
		require.NoError(t, err)
	}

	t.Logf("registering the external service")
	_, err := consulClient.Catalog().Register(&api.CatalogRegistration{
		Node:     "legacy_node",
		Address:  address,
		NodeMeta: map[string]string{"external-node": "true", "external-probe": "true"},
		Service:  service,
	}, nil)
	require.NoError(t, err)
}

// createTerminatingGatewayConfigEntry creates and sets a config entry that binds the
// services to the gateway.
func createTerminatingGatewayConfigEntry(t *testing.T, consulClient *api.Client, gwNamespace, serviceNamespace string, serviceNames ...string) {
	t.Helper()

	t.Logf("creating terminating gateway config entry")

	if serviceNamespace != "" {
		t.Logf("creating the %s namespace in Consul", serviceNamespace)
		_, _, err := consulClient.Namespaces().Create(&api.Namespace{
			Name: serviceNamespace,
		}, nil)
		require.NoError(t, err)
	}

	var gatewayServices []api.LinkedService
	for _, serviceName := range serviceNames {
		linkedService := api.LinkedService{Name: serviceName, Namespace: serviceNamespace}
		gatewayServices = append(gatewayServices, linkedService)
	}

	configEntry := &api.TerminatingGatewayConfigEntry{
		Kind:      api.TerminatingGateway,
		Name:      "terminating-gateway",
		Namespace: gwNamespace,
		Services:  gatewayServices,
	}

	created, _, err := consulClient.ConfigEntries().Set(configEntry, nil)
	require.NoError(t, err)
	require.True(t, created, "failed to create terminating gateway config entry")
}

// assertHTTPRequestToServiceAddress checks the result of a request from the
// given `client` container to the given `server` container. If expSuccess is
// true, this checks for a successful request and otherwise it checks for the
// error we expect when traffic is rejected by mTLS.
//
// This assumes the destination service is running Fortio. It makes the request
// to `<serverIP>:8080/debug?env=dump` and checks for `FORTIO_NAME=<expServiceName>`
// in the response.
func assertHTTPRequestToServiceAddress(t *testing.T, client *libservice.ConnectContainer, serviceName string, port int, expSuccess bool) {
	upstreamURL := fmt.Sprintf("http://localhost:%d/debug?env=dump", port)
	retry.RunWith(requestRetryTimer(), t, func(r *retry.R) {
		out, err := client.Exec(context.Background(), []string{"curl", "-s", upstreamURL})
		r.Logf("curl request to upstream service address: url=%s\nerr = %v\nout = %s", upstreamURL, err, out)

		if expSuccess {
			require.NoError(r, err)
			require.Contains(r, out, fmt.Sprintf("FORTIO_NAME=%s", serviceName))
			r.Logf("successfuly messaged %s", serviceName)
		} else {
			require.Error(r, err)
			require.Contains(r, err.Error(), "exit code 52")
		}
	})
}
