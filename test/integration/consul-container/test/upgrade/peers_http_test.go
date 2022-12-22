package upgrade

import (
	"context"
	"fmt"
	// "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/test/integration/consul-container/test/utils"
)

// TestPeering_UpgradeToTarget_fromLatest checks peering status after dialing cluster
// and accepting cluster upgrade
func TestPeering_UpgradeToTarget_fromLatest(t *testing.T) {
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
			targetVersion: *libutils.TargetVersion,
		},
	}

	run := func(t *testing.T, tc testcase) {
		acceptingCluster, dialingCluster, _, staticClientSvcSidecar := utils.BasicPeeringTwoClustersSetup(t, tc.oldversion)
		// move to teardown
		defer func() {
			err := acceptingCluster.Terminate()
			require.NoErrorf(t, err, "termining accepting cluster")
			dialingCluster.Terminate()
			require.NoErrorf(t, err, "termining dialing cluster")
		}()

		clientNodes, err := dialingCluster.Clients()
		require.NoError(t, err)
		require.True(t, len(clientNodes) > 0)
		clientNode := clientNodes[0]
		dialingClient := clientNode.GetClient()
		_, port := staticClientSvcSidecar.GetAddr()

		// Upgrade the dialingCluster cluster and assert peering is still ACTIVE
		err = dialingCluster.StandardUpgrade(t, context.Background(), tc.targetVersion)
		require.NoError(t, err)
		libassert.PeeringStatus(t, dialingClient, utils.DialingPeerName, api.PeeringStateActive)
		libassert.HTTPServiceEchoes(t, "localhost", port)

		// Upgrade the accepting cluster and assert peering is still ACTIVE
		err = acceptingCluster.StandardUpgrade(t, context.Background(), tc.targetVersion)
		require.NoError(t, err)

		libassert.PeeringStatus(t, dialingClient, utils.DialingPeerName, api.PeeringStateActive)
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		time.Sleep(3 * time.Second)
	}
}
