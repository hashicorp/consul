//go:build consulent
// +build consulent

package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Test partition crud using Current Clients and Latest GA Servers
func TestLatestGAServersWithCurrentClients_PartitionCRUD(t *testing.T) {
	testLatestGAServersWithCurrentClients_TenancyCRUD(t, "Partitions",
		func(t *testing.T, client *api.Client) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// CRUD partitions
			partition, _, err := client.Partitions().Read(ctx, "default", nil)
			require.NoError(t, err)
			fmt.Printf("%+v\n", partition)
			require.NotNil(t, partition)
			require.Equal(t, "default", partition.Name)

			fooPartReq := api.Partition{Name: "foo-part"}
			fooPart, _, err := client.Partitions().Create(ctx, &api.Partition{Name: "foo-part"}, nil)
			require.NoError(t, err)
			require.NotNil(t, fooPart)
			require.Equal(t, "foo-part", fooPart.Name)

			partition, _, err = client.Partitions().Read(ctx, "foo-part", nil)
			require.NoError(t, err)
			require.NotNil(t, partition)
			require.Equal(t, "foo-part", partition.Name)

			fooPartReq.Description = "foo-part part"
			partition, _, err = client.Partitions().Update(ctx, &fooPartReq, nil)
			require.NoError(t, err)
			require.NotNil(t, partition)
			require.Equal(t, "foo-part", partition.Name)
			require.Equal(t, "foo-part part", partition.Description)
		},
		func(t *testing.T, client *api.Client) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			//Read partition again
			retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
				partition, _, err := client.Partitions().Read(ctx, "default", nil)
				require.NoError(r, err)
				require.NotNil(r, partition)
				require.Equal(r, "default", partition.Name)
			})

			retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
				partition, _, err := client.Partitions().Read(ctx, "foo-part", nil)
				require.NoError(r, err)
				require.NotNil(r, partition)
				require.Equal(r, "foo-part", partition.Name)
				require.Equal(r, "foo-part part", partition.Description)
			})
		},
	)
}

// Test namespace crud using Current Clients and Latest GA Servers
func TestLatestGAServersWithCurrentClients_NamespaceCRUD(t *testing.T) {
	testLatestGAServersWithCurrentClients_TenancyCRUD(t, "Namespaces",
		func(t *testing.T, client *api.Client) {
			// CRUD namespaces
			namespace, _, err := client.Namespaces().Read("default", nil)
			require.NoError(t, err)
			require.NotNil(t, namespace, "default namespace does not exist yet")
			require.Equal(t, "default", namespace.Name)

			fooNsReq := api.Namespace{Name: "foo-ns"}
			fooNs, _, err := client.Namespaces().Create(&api.Namespace{Name: "foo-ns"}, nil)
			require.NoError(t, err)
			require.NotNil(t, fooNs)
			require.Equal(t, "foo-ns", fooNs.Name)

			namespace, _, err = client.Namespaces().Read("foo-ns", nil)
			require.NoError(t, err)
			require.NotNil(t, namespace)
			require.Equal(t, "foo-ns", namespace.Name)

			fooNsReq.Description = "foo-ns ns"
			namespace, _, err = client.Namespaces().Update(&fooNsReq, nil)
			require.NoError(t, err)
			require.NotNil(t, namespace)
			require.Equal(t, "foo-ns", namespace.Name)
			require.Equal(t, "foo-ns ns", namespace.Description)
		},
		func(t *testing.T, client *api.Client) {
			retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
				namespace, _, err := client.Namespaces().Read("default", nil)
				require.NoError(r, err)
				require.NotNil(r, namespace)
				require.Equal(r, "default", namespace.Name)
			})
			retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
				namespace, _, err := client.Namespaces().Read("foo-ns", nil)
				require.NoError(r, err)
				require.NotNil(r, namespace)
				require.Equal(r, "foo-ns", namespace.Name)
				require.Equal(r, "foo-ns ns", namespace.Description)
			})
		},
	)
}

func testLatestGAServersWithCurrentClients_TenancyCRUD(
	t *testing.T,
	tenancyName string,
	createFn func(t *testing.T, client *api.Client),
	readFn func(t *testing.T, client *api.Client),
) {
	const (
		numServers = 3
		numClients = 2
	)

	// Create initial cluster
	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: numServers,
		NumClients: numClients,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:      "dc1",
			ConsulImageName: utils.LatestImageName,
			ConsulVersion:   utils.LatestVersion,
		},
		ApplyDefaultProxySettings: true,
	})

	client := cluster.APIClient(0)
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 5)

	testutil.RunStep(t, "Create "+tenancyName, func(t *testing.T) {
		createFn(t, client)
	})

	ctx := context.Background()

	var snapshot io.ReadCloser
	testutil.RunStep(t, "Save snapshot", func(t *testing.T) {
		var err error
		snapshot, _, err = client.Snapshot().Save(nil)
		require.NoError(t, err)
	})

	testutil.RunStep(t, "Check "+tenancyName+" after upgrade", func(t *testing.T) {
		// Upgrade nodes
		leader, err := cluster.Leader()
		require.NoError(t, err)

		// upgrade things in the following order:
		//
		// 1. follower servers
		// 2. leader server
		// 3. clients
		var upgradeOrder []libcluster.Agent

		followers, err := cluster.Followers()
		require.NoError(t, err)
		upgradeOrder = append(upgradeOrder, followers...)
		upgradeOrder = append(upgradeOrder, leader)
		upgradeOrder = append(upgradeOrder, cluster.Clients()...)

		for _, n := range upgradeOrder {
			conf := n.GetConfig()

			// TODO: ensure this makes sense again, it was doing an apples/orange version!=image comparison
			if conf.Version == utils.TargetVersion {
				return
			}

			conf.Version = utils.TargetVersion

			if n.IsServer() {
				// You only ever need bootstrap settings the FIRST time, so we do not need
				// them again.
				conf.ConfigBuilder.Unset("bootstrap")
			} else {
				// If we upgrade the clients fast enough
				// membership might not be gossiped to all of
				// the clients to persist into their serf
				// snapshot, so force them to rejoin the
				// normal way on restart.
				conf.ConfigBuilder.Set("retry_join", []string{"agent-0"})
			}

			newJSON, err := json.MarshalIndent(conf.ConfigBuilder, "", "  ")
			require.NoError(t, err)
			conf.JSON = string(newJSON)
			t.Logf("Upgraded cluster config for %q:\n%s", n.GetName(), conf.JSON)

			selfBefore, err := n.GetClient().Agent().Self()
			require.NoError(t, err)

			require.NoError(t, n.Upgrade(ctx, conf))

			selfAfter, err := n.GetClient().Agent().Self()
			require.NoError(t, err)
			require.Truef(t,
				(selfBefore["Config"]["Version"] != selfAfter["Config"]["Version"]) || (selfBefore["Config"]["Revision"] != selfAfter["Config"]["Revision"]),
				fmt.Sprintf("upgraded version must be different (%s, %s), (%s, %s)", selfBefore["Config"]["Version"], selfBefore["Config"]["Revision"], selfAfter["Config"]["Version"], selfAfter["Config"]["Revision"]),
			)

			client := n.GetClient()

			libcluster.WaitForLeader(t, cluster, nil)
			libcluster.WaitForMembers(t, client, 5)
		}

		//get the client again as it changed after upgrade.
		client := cluster.APIClient(0)
		libcluster.WaitForLeader(t, cluster, client)

		// Read data again
		readFn(t, client)
	})

	// Terminate the cluster for the snapshot test
	testutil.RunStep(t, "Terminate the cluster", func(t *testing.T) {
		require.NoError(t, cluster.Terminate())
	})

	{ // Clear these so they super break if you tried to use them.
		cluster = nil
		client = nil
	}

	// Create a fresh cluster from scratch
	cluster2, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: numServers,
		NumClients: numClients,
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
		libcluster.WaitForMembers(t, client2, 5)

		// Restore the saved snapshot
		require.NoError(t, client2.Snapshot().Restore(nil, snapshot))

		libcluster.WaitForLeader(t, cluster2, client2)

		// make sure we still have the right data
		readFn(t, client2)
	})
}
