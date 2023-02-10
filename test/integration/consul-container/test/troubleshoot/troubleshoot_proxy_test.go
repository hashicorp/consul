package troubleshoot

import (
	"context"
	"fmt"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/test/observability"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
	"time"
)

func TestTroubleshootProxy_Success(t *testing.T) {

	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	_, clientService := observability.CreateServices(t, cluster)

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)
	_, port := clientSidecar.GetInternalAdminAddr()
	// wait for envoy
	time.Sleep(10 * time.Second)
	_, outputReader, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "proxy",
		"-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", port),
		"-upstream-envoy-id", libservice.StaticServerServiceName})
	require.NoError(t, err)
	buf, err := io.ReadAll(outputReader)
	require.NoError(t, err)
	require.Contains(t, string(buf), "certificates are valid")
	require.Contains(t, string(buf), fmt.Sprintf("listener for upstream \"%s\" found", libservice.StaticServerServiceName))
	require.Contains(t, string(buf), fmt.Sprintf("route for upstream \"%s\" found", libservice.StaticServerServiceName))
	require.Contains(t, string(buf), "\nhealthy endpoints for cluster")
}

func TestTroubleshootProxy_FailHealthCheck(t *testing.T) {

	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	serverService, clientService := observability.CreateServices(t, cluster)

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)

	_, clientAdminPort := clientSidecar.GetInternalAdminAddr()

	// wait for envoy
	time.Sleep(5 * time.Second)
	err := serverService.Terminate()
	require.NoError(t, err)

	time.Sleep(10 * time.Second)
	fmt.Println("waited 10 seconds")
	_, outputReader, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "proxy",
		"-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", clientAdminPort),
		"-upstream-envoy-id", libservice.StaticServerServiceName})
	require.NoError(t, err)
	buf, err := io.ReadAll(outputReader)
	require.NoError(t, err)
	require.Contains(t, string(buf), "certificates are valid")
	require.Contains(t, string(buf), fmt.Sprintf("listener for upstream \"%s\" found", libservice.StaticServerServiceName))
	require.Contains(t, string(buf), fmt.Sprintf("route for upstream \"%s\" found", libservice.StaticServerServiceName))
	require.Contains(t, string(buf), "no healthy endpoints for cluster")
}
