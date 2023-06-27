package catalog

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog/catalogtest"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

func TestCatalog(t *testing.T) {
	t.Parallel()

	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 3,
		BuildOpts:  &libcluster.BuildOptions{Datacenter: "dc1"},
		Cmd:        `-hcl=experiments=["resource-apis"]`,
	})

	followers, err := cluster.Followers()
	require.NoError(t, err)
	client := pbresource.NewResourceServiceClient(followers[0].GetGRPCConn())

	t.Run("one-shot", func(t *testing.T) {
		catalogtest.RunCatalogV1Alpha1IntegrationTest(t, client)
	})

	t.Run("lifecycle", func(t *testing.T) {
		catalogtest.RunCatalogV1Alpha1LifecycleIntegrationTest(t, client)
	})
}
