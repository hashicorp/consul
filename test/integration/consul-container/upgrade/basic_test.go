package consul_container

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	consulcontainer "github.com/hashicorp/consul/integration/ca/libs/consul-node"

	"github.com/hashicorp/consul/integration/ca/libs/utils"
	"github.com/hashicorp/consul/sdk/testutil/retry"

	"github.com/stretchr/testify/require"
)

const currentImage = "consul:local"

var loc = flag.String("cur-image", "consul:local", "docker image to be used as current")

func TestBasic(t *testing.T) {
	consulNode, err := consulcontainer.NewNode()
	require.NoError(t, err)
	defer Terminate(t, []*consulcontainer.ConsulNode{consulNode})
	retry.Run(t, func(r *retry.R) {
		leader, err := consulNode.Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
	})
}

func TestLatestGAServersWithCurrentClients(t *testing.T) {
	numServers := 3
	Servers, err := serversCluster(t, numServers, "consul:latest")
	require.NoError(t, err)
	defer Terminate(t, Servers)
	numClients := 2
	Clients := make([]*consulcontainer.ConsulNode, numClients)
	for i := 0; i < numClients; i++ {

		Clients[i], err = consulcontainer.NewNodeWitConfig(context.Background(),
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd:   []string{"agent", "-client=0.0.0.0"},
				Image: currentImage,
			})

		require.NoError(t, err)
		err = Clients[i].Client.Agent().Join(fmt.Sprintf("%s", Servers[0].IP), false)
		require.NoError(t, err)
	}
	defer Terminate(t, Clients)
	retry.Run(t, func(r *retry.R) {
		leader, err := Servers[0].Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Servers[0].Client.Agent().Members(false)
		require.Len(r, members, 5)
	})
}

func TestCurrentServersWithLatestGAClients(t *testing.T) {
	numServers := 3
	Servers, err := serversCluster(t, numServers, currentImage)
	require.NoError(t, err)
	defer Terminate(t, Servers)
	numClients := 2
	Clients := make([]*consulcontainer.ConsulNode, numClients)
	for i := 0; i < numClients; i++ {
		Clients[i], err = consulcontainer.NewNodeWitConfig(context.Background(),
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd:   []string{"agent", "-client=0.0.0.0"},
				Image: currentImage,
			})

		require.NoError(t, err)
		err = Clients[i].Client.Agent().Join(fmt.Sprintf("%s", Servers[0].IP), false)
		require.NoError(t, err)
	}
	defer Terminate(t, Clients)
	retry.Run(t, func(r *retry.R) {
		leader, err := Servers[0].Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Servers[0].Client.Agent().Members(false)
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
			Image: currentImage,
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
				Image: "consul:latest",
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
	Servers := make([]*consulcontainer.ConsulNode, 3)
	var err error
	Servers[0], err = consulcontainer.NewNodeWitConfig(context.Background(),
		consulcontainer.Config{
			ConsulConfig: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
			Cmd:   []string{"agent", "-client=0.0.0.0"},
			Image: "consul:latest",
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
				Image: currentImage,
			})

		require.NoError(t, err)

	}
	defer Terminate(t, Servers)
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

func serversCluster(t *testing.T, numServers int, image string) ([]*consulcontainer.ConsulNode, error) {
	Servers := make([]*consulcontainer.ConsulNode, numServers)
	var err error
	for i := 0; i < numServers; i++ {
		Servers[i], err = consulcontainer.NewNodeWitConfig(context.Background(),
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
				Cmd:   []string{"agent", "-client=0.0.0.0"},
				Image: image,
			})

		require.NoError(t, err)

	}
	for i := 1; i < numServers; i++ {
		err = Servers[i].Client.Agent().Join(fmt.Sprintf("%s", Servers[0].IP), false)
		require.NoError(t, err)
	}
	retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 100 * time.Millisecond}, t, func(r *retry.R) {
		leader, err := Servers[0].Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Servers[0].Client.Agent().Members(false)
		require.Len(r, members, numServers)
	})
	return Servers, err
}

func Terminate(t *testing.T, nodes []*consulcontainer.ConsulNode) {
	for _, s := range nodes {
		err := s.Terminate()
		require.NoError(t, err)
	}
}
