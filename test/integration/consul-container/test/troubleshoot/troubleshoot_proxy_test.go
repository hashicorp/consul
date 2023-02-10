package troubleshoot

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/test/observability"
)

func TestTroubleshootProxy_Success(t *testing.T) {
	t.Parallel()
	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	_, clientService := observability.CreateServices(t, cluster)

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)
	_, port := clientSidecar.GetInternalAdminAddr()

	require.Eventually(t, func() bool {
		output, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "proxy",
			"-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", port),
			"-upstream-envoy-id", libservice.StaticServerServiceName})
		require.NoError(t, err)
		certsValid := strings.Contains(output, "certificates are valid")
		listenersExist := strings.Contains(output, fmt.Sprintf("listener for upstream \"%s\" found", libservice.StaticServerServiceName))
		routesExist := strings.Contains(output, fmt.Sprintf("route for upstream \"%s\" found", libservice.StaticServerServiceName))
		healthyEndpoints := strings.Contains(output, "\nhealthy endpoints for cluster")
		return certsValid && listenersExist && routesExist && healthyEndpoints
	}, 60*time.Second, 10*time.Second)
}

func TestTroubleshootProxy_FailHealthCheck(t *testing.T) {
	t.Parallel()
	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	serverService, clientService := observability.CreateServices(t, cluster)

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)

	_, clientAdminPort := clientSidecar.GetInternalAdminAddr()

	require.Eventually(t, func() bool {
		output, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "proxy",
			"-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", clientAdminPort),
			"-upstream-envoy-id", libservice.StaticServerServiceName})
		require.NoError(t, err)
		return strings.Contains(output, "no healthy endpoints for cluster")
	}, 60*time.Second, 10*time.Second)

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
}
