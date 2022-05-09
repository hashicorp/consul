package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/integration/consul-container/libs/node"

	"github.com/hashicorp/consul/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/sdk/testutil/retry"

	"github.com/stretchr/testify/require"
)

const retryTimeout = 20 * time.Second
const retryFrequency = 500 * time.Millisecond

// Test health check GRPC call using Target Servers and Latest GA Clients
func TestTargetServersWithLatestGAClients(t *testing.T) {
	numServers := 3
	cluster, err := serversCluster(t, numServers, *targetImage)
	require.NoError(t, err)
	defer Terminate(t, cluster)
	numClients := 1

	clients, err := clientsCreate(numClients, *latestImage)
	require.NoError(t, err)
	client := cluster.Nodes[0].GetClient()
	err = cluster.AddNodes(clients)
	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := client.Agent().Members(false)
		require.Len(r, members, 4)
	})
	serviceName := "api"
	err, index := serviceCreate(t, client, serviceName)

	ch := make(chan []*api.ServiceEntry)
	errCh := make(chan error)

	go func() {
		service, q, err := client.Health().Service(serviceName, "", false, &api.QueryOptions{WaitIndex: index})
		if q.QueryBackend != api.QueryBackendStreaming {
			err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
		}
		if err != nil {
			errCh <- err
		} else {
			ch <- service
		}
	}()
	err = client.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: serviceName, Port: 9998})
	timer := time.NewTimer(1 * time.Second)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case service := <-ch:
		require.Len(t, service, 1)
		require.Equal(t, serviceName, service[0].Service.Service)
		require.Equal(t, 9998, service[0].Service.Port)
	case <-timer.C:
		t.Fatalf("test timeout")
	}
}

// Test health check GRPC call using Mixed (majority latest) Servers and Latest GA Clients
func TestMixedServersMajorityLatestGAClient(t *testing.T) {
	var configs []node.Config
	configs = append(configs,
		node.Config{
			HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					server=true`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: *targetImage,
		})

	for i := 1; i < 3; i++ {
		configs = append(configs,
			node.Config{
				HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: *latestImage,
			})

	}

	cluster, err := cluster.New(configs)
	require.NoError(t, err)
	defer Terminate(t, cluster)

	numClients := 1
	clients, err := clientsCreate(numClients, *latestImage)
	client := clients[0].GetClient()
	err = cluster.AddNodes(clients)
	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := client.Agent().Members(false)
		require.Len(r, members, 4)
	})

	serviceName := "api"
	err, index := serviceCreate(t, client, serviceName)

	ch := make(chan []*api.ServiceEntry)
	errCh := make(chan error)
	go func() {
		service, q, err := client.Health().Service(serviceName, "", false, &api.QueryOptions{WaitIndex: index})
		if q.QueryBackend != api.QueryBackendStreaming {
			err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
		}
		if err != nil {
			errCh <- err
		} else {
			ch <- service
		}
	}()
	err = client.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: serviceName, Port: 9998})
	timer := time.NewTimer(1 * time.Second)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case service := <-ch:
		require.Len(t, service, 1)
		require.Equal(t, serviceName, service[0].Service.Service)
		require.Equal(t, 9998, service[0].Service.Port)
	case <-timer.C:
		t.Fatalf("test timeout")
	}
}

// Test health check GRPC call using Mixed (majority target) Servers and Latest GA Clients
func TestMixedServersMajorityTargetGAClient(t *testing.T) {
	var configs []node.Config
	for i := 0; i < 2; i++ {
		configs = append(configs,
			node.Config{
				HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: *targetImage,
			})

	}
	configs = append(configs,
		node.Config{
			HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					server=true`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: *latestImage,
		})

	cluster, err := cluster.New(configs)
	require.NoError(t, err)
	defer Terminate(t, cluster)

	numClients := 1
	clients, err := clientsCreate(numClients, *latestImage)
	client := clients[0].GetClient()
	err = cluster.AddNodes(clients)
	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := client.Agent().Members(false)
		require.Len(r, members, 4)
	})

	serviceName := "api"
	err, index := serviceCreate(t, client, serviceName)

	ch := make(chan []*api.ServiceEntry)
	errCh := make(chan error)
	go func() {
		service, q, err := client.Health().Service(serviceName, "", false, &api.QueryOptions{WaitIndex: index})
		if q.QueryBackend != api.QueryBackendStreaming {
			err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
		}
		if err != nil {
			errCh <- err
		} else {
			ch <- service
		}
	}()
	err = client.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: serviceName, Port: 9998})
	timer := time.NewTimer(1 * time.Second)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case service := <-ch:
		require.Len(t, service, 1)
		require.Equal(t, serviceName, service[0].Service.Service)
		require.Equal(t, 9998, service[0].Service.Port)
	case <-timer.C:
		t.Fatalf("test timeout")
	}
}

func clientsCreate(numClients int, version string) ([]node.Node, error) {
	clients := make([]node.Node, numClients)
	var err error
	for i := 0; i < numClients; i++ {
		clients[i], err = node.NewConsulContainer(context.Background(),
			node.Config{
				HCL: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: version,
			})
	}
	return clients, err
}

func serviceCreate(t *testing.T, client *api.Client, serviceName string) (error, uint64) {
	err := client.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: serviceName, Port: 9999})
	require.NoError(t, err)
	service, meta, err := client.Catalog().Service(serviceName, "", &api.QueryOptions{})
	require.NoError(t, err)
	require.Len(t, service, 1)
	require.Equal(t, serviceName, service[0].ServiceName)
	require.Equal(t, 9999, service[0].ServicePort)
	return err, meta.LastIndex
}

func serversCluster(t *testing.T, numServers int, version string) (*cluster.Cluster, error) {
	var err error
	var configs []node.Config
	for i := 0; i < numServers; i++ {
		configs = append(configs, node.Config{
			HCL: `node_name="` + utils.RandName("consul-server") + `"
					log_level="TRACE"
					bootstrap_expect=3
					server=true`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: version,
		})
	}
	cluster, err := cluster.New(configs)
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

func Terminate(t *testing.T, cluster *cluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
