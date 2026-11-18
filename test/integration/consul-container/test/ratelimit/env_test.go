// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ratelimit

import (
	"testing"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// newServer starts a one server datacenter with the given server configuration
// and returns the address of the server's HTTP port as seen by the test and a
// client for it.
func newServer(t *testing.T, serverHCL string) (string, *api.Client) {
	t.Helper()

	cluster, _, _ := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 0,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
		},
	}, serverHCL, "")

	agent := cluster.Agents[0]
	cfg := agent.GetAPIClientConfig()
	return utils.HostPort(cfg.Address), agent.GetClient()
}
