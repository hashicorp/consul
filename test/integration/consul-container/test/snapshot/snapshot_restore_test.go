package snapshot

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

func TestSnapshotRestore(t *testing.T) {

	cases := []libcluster.LogStore{libcluster.LogStore_WAL, libcluster.LogStore_BoltDB}

	for _, c := range cases {
		t.Run(fmt.Sprintf("test log store: %s", c), func(t *testing.T) {
			testSnapShotRestoreForLogStore(t, c)
		})
	}
}

func testSnapShotRestoreForLogStore(t *testing.T, logStore libcluster.LogStore) {

	const (
		numServers = 3
	)

	// Create initial cluster
	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: numServers,
		NumClients: 0,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:      "dc1",
			ConsulImageName: utils.TargetImageName,
			ConsulVersion:   utils.TargetVersion,
			LogStore:        logStore,
		},
		ApplyDefaultProxySettings: true,
	})

	client := cluster.APIClient(0)
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 3)

	for i := 0; i < 100; i++ {
		_, err := client.KV().Put(&api.KVPair{Key: fmt.Sprintf("key-%d", i), Value: []byte(fmt.Sprintf("value-%d", i))}, nil)
		require.NoError(t, err)
	}

	var snapshot io.ReadCloser
	var err error
	snapshot, _, err = client.Snapshot().Save(nil)
	require.NoError(t, err)

	err = cluster.Terminate()
	require.NoError(t, err)
	// Create a fresh cluster from scratch
	cluster2, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: numServers,
		NumClients: 0,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:      "dc1",
			ConsulImageName: utils.TargetImageName,
			ConsulVersion:   utils.TargetVersion,
			LogStore:        logStore,
		},
		ApplyDefaultProxySettings: true,
	})
	client2 := cluster2.APIClient(0)

	libcluster.WaitForLeader(t, cluster2, client2)
	libcluster.WaitForMembers(t, client2, 3)

	// Restore the saved snapshot
	require.NoError(t, client2.Snapshot().Restore(nil, snapshot))

	libcluster.WaitForLeader(t, cluster2, client2)

	followers, err := cluster2.Followers()
	require.NoError(t, err)
	require.Len(t, followers, 2)

	// use a follower api client and set `AllowStale` to true
	// to test the follower snapshot install code path as well.
	fc := followers[0].GetClient()

	// Follower might not have finished loading snapshot yet which means attempts
	// could return nil or "key not found" for a while.
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 10 * time.Second, Wait: 100 * time.Millisecond}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		kv, _, err := fc.KV().Get(fmt.Sprintf("key-%d", 1), &api.QueryOptions{AllowStale: true})
		require.NoError(t, err)
		require.NotNil(t, kv)
		require.Equal(t, kv.Key, fmt.Sprintf("key-%d", 1))
		require.Equal(t, kv.Value, []byte(fmt.Sprintf("value-%d", 1)))
	})

	// Now we have at least one non-nil key, the snapshot must be loaded so check
	// we can read all the rest of them too.
	for i := 2; i < 100; i++ {
		kv, _, err := fc.KV().Get(fmt.Sprintf("key-%d", i), &api.QueryOptions{AllowStale: true})
		require.NoError(t, err)
		require.NotNil(t, kv)
		require.Equal(t, kv.Key, fmt.Sprintf("key-%d", i))
		require.Equal(t, kv.Value, []byte(fmt.Sprintf("value-%d", i)))
	}
}
