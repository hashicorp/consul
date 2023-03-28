// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"os"
	"testing"

	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

type testClusterConfig struct {
	Datacenter string
	Servers    int
	Clients    int
	ServerConf func(*Config)
	ClientConf func(*Config)

	ServerWait func(*testing.T, *Server)
	ClientWait func(*testing.T, *Client)
}

type testCluster struct {
	Servers      []*Server
	ServerCodecs []rpc.ClientCodec
	Clients      []*Client
}

func newTestCluster(t *testing.T, conf *testClusterConfig) *testCluster {
	t.Helper()

	require.NotNil(t, conf)
	cluster := testCluster{}

	// create the servers
	for i := 0; i < conf.Servers; i++ {
		dir, srv := testServerWithConfig(t, func(c *Config) {
			if conf.Datacenter != "" {
				c.Datacenter = conf.Datacenter
			}
			c.Bootstrap = false
			c.BootstrapExpect = conf.Servers

			if conf.ServerConf != nil {
				conf.ServerConf(c)
			}
		})
		t.Cleanup(func() { os.RemoveAll(dir) })
		t.Cleanup(func() { srv.Shutdown() })

		cluster.Servers = append(cluster.Servers, srv)

		codec := rpcClient(t, srv)

		cluster.ServerCodecs = append(cluster.ServerCodecs, codec)
		t.Cleanup(func() { codec.Close() })

		if i > 0 {
			joinLAN(t, srv, cluster.Servers[0])
		}
	}

	waitForLeaderEstablishment(t, cluster.Servers...)
	if conf.ServerWait != nil {
		for _, srv := range cluster.Servers {
			conf.ServerWait(t, srv)
		}
	}

	// create the clients
	for i := 0; i < conf.Clients; i++ {
		dir, client := testClientWithConfig(t, func(c *Config) {
			if conf.Datacenter != "" {
				c.Datacenter = conf.Datacenter
			}
			if conf.ClientConf != nil {
				conf.ClientConf(c)
			}
		})

		t.Cleanup(func() { os.RemoveAll(dir) })
		t.Cleanup(func() { client.Shutdown() })

		if len(cluster.Servers) > 0 {
			joinLAN(t, client, cluster.Servers[0])
		}

		cluster.Clients = append(cluster.Clients, client)
	}

	for _, client := range cluster.Clients {
		if conf.ClientWait != nil {
			conf.ClientWait(t, client)
		} else {
			testrpc.WaitForTestAgent(t, client.RPC, client.config.Datacenter)
		}
	}

	return &cluster
}
