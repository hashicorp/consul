// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package jwtauth

import (
	"testing"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// newConfigClient creates a datacenter and returns a client to its server. The
// tests below exercise how servers validate and store JWT configuration, which
// needs no data plane.
func newConfigClient(t *testing.T) *api.Client {
	t.Helper()

	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 0,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	})
	return cluster.Agents[0].GetClient()
}
