package l7_traffic_management

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

// TestTrafficManagement_ResolverDefaultSubset_Agentless tests resolver directs traffic to default subset - agentless
//   - Create a topology with static-server (meta version V2) and static-client (with static-server upstream)
//   - Create a service resolver with V2 as default subset
//   - Resolver directs traffic to the default subset, which is V2
//   - Do a standard upgrade and validate the traffic is still directed to V2
//   - Change the default version in serviceResolver to v1 and the client to server request fails
//     since we only have V2 instance
//   - Change the default version in serviceResolver to v2 and the client to server request succeeds
//     (V2 instance is available and traffic is directed to it)
func TestTrafficManagement_ResolverDefaultSubset_Agentless(t *testing.T) {
	t.Parallel()

	ct := NewCommonTopo(t)
	configEntries := ct.Cfg.Clusters[0].InitialConfigEntries
	configEntries = addServiceResolver(configEntries, staticServerSID.Name, "v2")
	ct.Cfg.Clusters[0].InitialConfigEntries = configEntries
	ct.Launch(t)

	resolverV2AssertFn := func() {
		cluster := ct.Sprawl.Topology().Clusters[dc1]
		staticClientWorkload := cluster.WorkloadByID(
			topology.NewNodeID("dc1-client2", defaultPartition),
			ct.StaticClientSID,
		)
		ct.Assert.FortioFetch2HeaderEcho(t, staticClientWorkload, &topology.Destination{
			ID:        ct.StaticServerSID,
			LocalPort: 5000,
		})
		ct.Assert.FortioFetch2FortioName(t, staticClientWorkload, &topology.Destination{
			ID:        ct.StaticServerSID,
			LocalPort: 5000,
		}, dc1, staticServerSID)
		ct.Assert.DestinationEndpointStatus(t, staticClientWorkload, "v2.static-server.default", "HEALTHY", 1)
	}
	resolverV2AssertFn()

	t.Log("Start standard upgrade ...")
	sp := ct.Sprawl
	cfg := sp.Config()
	require.NoError(t, ct.Sprawl.LoadKVDataToCluster("dc1", 1, &api.WriteOptions{}))
	require.NoError(t, sp.Upgrade(cfg, "dc1", sprawl.UpgradeTypeStandard, utils.TargetImages(), nil))
	t.Log("Finished standard upgrade ...")

	// verify data is not lost
	data, err := ct.Sprawl.GetKV("dc1", "key-0", &api.QueryOptions{})
	require.NoError(t, err)
	require.NotNil(t, data)

	ct.ValidateTopology(t)
	resolverV2AssertFn()

	// Change the default version in serviceResolver to v1 and the client to server request fails
	configEntries = cfg.Clusters[0].InitialConfigEntries
	configEntries = removeServiceResolvers(configEntries, staticServerSID.Name)
	configEntries = addServiceResolver(configEntries, staticServerSID.Name, "v1")
	cfg.Clusters[0].InitialConfigEntries = configEntries

	require.NoError(t, ct.Sprawl.RelaunchWithPhase(cfg, sprawl.LaunchPhaseRegular))
	t.Log("Finished first relaunch ...")
	ct.ValidateTopology(t)

	resolverV1AssertFn := func() {
		cluster := ct.Sprawl.Topology().Clusters[dc1]
		staticClientWorkload := cluster.WorkloadByID(
			topology.NewNodeID("dc1-client2", defaultPartition),
			ct.StaticClientSID,
		)
		ct.Assert.FortioFetch2ServiceUnavailable(t, staticClientWorkload, &topology.Destination{
			ID:        ct.StaticServerSID,
			LocalPort: 5000,
		})
	}
	resolverV1AssertFn()

	// Change the default version in serviceResolver to v2 and the client to server request succeeds
	cfg = ct.Sprawl.Config()
	configEntries = cfg.Clusters[0].InitialConfigEntries
	configEntries = removeServiceResolvers(configEntries, staticServerSID.Name)
	configEntries = addServiceResolver(configEntries, staticServerSID.Name, "v2")
	cfg.Clusters[0].InitialConfigEntries = configEntries

	require.NoError(t, ct.Sprawl.RelaunchWithPhase(cfg, sprawl.LaunchPhaseRegular))
	t.Log("Finished second relaunch ...")
	ct.ValidateTopology(t)
	resolverV2AssertFn()
}

func addServiceResolver(configEntries []api.ConfigEntry, serviceResolverName string, defaultSubset string) []api.ConfigEntry {
	configEntries = append(configEntries,
		&api.ServiceResolverConfigEntry{
			Kind:          api.ServiceResolver,
			Name:          serviceResolverName,
			DefaultSubset: defaultSubset,
			Subsets: map[string]api.ServiceResolverSubset{
				"v1": {
					Filter: "Service.Meta.version == v1",
				},
				"v2": {
					Filter: "Service.Meta.version == v2",
				},
			},
		},
	)
	return configEntries
}

func removeServiceResolvers(configEntries []api.ConfigEntry, serviceResolverNames ...string) []api.ConfigEntry {
	var resConfigEntries []api.ConfigEntry
	containsString := func(element string, slice []string) bool {
		for _, v := range slice {
			if v == element {
				return true
			}
		}
		return false
	}

	for _, entry := range configEntries {
		if entry.GetKind() == api.ServiceResolver && containsString(entry.GetName(), serviceResolverNames) {
			continue
		}
		resConfigEntries = append(resConfigEntries, entry)
	}
	return resConfigEntries
}
