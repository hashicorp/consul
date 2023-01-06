package topology

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
)

const (
	AcceptingPeerName = "accepting-to-dialer"
	DialingPeerName   = "dialing-to-acceptor"
)

// BasicPeeringTwoClustersSetup sets up a scenario for testing peering, which consists of
//   - an accepting cluster with 3 servers and 1 client agnet. The client should be used to
//     host a service for export: staticServerSvc.
//   - an dialing cluster with 1 server and 1 client. The client should be used to host a
//     service connecting to staticServerSvc.
//   - Create the peering, export the service from accepting cluster, and verify service
//     connectivity.
//
// It returns objects of the accepting cluster, dialing cluster, staticServerSvc, and staticClientSvcSidecar
func BasicPeeringTwoClustersSetup(t *testing.T, consulVersion string) (*libcluster.Cluster, *libcluster.Cluster, *libservice.Service, *libservice.ConnectContainer) {
	var wg sync.WaitGroup
	var acceptingCluster, dialingCluster *libcluster.Cluster
	var acceptingClient *api.Client

	wg.Add(1)
	go func() {
		opts := &libcluster.Options{
			Datacenter: "dc1",
			NumServer:  3,
			NumClient:  1,
			Version:    consulVersion,
		}
		acceptingCluster, acceptingClient = libcluster.CreatingPeeringClusterAndSetup(t, opts)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		opts := &libcluster.Options{
			Datacenter: "dc2",
			NumServer:  1,
			NumClient:  1,
			Version:    consulVersion,
		}
		dialingCluster, _ = libcluster.CreatingPeeringClusterAndSetup(t, opts)
		wg.Done()
	}()
	wg.Wait()

	err := dialingCluster.PeerWithCluster(acceptingClient, AcceptingPeerName, DialingPeerName)
	require.NoError(t, err)

	libassert.PeeringStatus(t, acceptingClient, AcceptingPeerName, api.PeeringStateActive)

	// Register an static-server service in acceptingCluster and export to dialing cluster
	clientNodes, err := acceptingCluster.Clients()
	require.NoError(t, err)
	require.True(t, len(clientNodes) > 0)
	staticServerSvc, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(clientNodes[0])
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, acceptingClient, "static-server")
	libassert.CatalogServiceExists(t, acceptingClient, "static-server-sidecar-proxy")

	staticServerSvc.Export("default", AcceptingPeerName, acceptingClient)
	libassert.PeeringExports(t, acceptingClient, AcceptingPeerName, 1)

	// Register an static-client service in dialing cluster and set upstream to static-server service
	clientNodesDialing, err := dialingCluster.Clients()
	require.NoError(t, err)
	require.True(t, len(clientNodesDialing) > 0)
	staticClientSvcSidecar, err := libservice.CreateAndRegisterStaticClientSidecar(clientNodesDialing[0], DialingPeerName, true)
	require.NoError(t, err)

	_, port := staticClientSvcSidecar.GetAddr()
	libassert.HTTPServiceEchoes(t, "localhost", port)

	return acceptingCluster, dialingCluster, &staticServerSvc, staticClientSvcSidecar
}
