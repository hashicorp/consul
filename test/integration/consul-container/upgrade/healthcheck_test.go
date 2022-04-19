package consul_container

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"

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

// Test health check GRPC call using Current Clients and Latest GA Servers
func TestLatestGAServersWithCurrentClients(t *testing.T) {
	t.Parallel()
	numServers := 3
	Cluster, err := serversCluster(t, numServers, *latestImage)
	require.NoError(t, err)
	defer Terminate(t, Cluster)
	numClients := 2
	Clients, err := clientsCreate(numClients)
	client := Clients[0].GetClient()
	err = Cluster.AddNodes(Clients)
	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := Cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
		members, err := client.Agent().Members(false)
		require.Len(r, members, 5)
	})

	serviceName := "api"
	err, index := serviceCreate(t, client, serviceName)

	ch := make(chan []*api.ServiceEntry)
	errCh := make(chan error)

	go func() {
		service, q, err := client.Health().Service(serviceName, "", false, &api.QueryOptions{WaitIndex: index})
		if q.QueryBackend != api.QueryBackendStreaming || q.QueryBackend == "" {
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

// Test health check GRPC call using Current Servers and Latest GA Clients
func TestCurrentServersWithLatestGAClients(t *testing.T) {
	t.Parallel()
	numServers := 3
	Cluster, err := serversCluster(t, numServers, *curImage)
	require.NoError(t, err)
	defer Terminate(t, Cluster)
	numClients := 1

	Clients, err := clientsCreate(numClients)
	client := Cluster.Nodes[0].GetClient()
	err = Cluster.AddNodes(Clients)
	retry.RunWith(&retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}, t, func(r *retry.R) {
		leader, err := Cluster.Leader()
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
		if q.QueryBackend != api.QueryBackendStreaming || q.QueryBackend == "" {
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

	numClients := 1
	Clients, err := clientsCreate(numClients)
	client := Clients[0].GetClient()
	err = cluster.AddNodes(Clients)
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
		if q.QueryBackend != api.QueryBackendStreaming || q.QueryBackend == "" {
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

// Test health check GRPC call using Mixed (majority current) Servers and Latest GA Clients
func TestMixedServersMajorityCurrentGAClient(t *testing.T) {
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

	numClients := 1
	clients, err := clientsCreate(numClients)
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
		if q.QueryBackend != api.QueryBackendStreaming || q.QueryBackend == "" {
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

func clientsCreate(numClients int) ([]consulNode.ConsulNode, error) {
	Clients := make([]consulNode.ConsulNode, numClients)
	var err error
	for i := 0; i < numClients; i++ {
		Clients[i], err = consulNode.NewConsulContainer(context.Background(),
			consulNode.Config{
				HCL: `node_name="` + utils.RandName("consul-client") + `"
					log_level="TRACE"`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: *curImage,
			})
	}
	return Clients, err
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
