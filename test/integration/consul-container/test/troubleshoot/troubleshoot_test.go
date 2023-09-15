// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package troubleshoot

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

func TestTroubleshootProxy(t *testing.T) {
	t.Parallel()
	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
		ApplyDefaultProxySettings: true,
	})

	serverService, clientService := topology.CreateServices(t, cluster, "http")

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)
	_, clientAdminPort := clientSidecar.GetInternalAdminAddr()

	t.Run("upstream exists and is healthy", func(t *testing.T) {
		require.Eventually(t, func() bool {
			output, err := clientSidecar.Exec(context.Background(),
				[]string{"consul", "troubleshoot", "upstreams",
					"-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", clientAdminPort)})
			require.NoError(t, err)
			upstreamExists := assert.Contains(t, output, libservice.StaticServerServiceName)

			output, err = clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "proxy",
				"-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", clientAdminPort),
				"-upstream-envoy-id", libservice.StaticServerServiceName})
			require.NoError(t, err)
			certsValid := strings.Contains(output, "Certificates are valid")
			noRejectedConfig := strings.Contains(output, "Envoy has 0 rejected configurations")
			noConnFailure := strings.Contains(output, "Envoy has detected 0 connection failure(s)")
			listenersExist := strings.Contains(output, fmt.Sprintf("Listener for upstream \"%s\" found", libservice.StaticServerServiceName))
			healthyEndpoints := strings.Contains(output, "Healthy endpoints for cluster")
			return upstreamExists && certsValid && listenersExist && noRejectedConfig && noConnFailure && healthyEndpoints
		}, 60*time.Second, 10*time.Second)
	})

	t.Run("terminate upstream and check if client sees it as unhealthy", func(t *testing.T) {
		err := serverService.Terminate()
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			output, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "proxy",
				"-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", clientAdminPort),
				"-upstream-envoy-id", libservice.StaticServerServiceName})
			require.NoError(t, err)

			certsValid := strings.Contains(output, "Certificates are valid")
			noRejectedConfig := strings.Contains(output, "Envoy has 0 rejected configurations")
			noConnFailure := strings.Contains(output, "Envoy has detected 0 connection failure(s)")
			listenersExist := strings.Contains(output, fmt.Sprintf("Listener for upstream \"%s\" found", libservice.StaticServerServiceName))
			endpointUnhealthy := strings.Contains(output, "No healthy endpoints for cluster")
			return certsValid && listenersExist && noRejectedConfig && noConnFailure && endpointUnhealthy
		}, 60*time.Second, 10*time.Second)
	})
}
