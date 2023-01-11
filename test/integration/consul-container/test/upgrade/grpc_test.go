package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"fortio.org/fortio/fgrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/test/integration/consul-container/test/topology"
)

// Test service intention continues functioning after upgrade
// Steps:
// 1. Create a cluster with one server and one client
// 2. Register a static-server at server agent and a static-client at the client agent
// 3. Upgrade the cluster
// 4. Write the service-intention to allow the connection
func TestGRPC_UpgradeToTarget_fromLatest(t *testing.T) {
	type testcase struct {
		oldversion    string
		targetVersion string
	}
	tcs := []testcase{
		//{
		//  TODO: Recreat config during upgrade due to the breaking change to ports.grpc_tls
		// 	oldversion:    "1.13",
		// 	targetVersion: *libutils.TargetVersion,
		// },
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
		_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(clientNodes[0])
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

		/*
				cluster.ConfigEntryWrite(`
			Kind      = "service-defaults"
			Name      = "static-server"
			Protocol  = "http2"
			`)
				cluster.ConfigEntryWrite(`
			Kind      = "proxy-defaults"
			Name      = "global"
			Config {
			protocol = "http2"
			}
			`)
		*/

		libassert.CatalogServiceHealthy(t, client, "static-server-sidecar-proxy")
		_, clientPort := staticClientSvcSidecar.GetAddr()

		grpcPing := func() {
			pingConn, err := grpc.Dial(fmt.Sprintf("localhost:%d", clientPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
			require.NoError(t, err)
			pingCl := fgrpc.NewPingServerClient(pingConn)
			// TODO: not sure why, but this doesn't work the first time, needs some time to become ready
			retry.Run(t, func(r *retry.R) {
				_, err = pingCl.Ping(context.Background(), &fgrpc.PingMessage{})
				if err != nil {
					r.Fatalf("ping: %s", err)
				}
			})
		}
		grpcPing()

		// Upgrade the cluster to targetVersion
		t.Logf("Upgrade to version %s", tc.targetVersion)
		err = cluster.StandardUpgrade(t, context.Background(), tc.targetVersion)
		require.NoError(t, err)

		grpcPing()
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		time.Sleep(3 * time.Second)
	}
}
