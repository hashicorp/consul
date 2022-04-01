package consul_container

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil/retry"

	"github.com/hashicorp/consul/integration/ca/libs/utils"

	consulcontainer "github.com/hashicorp/consul/integration/ca/libs/consul-node"

	"github.com/stretchr/testify/require"
)

const (
	connectCAPolicyTemplate = `
path "/sys/mounts" {
  capabilities = [ "read" ]
}

path "/sys/mounts/connect_root" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "/sys/mounts/%s/connect_inter" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "/connect_root/*" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "/%s/connect_inter/*" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}
`
	caPolicy = `
path "pki/cert/ca" {
  capabilities = ["read"]
}`
)

func TestBasic(t *testing.T) {
	consulNode, err := consulcontainer.NewNode()
	require.NoError(t, err)
	defer consulNode.Terminate()
	retry.Run(t, func(r *retry.R) {
		leader, err := consulNode.Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
	})
}

func TestBasicWithConfig(t *testing.T) {
	numServers := 3
	Servers := make([]*consulcontainer.ConsulNode, numServers)
	var err error
	for i := 0; i < numServers; i++ {
		Servers[i], err = consulcontainer.NewNodeWitConfig(context.Background(),
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
				Cmd: []string{"agent", "-client=0.0.0.0"},
			})

		require.NoError(t, err)
		defer Servers[i].Terminate()
	}
	err = Servers[1].Client.Agent().Join(fmt.Sprintf("%s", Servers[0].IP), false)
	require.NoError(t, err)
	numClients := 2
	err = Servers[numClients].Client.Agent().Join(fmt.Sprintf("%s", Servers[0].IP), false)
	require.NoError(t, err)
	retry.Run(t, func(r *retry.R) {
		leader, err := Servers[0].Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Servers[0].Client.Agent().Members(false)
		require.Len(r, members, numServers)
	})
	Clients := make([]*consulcontainer.ConsulNode, numClients)
	for i := 0; i < numClients; i++ {
		Clients[i], err = consulcontainer.NewNodeWitConfig(context.Background(),
			consulcontainer.Config{
				ConsulConfig: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd: []string{"agent", "-client=0.0.0.0"},
			})

		require.NoError(t, err)
		defer Clients[i].Terminate()
		err = Clients[i].Client.Agent().Join(fmt.Sprintf("%s", Servers[0].IP), false)
		require.NoError(t, err)
	}
	retry.Run(t, func(r *retry.R) {
		leader, err := Servers[0].Client.Status().Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := Servers[0].Client.Agent().Members(false)
		require.Len(r, members, 5)
	})
}
