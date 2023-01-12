package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/test/integration/consul-container/test/topology"
)

// Test GRPC continues functioning after upgrade
// Steps:
// 1. Create a cluster with one server and one client
// 2. Register a static-server at server agent and a static-client at the client agent
// 3. Do some tests (namely, a GRPC ping)
// 4. Upgrade the cluster
// 5. Repeat tests
//
// This is based on envoy/case-grpc bats test
func TestGRPC_UpgradeToTarget_fromLatest(t *testing.T) {
	type testcase struct {
		oldversion    string
		targetVersion string
	}
	tcs := []testcase{
		{
			oldversion:    "1.14",
			targetVersion: *libutils.TargetVersion,
		},
	}

	run := func(t *testing.T, tc testcase) {
		cluster := topology.BasicSingleClusterTopology(t, &libcluster.Options{
			Datacenter: "dc1",
			NumServer:  1,
			NumClient:  1,
			Version:    tc.oldversion,
		})
		defer func() {
			err := cluster.Terminate()
			require.NoErrorf(t, err, "termining cluster")
		}()

		// Register an static-server service
		clientNodes, err := cluster.Clients()
		require.NoError(t, err)
		require.True(t, len(clientNodes) > 0)
		_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(clientNodes[0], true)
		require.NoError(t, err)
		client := clientNodes[0].GetClient()
		libassert.CatalogServiceExists(t, client, "static-server")
		libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy")

		// Register an static-client service
		serverNodes, err := cluster.Servers()
		require.NoError(t, err)
		require.True(t, len(serverNodes) > 0)
		staticClientSvcSidecar, err := libservice.CreateAndRegisterStaticClientSidecar(serverNodes[0], "", true)
		require.NoError(t, err)

		libassert.CatalogServiceHealthy(t, client, "static-server-sidecar-proxy")
		_, clientPort := staticClientSvcSidecar.GetAddr()
		// TODO: other checks from case-grpc/verify.bats:
		// - check proxy admin is up
		// - check cert URIs
		// - check client has healthy upstream endpoints to server
		// - check client is sending gRPC metrics to statsd

		tests := func() {
			libassert.GRPCPing(t, fmt.Sprintf("localhost:%d", clientPort))
		}
		tests()

		// Upgrade the cluster to targetVersion
		t.Logf("Upgrade to version %s", tc.targetVersion)
		err = cluster.StandardUpgrade(t, context.Background(), tc.targetVersion)
		require.NoError(t, err)

		tests()
		// TODO: envoy grpc filter verification
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		time.Sleep(3 * time.Second)
	}
}
