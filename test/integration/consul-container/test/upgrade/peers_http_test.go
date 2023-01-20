package upgrade

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TestPeering_UpgradeToTarget_fromLatest checks peering status after dialing cluster
// and accepting cluster upgrade
func TestPeering_UpgradeToTarget_fromLatest(t *testing.T) {
	t.Parallel()

	type testcase struct {
		oldversion    string
		targetVersion string
	}
	tcs := []testcase{
		// {
		//  TODO: API changed from 1.13 to 1.14 in , PeerName to Peer
		//  exportConfigEntry
		// 	oldversion:    "1.13",
		// 	targetVersion: *utils.TargetVersion,
		// },
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
		},
	}

	run := func(t *testing.T, tc testcase) {
		accepting, dialing := libtopology.BasicPeeringTwoClustersSetup(t, tc.oldversion)
		var (
			acceptingCluster = accepting.Cluster
			dialingCluster   = dialing.Cluster
		)

		dialingClient, err := dialingCluster.GetClient(nil, false)
		require.NoError(t, err)

		acceptingClient, err := acceptingCluster.GetClient(nil, false)
		require.NoError(t, err)

		// Upgrade the accepting cluster and assert peering is still ACTIVE
		require.NoError(t, acceptingCluster.StandardUpgrade(t, context.Background(), tc.targetVersion))

		libassert.PeeringStatus(t, acceptingClient, libtopology.AcceptingPeerName, api.PeeringStateActive)
		libassert.PeeringStatus(t, dialingClient, libtopology.DialingPeerName, api.PeeringStateActive)
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		// time.Sleep(3 * time.Second)
	}
}
