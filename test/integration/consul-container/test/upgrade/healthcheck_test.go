package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"

	libagent "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Test health check GRPC call using Target Servers and Latest GA Clients
func TestTargetServersWithLatestGAClients(t *testing.T) {
	const (
		numServers = 3
		numClients = 1
	)

	cluster := serversCluster(t, numServers, utils.TargetImage, utils.TargetVersion)
	defer terminate(t, cluster)

	clients := clientsCreate(t, numClients, utils.LatestImage, utils.LatestVersion, cluster)

	require.NoError(t, cluster.Join(clients))

	client := cluster.Agents[0].GetClient()

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 4)

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
	var configs []libagent.Config

	leaderConf, err := libagent.NewConfigBuilder(nil).ToAgentConfig()
	require.NoError(t, err)

	configs = append(configs, *leaderConf)

	// This needs a specialized config since it is using an older version of the agent.
	// That is missing fields like GRPC_TLS and PEERING, which are passed as defaults
	serverConf := `{
		"advertise_addr": "{{ GetInterfaceIP \"eth0\" }}",
		"bind_addr": "0.0.0.0",
		"client_addr": "0.0.0.0",
		"log_level": "DEBUG",
		"server": true,
		"bootstrap_expect": 3
	}`

	for i := 1; i < 3; i++ {
		configs = append(configs,
			libagent.Config{
				JSON:    serverConf,
				Cmd:     []string{"agent"},
				Version: utils.LatestVersion,
				Image:   utils.LatestImage,
			})
	}

	cluster, err := libcluster.New(configs)
	require.NoError(t, err)
	defer terminate(t, cluster)

	const (
		numClients = 1
	)

	clients := clientsCreate(t, numClients, utils.LatestImage, utils.LatestVersion, cluster)

	require.NoError(t, cluster.Join(clients))

	client := clients[0].GetClient()

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 4)

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
	var configs []libagent.Config

	for i := 0; i < 2; i++ {
		serverConf, err := libagent.NewConfigBuilder(nil).Bootstrap(3).ToAgentConfig()
		require.NoError(t, err)
		configs = append(configs, *serverConf)
	}

	leaderConf := `{
		"advertise_addr": "{{ GetInterfaceIP \"eth0\" }}",
		"bind_addr": "0.0.0.0",
		"client_addr": "0.0.0.0",
		"log_level": "DEBUG",
		"server": true
	}`

	configs = append(configs,
		libagent.Config{
			JSON:    leaderConf,
			Cmd:     []string{"agent"},
			Version: utils.LatestVersion,
			Image:   utils.LatestImage,
		})

	cluster, err := libcluster.New(configs)
	require.NoError(t, err)
	defer terminate(t, cluster)

	const (
		numClients = 1
	)

	clients := clientsCreate(t, numClients, utils.LatestImage, utils.LatestVersion, cluster)

	require.NoError(t, cluster.Join(clients))

	client := clients[0].GetClient()

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 4)

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

	timer := time.NewTimer(3 * time.Second)
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

func clientsCreate(t *testing.T, numClients int, image string, version string, cluster *libcluster.Cluster) []libagent.Agent {
	clients := make([]libagent.Agent, numClients)

	// This needs a specialized config since it is using an older version of the agent.
	// That is missing fields like GRPC_TLS and PEERING, which are passed as defaults
	conf := `{
		"advertise_addr": "{{ GetInterfaceIP \"eth0\" }}",
		"bind_addr": "0.0.0.0",
		"client_addr": "0.0.0.0",
		"log_level": "DEBUG"
	}`

	for i := 0; i < numClients; i++ {
		var err error
		clients[i], err = libagent.NewConsulContainer(context.Background(),
			libagent.Config{
				JSON:    conf,
				Cmd:     []string{"agent"},
				Version: version,
				Image:   image,
			},
			cluster.NetworkName,
			cluster.Index)
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

func serversCluster(t *testing.T, numServers int, image string, version string) *libcluster.Cluster {
	var configs []libagent.Config

	conf, err := libagent.NewConfigBuilder(nil).
		Bootstrap(3).
		ToAgentConfig()
	require.NoError(t, err)
	conf.Image = image
	conf.Version = version

	for i := 0; i < numServers; i++ {
		configs = append(configs, *conf)
	}
	cluster, err := libcluster.New(configs)
	require.NoError(t, err)

	libcluster.WaitForLeader(t, cluster, nil)
	libcluster.WaitForMembers(t, cluster.Agents[0].GetClient(), numServers)

	return cluster
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
