package api

import (
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
)

type ConfigCallback func(c *Config)

func MakeTestClient(t *testing.T) (*Client, *testutil.TestServer) {
	return MakeTestClientWithConfig(t, nil, nil)
}

func MakeTestClientWithoutConnect(t *testing.T) (*Client, *testutil.TestServer) {
	return MakeTestClientWithConfig(t, nil, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.Connect = nil
	})
}

func MakeTestACLClient(t *testing.T) (*Client, *testutil.TestServer) {
	return MakeTestClientWithConfig(t, func(clientConfig *Config) {
		clientConfig.Token = "root"
	}, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.PrimaryDatacenter = "dc1"
		serverConfig.ACLMasterToken = "root"
		serverConfig.ACL.Enabled = true
		serverConfig.ACLDefaultPolicy = "deny"
	})
}

func MakeTestClientWithConfig(
	t *testing.T,
	cb1 ConfigCallback,
	cb2 testutil.ServerConfigCallback) (*Client, *testutil.TestServer) {

	// Make client config
	conf := DefaultConfig()
	if cb1 != nil {
		cb1(conf)
	}
	// Create server
	server, err := testutil.NewTestServerConfigT(t, cb2)
	if err != nil {
		t.Fatal(err)
	}
	conf.Address = server.HTTPAddr

	// Create client
	client, err := NewClient(conf)
	if err != nil {
		server.Stop()
		t.Fatalf("err: %v", err)
	}

	return client, server
}
