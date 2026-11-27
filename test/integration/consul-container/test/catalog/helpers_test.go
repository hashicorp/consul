// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package catalog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// aeTimer bounds waits that depend on the anti-entropy full sync, which runs
// roughly every minute on small clusters.
var aeTimer = &retry.Timer{Timeout: 3 * time.Minute, Wait: 2 * time.Second}

// newCluster creates a one server + one client datacenter. Services are
// registered through the client agent so that anti-entropy is exercised.
func newCluster(t *testing.T) (cluster *libcluster.Cluster, server, client *api.Client) {
	t.Helper()

	cluster, _, client = libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	})
	return cluster, cluster.Agents[0].GetClient(), client
}

func serviceInstances(t retry.TestingTB, c *api.Client, name string, opts *api.QueryOptions) []*api.CatalogService {
	svcs, _, err := c.Catalog().Service(name, "", opts)
	require.NoError(t, err)
	return svcs
}

func healthStatus(t retry.TestingTB, c *api.Client, service, checkID string) string {
	checks, _, err := c.Health().Checks(service, nil)
	require.NoError(t, err)
	for _, chk := range checks {
		if chk.CheckID == checkID {
			return chk.Status
		}
	}
	return ""
}
