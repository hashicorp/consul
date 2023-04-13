package test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

func TestManyClusters(t *testing.T) {
	t.Parallel()
	const n = 8
	for i := 0; i < n; i++ {
		t.Run(fmt.Sprintf("cluster %d", i), func(t *testing.T) {
			t.Parallel()
			topology.NewCluster(t, &topology.ClusterConfig{
				NumServers: 1,
				NumClients: 1,
				BuildOpts: &cluster.BuildOptions{
					Datacenter:    fmt.Sprintf("dc%d", i),
					ConsulVersion: utils.TargetVersion,
				},
				ApplyDefaultProxySettings: true,
			})
		})

	}
}
