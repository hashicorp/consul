package troubleshoot

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

func TestTroubleshootProxy(t *testing.T) {
	t.Parallel()
	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	serverService, clientService := topology.CreateServices(t, cluster)

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
			certsValid := strings.Contains(output, "certificates are valid")
			listenersExist := strings.Contains(output, fmt.Sprintf("listener for upstream \"%s\" found", libservice.StaticServerServiceName))
			routesExist := strings.Contains(output, fmt.Sprintf("route for upstream \"%s\" found", libservice.StaticServerServiceName))
			healthyEndpoints := strings.Contains(output, "✓ healthy endpoints for cluster")
			return upstreamExists && certsValid && listenersExist && routesExist && healthyEndpoints
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

			certsValid := strings.Contains(output, "certificates are valid")
			listenersExist := strings.Contains(output, fmt.Sprintf("listener for upstream \"%s\" found", libservice.StaticServerServiceName))
			routesExist := strings.Contains(output, fmt.Sprintf("route for upstream \"%s\" found", libservice.StaticServerServiceName))
			endpointUnhealthy := strings.Contains(output, "no healthy endpoints for cluster")
			return certsValid && listenersExist && routesExist && endpointUnhealthy
		}, 60*time.Second, 10*time.Second)
	})
}
