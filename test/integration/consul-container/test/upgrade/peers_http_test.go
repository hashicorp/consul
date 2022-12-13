package upgrade

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const (
	acceptingPeerName = "accepting-to-dialer"
	dialingPeerName   = "dialing-to-acceptor"
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
			targetVersion: *utils.TargetVersion,
		},
	}

	run := func(t *testing.T, tc testcase) {
		var acceptingCluster, dialingCluster *libcluster.Cluster
		var acceptingClient, dialingClient *api.Client

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			acceptingCluster, acceptingClient, _ = libcluster.CreatingAcceptingClusterAndSetup(t, 3, tc.oldversion, acceptingPeerName)
			wg.Done()
		}()
		defer func() {
			terminate(t, acceptingCluster)
		}()

		wg.Add(1)
		go func() {
			dialingCluster, dialingClient, _ = libcluster.CreateDialingClusterAndSetup(t, tc.oldversion, dialingPeerName)
			wg.Done()
		}()
		defer func() {
			terminate(t, dialingCluster)
		}()
		wg.Wait()

		err := dialingCluster.PeerWithCluster(acceptingClient, acceptingPeerName, dialingPeerName)
		require.NoError(t, err)

		libassert.PeeringStatus(t, acceptingClient, acceptingPeerName, api.PeeringStateActive)
		libassert.PeeringExports(t, acceptingClient, acceptingPeerName, 1)

		// Upgrade the dialingCluster cluster and assert peering is still ACTIVE
		err = dialingCluster.StandardUpgrade(t, context.Background(), tc.targetVersion)
		require.NoError(t, err)
		libassert.PeeringStatus(t, dialingClient, dialingPeerName, api.PeeringStateActive)

		// Upgrade the accepting cluster and assert peering is still ACTIVE
		err = acceptingCluster.StandardUpgrade(t, context.Background(), tc.targetVersion)
		require.NoError(t, err)

		libassert.PeeringStatus(t, acceptingClient, acceptingPeerName, api.PeeringStateActive)
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		time.Sleep(3 * time.Second)
	}
}
