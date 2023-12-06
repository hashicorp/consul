package usage_profiles

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

const (
	// The long term support version
	ltsVersion = "1.15.7"
)

// Test_Upgrade_ServiceDiscovery_Wan_Segment test upgrade from a source version
// to a specified long term support version
// Clusters: multi-segment and multi-cluster (TODO)
// Workload: service discovery (no mesh) (TODO)
func Test_Upgrade_ServiceDiscovery_Wan_Segment(t *testing.T) {
	utils.LatestVersion = "1.10.8"
	utils.TargetVersion = ltsVersion

	dc1, err := createTopology("dc1")
	require.NoError(t, err)
	t.Log("Created topology:", dc1.Name, "enterprise:", utils.IsEnterprise())

	toplogyConfig := &topology.Config{
		Networks: []*topology.Network{
			{Name: "dc1"},
		},
	}
	toplogyConfig.Clusters = append(toplogyConfig.Clusters, dc1)
	sp := sprawltest.Launch(t, toplogyConfig)

	cfg := sp.Config()
	require.NoError(t, sp.Upgrade(cfg, "dc1", sprawl.UpgradeTypeStandard, utils.TargetImages(), nil))
	t.Log("Finished standard upgrade ...")

	time.Sleep(30 * time.Second)
}

func createTopology(name string) (*topology.Cluster, error) {
	clu := &topology.Cluster{
		Name:   name,
		Images: utils.LatestImages(),
		Nodes: []*topology.Node{
			{
				Kind: topology.NodeKindServer,
				Name: "dc1-server1",
				Addresses: []*topology.Address{
					{Network: "dc1"},
				},
			},
			{
				Kind: topology.NodeKindClient,
				Name: "dc1-client1",
			},
		},
		Enterprise: utils.IsEnterprise(),
	}
	return clu, nil
}
