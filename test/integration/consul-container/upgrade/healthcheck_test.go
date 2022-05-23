package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/integration/consul-container/libs/node"
	"github.com/hashicorp/consul/integration/consul-container/libs/utils"
)

// Test health check GRPC call using Target Servers and Latest GA Clients
func TestTargetServersWithLatestGAClients(t *testing.T) {
	const (
		numServers = 3
		numClients = 1
	)

	cluster := serversCluster(t, numServers, *targetImage)
	defer Terminate(t, cluster)

	clients := clientsCreate(t, numClients, *latestImage, cluster.EncryptKey)

	require.NoError(t, cluster.AddNodes(clients))

	client := cluster.Nodes[0].GetClient()

	waitForLeader(t, cluster, client)
	waitForMembers(t, client, 4)

	serviceName := "api"
	index := serviceCreate(t, client, serviceName)

	ch := make(chan []*api.ServiceEntry)
	errCh := make(chan error)

	go func() {
		service, q, err := client.Health().Service(serviceName, "", false, &api.QueryOptions{WaitIndex: index})
		if err == nil && q.QueryBackend != api.QueryBackendStreaming {
			err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
		}
		if err != nil {
			errCh <- err
		} else {
			ch <- service
		}
	}()

	require.NoError(t, client.Agent().ServiceRegister(
		&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
	))

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

	const (
		numClients = 1
	)

	clients := clientsCreate(t, numClients, *latestImage, cluster.EncryptKey)

	require.NoError(t, cluster.AddNodes(clients))

	client := clients[0].GetClient()

	waitForLeader(t, cluster, client)
	waitForMembers(t, client, 4)

	serviceName := "api"
	index := serviceCreate(t, client, serviceName)

	ch := make(chan []*api.ServiceEntry)
	errCh := make(chan error)
	go func() {
		service, q, err := client.Health().Service(serviceName, "", false, &api.QueryOptions{WaitIndex: index})
		if err == nil && q.QueryBackend != api.QueryBackendStreaming {
			err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
		}
		if err != nil {
			errCh <- err
		} else {
			ch <- service
		}
	}()

	require.NoError(t, client.Agent().ServiceRegister(
		&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
	))

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

	const (
		numClients = 1
	)

	clients := clientsCreate(t, numClients, *latestImage, cluster.EncryptKey)

	require.NoError(t, cluster.AddNodes(clients))

	client := clients[0].GetClient()

	waitForLeader(t, cluster, client)
	waitForMembers(t, client, 4)

	serviceName := "api"
	index := serviceCreate(t, client, serviceName)

	ch := make(chan []*api.ServiceEntry)
	errCh := make(chan error)
	go func() {
		service, q, err := client.Health().Service(serviceName, "", false, &api.QueryOptions{WaitIndex: index})
		if err == nil && q.QueryBackend != api.QueryBackendStreaming {
			err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
		}
		if err != nil {
			errCh <- err
		} else {
			ch <- service
		}
	}()

	require.NoError(t, client.Agent().ServiceRegister(
		&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
	))

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

func clientsCreate(t *testing.T, numClients int, version string, serfKey string) []node.Node {
	clients := make([]node.Node, numClients)
	for i := 0; i < numClients; i++ {
		var err error
		clients[i], err = node.NewConsulContainer(context.Background(),
			node.Config{
				HCL: fmt.Sprintf(`
				node_name = %q
				log_level = "TRACE"
				encrypt = %q`, utils.RandName("consul-client"), serfKey),
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: version,
			})
		require.NoError(t, err)
	}
	return clients
}

func serviceCreate(t *testing.T, client *api.Client, serviceName string) uint64 {
	err := client.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: serviceName, Port: 9999})
	require.NoError(t, err)

	service, meta, err := client.Catalog().Service(serviceName, "", &api.QueryOptions{})
	require.NoError(t, err)
	require.Len(t, service, 1)
	require.Equal(t, serviceName, service[0].ServiceName)
	require.Equal(t, 9999, service[0].ServicePort)

	return meta.LastIndex
}

func serversCluster(t *testing.T, numServers int, version string) *cluster.Cluster {
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

	waitForLeader(t, cluster, nil)
	waitForMembers(t, cluster.Nodes[0].GetClient(), numServers)

	return cluster
}

func Terminate(t *testing.T, cluster *cluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
