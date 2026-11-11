// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package dataplane

import (
	"testing"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// env is a datacenter with numServers servers and one client agent. conn is
// the gRPC connection to the first server.
type env struct {
	cluster *libcluster.Cluster
	server  *api.Client
	client  *api.Client
	conn    *grpc.ClientConn
}

func newEnv(t *testing.T, servers int) *env {
	t.Helper()

	cluster, _, client := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: servers,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	})
	return &env{
		cluster: cluster,
		server:  cluster.Agents[0].GetClient(),
		client:  client,
		conn:    cluster.Agents[0].GetGRPCConn(),
	}
}
