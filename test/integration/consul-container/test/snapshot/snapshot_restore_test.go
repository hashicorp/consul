package snapshot

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestSnapshotRestore(t *testing.T) {
	const (
		numServers = 3
	)

	// Create initial cluster
	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: numServers,
		NumClients: 0,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:      "dc1",
			ConsulImageName: utils.LatestImageName,
			ConsulVersion:   utils.LatestVersion,
		},
		ApplyDefaultProxySettings: true,
	})

	client := cluster.APIClient(0)
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 3)

	testutil.RunStep(t, "Create Data", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			_, err := client.KV().Put(&api.KVPair{Key: fmt.Sprintf("key-%d", i), Value: []byte(fmt.Sprintf("value-%d", i))}, nil)
			require.NoError(t, err)
		}
	})

	var snapshot io.ReadCloser
	testutil.RunStep(t, "Save snapshot", func(t *testing.T) {
		var err error
		snapshot, _, err = client.Snapshot().Save(nil)
		require.NoError(t, err)
	})

	err := cluster.Terminate()
	require.NoError(t, err)
	// Create a fresh cluster from scratch
	cluster2, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: numServers,
		NumClients: 0,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:      "dc1",
			ConsulImageName: utils.LatestImageName,
			ConsulVersion:   utils.LatestVersion,
		},
		ApplyDefaultProxySettings: true,
	})
	client2 := cluster2.APIClient(0)

	testutil.RunStep(t, "Restore saved snapshot", func(t *testing.T) {
		libcluster.WaitForLeader(t, cluster2, client2)
		libcluster.WaitForMembers(t, client2, 3)

		// Restore the saved snapshot
		require.NoError(t, client2.Snapshot().Restore(nil, snapshot))

		libcluster.WaitForLeader(t, cluster2, client2)

	})

	for i := 0; i < 100; i++ {
		kv, _, err := client2.KV().Get(fmt.Sprintf("key-%d", i), nil)
		require.NoError(t, err)
		require.Equal(t, kv.Key, fmt.Sprintf("key-%d", i))
		require.Equal(t, kv.Value, []byte(fmt.Sprintf("value-%d", i)))
	}
}
