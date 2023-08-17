// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
)

// CreateServices
func CreateServices(t *testing.T, cluster *libcluster.Cluster, protocol string) (libservice.Service, libservice.Service) {
	node := cluster.Agents[0]
	client := node.GetClient()

	// Register service as HTTP
	serviceDefault := &api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     libservice.StaticServerServiceName,
		Protocol: protocol,
	}

	ok, _, err := client.ConfigEntries().Set(serviceDefault, nil)
	require.NoError(t, err, "error writing HTTP service-default")
	require.True(t, ok, "did not write HTTP service-default")

	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}

	if protocol == "grpc" {
		serviceOpts.RegisterGRPC = true
	}

	// Create a service and proxy instance
	_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticServerServiceName), nil)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName), nil)

	return serverConnectProxy, clientConnectProxy
}
