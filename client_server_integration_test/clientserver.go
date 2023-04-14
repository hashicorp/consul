package integration_test

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

type TestServerAdapter struct {
	cluster *libcluster.Cluster
}

func (a *TestServerAdapter) Stop() error {
	return nil
}

// assert that we fulfill the interface
var _ TestServerI = &TestServerAdapter{}

// TODO: name, location
type TestServerI interface {
	// TODO: not sure we really need this; just use t.Cleanup, so maybe a no-op
	Stop() error
}

func NewClusterTestServerAdapter(t *testing.T) (*api.Client, *TestServerAdapter) {
	// TODO: not sure what these values should be
	cluster_, _, client := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
		},
	})
	return client, &TestServerAdapter{
		cluster: cluster_,
	}
}

type configCallback func(c *api.Config)

func NewClientServer(t *testing.T) (*api.Client, *testutil.TestServer) {
	return makeClientWithConfig(t, nil, nil)
}

/* TODO
func makeClientWithoutConnect(t *testing.T) (*api.Client, *testutil.TestServer) {
	return makeClientWithConfig(t, nil, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.Connect = nil
	})
}

func makeACLClient(t *testing.T) (*api.Client, *testutil.TestServer) {
	return makeClientWithConfig(t, func(clientConfig *api.Config) {
		clientConfig.Token = "root"
	}, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.PrimaryDatacenter = "dc1"
		serverConfig.ACL.Tokens.InitialManagement = "root"
		serverConfig.ACL.Tokens.Agent = "root"
		serverConfig.ACL.Enabled = true
		serverConfig.ACL.DefaultPolicy = "deny"
	})
}

func makeNonBootstrappedACLClient(t *testing.T, defaultPolicy string) (*api.Client, *testutil.TestServer) {
	return makeClientWithConfig(t,
		func(clientConfig *api.Config) {
			clientConfig.Token = ""
		},
		func(serverConfig *testutil.TestServerConfig) {
			serverConfig.PrimaryDatacenter = "dc1"
			serverConfig.ACL.Enabled = true
			serverConfig.ACL.DefaultPolicy = defaultPolicy
			serverConfig.Bootstrap = true
		})
}

func makeClientWithCA(t *testing.T) (*api.Client, *testutil.TestServer) {
	return makeClientWithConfig(t,
		func(c *api.Config) {
			c.TLSConfig = api.TLSConfig{
				Address:  "consul.test",
				CAFile:   "../test/client_certs/rootca.crt",
				CertFile: "../test/client_certs/client.crt",
				KeyFile:  "../test/client_certs/client.key",
			}
		},
		func(c *testutil.TestServerConfig) {
			c.CAFile = "../test/client_certs/rootca.crt"
			c.CertFile = "../test/client_certs/server.crt"
			c.KeyFile = "../test/client_certs/server.key"
		})
}
*/

func makeClientWithConfig(
	t *testing.T,
	cb1 configCallback,
	cb2 testutil.ServerConfigCallback) (*api.Client, *testutil.TestServer) {
	// Skip test when -short flag provided; any tests that create a test
	// server will take at least 100ms which is undesirable for -short
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Make client config
	conf := api.DefaultConfig()
	if cb1 != nil {
		cb1(conf)
	}

	// Create server
	var server *testutil.TestServer
	var err error
	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		server, err = testutil.NewTestServerConfigT(t, cb2)
		if err != nil {
			r.Fatalf("Failed to start server: %v", err.Error())
		}
	})
	if server.Config.Bootstrap {
		server.WaitForLeader(t)
	}
	connectEnabled := server.Config.Connect["enabled"]
	if enabled, ok := connectEnabled.(bool); ok && server.Config.Server && enabled {
		server.WaitForActiveCARoot(t)
	}

	conf.Address = server.HTTPAddr

	// Create client
	client, err := api.NewClient(conf)
	if err != nil {
		server.Stop()
		t.Fatalf("err: %v", err)
	}

	return client, server
}
