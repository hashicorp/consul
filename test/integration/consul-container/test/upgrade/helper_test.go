package upgrade

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

func serversCluster(t *testing.T, numServers int, image, version string) *libcluster.Cluster {
	t.Helper()

	opts := libcluster.BuildOptions{
		ConsulImageName: image,
		ConsulVersion:   version,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	conf := libcluster.NewConfigBuilder(ctx).
		Bootstrap(numServers).
		ToAgentConfig(t)
	t.Logf("Cluster server config:\n%s", conf.JSON)

	cluster, err := libcluster.NewN(t, *conf, numServers)
	require.NoError(t, err)

	libcluster.WaitForLeader(t, cluster, nil)
	libcluster.WaitForMembers(t, cluster.APIClient(0), numServers)

	return cluster
}

func clientsCreate(t *testing.T, numClients int, image, version string, cluster *libcluster.Cluster) {
	t.Helper()

	opts := libcluster.BuildOptions{
		ConsulImageName: image,
		ConsulVersion:   version,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	conf := libcluster.NewConfigBuilder(ctx).
		Client().
		ToAgentConfig(t)
	t.Logf("Cluster client config:\n%s", conf.JSON)

	require.NoError(t, cluster.AddN(*conf, numClients, true))
}

func serviceCreate(t *testing.T, client *api.Client, serviceName string) uint64 {
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: serviceName,
		Port: 9999,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Port: 22005,
			},
		},
	}))

	service, meta, err := client.Catalog().Service(serviceName, "", &api.QueryOptions{})
	require.NoError(t, err)
	require.Len(t, service, 1)
	require.Equal(t, serviceName, service[0].ServiceName)
	require.Equal(t, 9999, service[0].ServicePort)

	return meta.LastIndex
}

func serviceHealthBlockingQuery(client *api.Client, serviceName string, waitIndex uint64) (chan []*api.ServiceEntry, chan error) {
	var (
		ch    = make(chan []*api.ServiceEntry, 1)
		errCh = make(chan error, 1)
	)
	go func() {
		opts := &api.QueryOptions{WaitIndex: waitIndex}
		service, q, err := client.Health().Service(serviceName, "", false, opts)
		if err == nil && q.QueryBackend != api.QueryBackendStreaming {
			err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
		}
		if err != nil {
			errCh <- err
		} else {
			ch <- service
		}
	}()

	return ch, errCh
}
