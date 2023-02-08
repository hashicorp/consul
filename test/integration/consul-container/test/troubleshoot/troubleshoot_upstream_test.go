package troubleshoot

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTroubleshootUpstream(t *testing.T) {
	t.Parallel()

	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	_, clientService := createServices(t, cluster)

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)
	ip, port := clientSidecar.GetAdminAddr()
	_, outputReader, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "upstreams", "-envoy-admin-endpoint", fmt.Sprintf("%v:%v", ip, port)})
	var output []byte
	outputReader.Read(output)
	actual := string(output)
	require.NoError(t, err)
	require.Equal(t, "", actual)
}

func createServices(t *testing.T, cluster *libcluster.Cluster) (libservice.Service, libservice.Service) {
	node := cluster.Agents[0]
	client := node.GetClient()

	// Register service as HTTP
	serviceDefault := &api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     libservice.StaticServerServiceName,
		Protocol: "http",
	}

	ok, _, err := client.ConfigEntries().Set(serviceDefault, nil)
	require.NoError(t, err, "error writing HTTP service-default")
	require.True(t, ok, "did not write HTTP service-default")

	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name: libservice.StaticServerServiceName,
		ID:   "static-server",
	}

	// Create a service and proxy instance
	_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticServerServiceName))
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName))

	return serverConnectProxy, clientConnectProxy
}
