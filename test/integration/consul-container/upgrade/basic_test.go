package consul_container

import (
	"context"
	"flag"
	"testing"
	"time"

	consulCluster "github.com/hashicorp/consul/integration/consul-container/libs/consul-cluster"
	consulNode "github.com/hashicorp/consul/integration/consul-container/libs/consul-node"

	"github.com/hashicorp/consul/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/sdk/testutil/retry"

	"github.com/stretchr/testify/require"
)

var curImage = flag.String("uut-version", "local", "docker image to be used as UUT (unit under test)")
var latestImage = flag.String("latest-version", "latest", "docker image to be used as latest")

const retryTimeout = 10 * time.Second
const retryFrequency = 500 * time.Millisecond

func TestBasic(t *testing.T) {
	t.Parallel()
	cluster, err := consulCluster.New([]consulNode.Config{consulNode.Config{Version: *latestImage}})

	require.NoError(t, err)

	defer Terminate(t, cluster)

	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := cluster.Nodes[0].GetClient().Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
	})
}

func TestLatestGAServersWithCurrentClients(t *testing.T) {
	t.Parallel()
	numServers := 3
	Cluster, err := serversCluster(t, numServers, *latestImage)
	require.NoError(t, err)
	defer Terminate(t, Cluster)
	numClients := 2
	Clients := make([]consulNode.Node, numClients)
	for i := 0; i < numClients; i++ {
		Clients[i], err = consulNode.NewConsulContainer(context.Background(),
			consulNode.Config{
				HCL: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: *curImage,
			})
		require.NoError(t, err)
	}
	err = Cluster.AddNodes(Clients)
	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := Cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Cluster.Nodes[0].GetClient().Agent().Members(false)
		require.Len(r, members, 5)
	})
}

func TestCurrentServersWithLatestGAClients(t *testing.T) {
	t.Parallel()
	numServers := 3
	Cluster, err := serversCluster(t, numServers, *curImage)
	require.NoError(t, err)
	defer Terminate(t, Cluster)
	numClients := 2
	Clients := make([]consulNode.Node, numClients)
	for i := 0; i < numClients; i++ {
		Clients[i], err = consulNode.NewConsulContainer(context.Background(),
			consulNode.Config{
				HCL: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: *curImage,
			})
	}
	err = Cluster.AddNodes(Clients)
	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := Cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Cluster.Nodes[0].GetClient().Agent().Members(false)
		require.Len(r, members, 5)
	})
}

func TestMixedServersMajorityLatest(t *testing.T) {
	t.Parallel()
	var configs []consulNode.Config
	configs = append(configs,
		consulNode.Config{
			HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: *curImage,
		})

	for i := 1; i < 3; i++ {
		configs = append(configs,
			consulNode.Config{
				HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: *latestImage,
			})

	}

	cluster, err := consulCluster.New(configs)
	require.NoError(t, err)
	defer Terminate(t, cluster)

	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := cluster.Nodes[0].GetClient().Agent().Members(false)
		require.Len(r, members, 3)
	})
}

func TestMixedServersMajorityCurrent(t *testing.T) {
	t.Parallel()
	var configs []consulNode.Config
	configs = append(configs,
		consulNode.Config{
			HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: *latestImage,
		})

	for i := 1; i < 3; i++ {
		configs = append(configs,
			consulNode.Config{
				HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: *curImage,
			})

	}

	cluster, err := consulCluster.New(configs)
	require.NoError(t, err)
	defer Terminate(t, cluster)

	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := cluster.Nodes[0].GetClient().Agent().Members(false)
		require.Len(r, members, 3)
	})
}

func serversCluster(t *testing.T, numServers int, image string) (*consulCluster.Cluster, error) {
	var err error
	var configs []consulNode.Config
	for i := 0; i < numServers; i++ {
		configs = append(configs, consulNode.Config{
			HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: image,
		})
	}
	cluster, err := consulCluster.New(configs)
	require.NoError(t, err)
	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := cluster.Nodes[0].GetClient().Agent().Members(false)
		require.Len(r, members, numServers)
	})
	return cluster, err
}

func Terminate(t *testing.T, cluster *consulCluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
