package consul_container

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	consul_cluster "github.com/hashicorp/consul/integration/consul-container/libs/consul-cluster"
	consulcontainer "github.com/hashicorp/consul/integration/consul-container/libs/consul-node"

	"github.com/hashicorp/consul/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/sdk/testutil/retry"

	"github.com/stretchr/testify/require"
)

var curImage = flag.String("uut-image", "consul:local", "docker image to be used as UUT (unit under test)")
var latestImage = flag.String("latest-image", "consul:latest", "docker image to be used as latest")

func TestBasic(t *testing.T) {
	consulNode, err := consulcontainer.NewNodeWitConfig(context.Background(), consulcontainer.Config{Image: *latestImage})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := consulNode.Terminate()
		require.NoError(t, err)
	})
	retry.Run(t, func(r *retry.R) {
		leader, err := consulNode.Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
	})
}

func TestLatestGAServersWithCurrentClients(t *testing.T) {
	numServers := 3
	Cluster, err := serversCluster(t, numServers, *latestImage)
	require.NoError(t, err)
	defer Terminate(t, Cluster)
	numClients := 2
	Clients := make([]*consulcontainer.ConsulNode, numClients)
	for i := 0; i < numClients; i++ {
		Clients[i], err = consulcontainer.NewNodeWitConfig(context.Background(),
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd:   []string{"agent", "-client=0.0.0.0"},
				Image: *curImage,
			})
		require.NoError(t, err)
	}
	err = Cluster.AddNodes(Clients)
	retry.Run(t, func(r *retry.R) {
		leader, err := Cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Cluster.Nodes[0].Client.Agent().Members(false)
		require.Len(r, members, 5)
	})
}

func TestCurrentServersWithLatestGAClients(t *testing.T) {
	numServers := 3
	Cluster, err := serversCluster(t, numServers, *curImage)
	require.NoError(t, err)
	defer Cluster.Terminate()
	numClients := 2
	Clients := make([]*consulcontainer.ConsulNode, numClients)
	for i := 0; i < numClients; i++ {
		Clients[i], err = consulcontainer.NewNodeWitConfig(context.Background(),
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd:   []string{"agent", "-client=0.0.0.0"},
				Image: *curImage,
			})
	}
	err = Cluster.AddNodes(Clients)
	retry.Run(t, func(r *retry.R) {
		leader, err := Cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Cluster.Nodes[0].Client.Agent().Members(false)
		require.Len(r, members, 5)
	})
}

func TestMixedServersMajorityLatest(t *testing.T) {
	Servers := make([]*consulcontainer.ConsulNode, 3)
	var err error
	Servers[0], err = consulcontainer.NewNodeWitConfig(context.Background(),
		consulcontainer.Config{
			ConsulConfig: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
			Cmd:   []string{"agent", "-client=0.0.0.0"},
			Image: *curImage,
		})

	require.NoError(t, err)
	for i := 1; i < 3; i++ {
		Servers[i], err = consulcontainer.NewNodeWitConfig(context.Background(),
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
				Cmd:   []string{"agent", "-client=0.0.0.0"},
				Image: *latestImage,
			})

		require.NoError(t, err)

	}
	for i := 1; i < 3; i++ {
		err = Servers[i].Client.Agent().Join(fmt.Sprintf("%s", Servers[0].IP), false)
		require.NoError(t, err)
	}
	retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 100 * time.Millisecond}, t, func(r *retry.R) {
		leader, err := Servers[0].Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Servers[0].Client.Agent().Members(false)
		require.Len(r, members, 3)
	})
}

func TestMixedServersMajorityCurrent(t *testing.T) {
	var configs []consulcontainer.Config
	configs = append(configs,
		consulcontainer.Config{
			ConsulConfig: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
			Cmd:   []string{"agent", "-client=0.0.0.0"},
			Image: *latestImage,
		})

	for i := 1; i < 3; i++ {
		configs = append(configs,
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
				Cmd:   []string{"agent", "-client=0.0.0.0"},
				Image: *curImage,
			})

	}

	cluster, err := consul_cluster.New(configs)
	require.NoError(t, err)
	defer Terminate(t, cluster)
	retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 100 * time.Millisecond}, t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := cluster.Nodes[0].Client.Agent().Members(false)
		require.Len(r, members, 3)
	})
}

func serversCluster(t *testing.T, numServers int, image string) (*consul_cluster.Cluster, error) {
	var err error
	var configs []consulcontainer.Config
	for i := 0; i < numServers; i++ {
		configs = append(configs, consulcontainer.Config{
			ConsulConfig: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
			Cmd:   []string{"agent", "-client=0.0.0.0"},
			Image: image,
		})
	}
	cluster, err := consul_cluster.New(configs)
	require.NoError(t, err)
	retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 100 * time.Millisecond}, t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := cluster.Nodes[0].Client.Agent().Members(false)
		require.Len(r, members, numServers)
	})
	return cluster, err
}

func Terminate(t *testing.T, cluster *consul_cluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
