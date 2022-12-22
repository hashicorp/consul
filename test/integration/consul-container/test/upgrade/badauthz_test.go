package upgrade

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/test/integration/consul-container/test/topology"
)

// Test service intention continues functioning after upgrade
// Steps:
// 1. Create a cluster with one server and one client
// 2. Register a static-server at server agent and a static-client at the client agent
// 3. Upgrade the cluster
// 4. Write the service-intention to allow the connection
func TestBadauthz_UpgradeToTarget_fromLatest(t *testing.T) {
	cluster := topology.BasicSingleClusterTopology(t, &libcluster.Options{
		Datacenter: "dc1",
		NumServer:  1,
		NumClient:  1,
		Version:    *libutils.LatestVersion,
	})
	defer func() {
		err := cluster.Terminate()
		require.NoErrorf(t, err, "termining cluster")
	}()

	// Register an static-server service
	clientNodes, err := cluster.Clients()
	require.NoError(t, err)
	require.True(t, len(clientNodes) > 0)
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(clientNodes[0])
	require.NoError(t, err)
	client := clientNodes[0].GetClient()
	libassert.CatalogServiceExists(t, client, "static-server")
	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy")

	// Register an static-client service
	serverNodes, err := cluster.Servers()
	require.NoError(t, err)
	require.True(t, len(serverNodes) > 0)
	staticClientSvcSidecar, err := libservice.CreateAndRegisterStaticClientSidecar(serverNodes[0], "", true)
	require.NoError(t, err)

	cluster.ConfigEntryWrite(`
	Kind = "service-intentions"
	Name = "static-server"
	Sources = [
  		{
    		Name   = "static-client"
    		Action = "deny"
  		}
	]
	`)
	_, port := staticClientSvcSidecar.GetAddr()
	libassert.HTTPServiceFailTcpConnection(t, "localhost", port)

	// Upgrade the cluster to targetVersion
	t.Logf("Upgrade to version %s", *libutils.TargetVersion)
	err = cluster.StandardUpgrade(t, context.Background(), *libutils.TargetVersion)
	require.NoError(t, err)

	// Verify intentions work after upgrade
	err = cluster.ConfigEntryWrite(`
	Kind = "service-intentions"
	Name = "static-server"
	Sources = [
			{
			Name   = "static-client"
			Action = "allow"
			}
	]
	`)
	require.NoError(t, err)

	_, port = staticClientSvcSidecar.GetAddr()
	libassert.HTTPServiceEchoes(t, "localhost", port)
}
