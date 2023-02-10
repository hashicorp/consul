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

func TestTroubleshootUpstream_Success(t *testing.T) {

	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	_, clientService := observability.CreateServices(t, cluster)

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)
	_, port := clientSidecar.GetInternalAdminAddr()
	// wait for envoy
	time.Sleep(5 * time.Second)
	_, outputReader, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "upstreams", "-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", port)})
	buf, err := io.ReadAll(outputReader)
	require.NoError(t, err)
	require.Contains(t, string(buf), libservice.StaticServerServiceName)
}

//func createServices(t *testing.T, cluster *libcluster.Cluster) (libservice.Service, libservice.Service) {
//	node := cluster.Agents[0]
//	client := node.GetClient()
//
//	// Register service as HTTP
//	serviceDefault := &api.ServiceConfigEntry{
//		Kind:     api.ServiceDefaults,
//		Name:     libservice.StaticServerServiceName,
//		Protocol: "http",
//	}
//
//	ok, _, err := client.ConfigEntries().Set(serviceDefault, nil)
//	require.NoError(t, err, "error writing HTTP service-default")
//	require.True(t, ok, "did not write HTTP service-default")
//
//	// Create a service and proxy instance
//	serviceOpts := &libservice.ServiceOpts{
//		Name:     libservice.StaticServerServiceName,
//		ID:       "static-server",
//		HTTPPort: 8080,
//		GRPCPort: 8079,
//	}
//
//	// Create a service and proxy instance
//	_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
//	require.NoError(t, err)
//
//	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticServerServiceName))
//	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName)
//
//	// Create a client proxy instance with the server as an upstream
//	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
//	require.NoError(t, err)
//
//	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName))
//
//	return serverConnectProxy, clientConnectProxy
//}
