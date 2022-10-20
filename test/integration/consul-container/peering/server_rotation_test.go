package peering

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/integration/consul-container/libs/cluster"
	libnode "github.com/hashicorp/consul/integration/consul-container/libs/node"
	libservice "github.com/hashicorp/consul/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/integration/consul-container/libs/utils"
)

const (
	acceptingPeerName = "accepting-to-dialer"
	dialingPeerName   = "dialing-to-acceptor"
)

// TestBasicConnectService SUMMARY
// This test makes sure that the peering stream send server address updates between peers.
// It also verifies that dialing clusters will use this stored information to supersede the addresses
// encoded in the peering token.
//
// Steps:
//  * Create an accepting cluster with 3 servers. 1 client should be used to host a service for export
//  * Create a single node dialing cluster.
//	* Create the peering and export the service. Verify it is working
//  * Incrementally replace the follower nodes.
// 	* Replace the leader node
//  * Verify the dialer can reach the new server nodes and the service becomes available.
func TestServerRotation(t *testing.T) {
	var wg sync.WaitGroup
	var acceptingCluster, dialingCluster *libcluster.Cluster
	var acceptingClient, dialingClient *api.Client
	var clientService libservice.Service
	wg.Add(1)
	go func() {
		acceptingCluster, acceptingClient = creatingAcceptingClusterAndSetup(t)
		wg.Done()
	}()
	defer func() {
		terminate(t, acceptingCluster)
	}()

	wg.Add(1)
	go func() {
		dialingCluster, dialingClient, clientService = createDialingClusterAndSetup(t)
		wg.Done()
	}()
	defer func() {
		terminate(t, dialingCluster)
	}()

	wg.Wait()

	// Establish Peering
	generateReq := api.PeeringGenerateTokenRequest{
		PeerName: acceptingPeerName,
	}
	generateRes, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), generateReq, &api.WriteOptions{})
	require.NoError(t, err)

	establishReq := api.PeeringEstablishRequest{
		PeerName:     dialingPeerName,
		PeeringToken: generateRes.PeeringToken,
	}
	_, _, err = dialingClient.Peerings().Establish(context.Background(), establishReq, &api.WriteOptions{})
	require.NoError(t, err)

	libassert.PeeringStatus(t, acceptingClient, acceptingPeerName, api.PeeringStateActive)
	libassert.PeeringExports(t, acceptingClient, acceptingPeerName, 1)

	_, port := clientService.GetAddr()
	libassert.HTTPServiceEchoes(t, "localhost", port)

	// Start by replacing the Followers
	leader, err := acceptingCluster.Leader()
	require.NoError(t, err)

	followers, err := acceptingCluster.Followers()
	require.NoError(t, err)
	require.Len(t, followers, 2)

	for idx, follower := range followers {
		t.Log("Removing follower", idx)
		rotateServer(t, acceptingCluster, acceptingClient, follower)
	}

	t.Log("Removing leader")
	rotateServer(t, acceptingCluster, acceptingClient, leader)

	libassert.PeeringStatus(t, acceptingClient, acceptingPeerName, api.PeeringStateActive)
	libassert.PeeringExports(t, acceptingClient, acceptingPeerName, 1)

	libassert.HTTPServiceEchoes(t, "localhost", port)
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}

// creatingAcceptingClusterAndSetup creates a cluster with 3 servers and 1 client.
// It also creates and registers a service+sidecar.
// The API client returned is pointed at the client agent.
func creatingAcceptingClusterAndSetup(t *testing.T) (*libcluster.Cluster, *api.Client) {
	var configs []libnode.Config

	numServer := 3
	for i := 0; i < numServer; i++ {
		configs = append(configs,
			libnode.Config{
				HCL: `node_name="` + utils.RandName("consul-server") + `"
					ports {
					  dns = 8600
					  http = 8500
					  https = 8501
					  grpc = 8502
    				  grpc_tls = 8503
					  serf_lan = 8301
					  serf_wan = 8302
					  server = 8300
					}	
					bind_addr = "0.0.0.0"
					advertise_addr = "{{ GetInterfaceIP \"eth0\" }}"
					log_level="DEBUG"
					server=true
					peering {
						enabled=true
					}
					bootstrap_expect=3
					connect {
					  enabled = true
					}`,
				Cmd:     []string{"agent", "-client=0.0.0.0"},
				Version: *utils.TargetImage,
			},
		)
	}

	// Add a stable client to register the service
	configs = append(configs,
		libnode.Config{
			HCL: `node_name="` + utils.RandName("consul-client") + `"
					ports {
					  dns = 8600
					  http = 8500
					  https = 8501
					  grpc = 8502
    				  grpc_tls = 8503
					  serf_lan = 8301
					  serf_wan = 8302
					}	
					bind_addr = "0.0.0.0"
					advertise_addr = "{{ GetInterfaceIP \"eth0\" }}"
					log_level="DEBUG"
					peering {
						enabled=true
					}
					connect {
					  enabled = true
					}`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: *utils.TargetImage,
		},
	)

	cluster, err := libcluster.New(configs)
	require.NoError(t, err)

	// Use the client agent as the HTTP endpoint since we will not rotate it
	clientNode := cluster.Nodes[3]
	client := clientNode.GetClient()
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 4)

	// Default Proxy Settings
	ok, err := utils.ApplyDefaultProxySettings(client)
	require.NoError(t, err)
	require.True(t, ok)

	// Create the mesh gateway for dataplane traffic
	_, err = libservice.NewGatewayService(context.Background(), "mesh", "mesh", clientNode)
	require.NoError(t, err)

	// Create a service and proxy instance
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(clientNode)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server")
	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy")

	// Export the service
	config := &api.ExportedServicesConfigEntry{
		Name: "default",
		Services: []api.ExportedService{
			{
				Name: "static-server",
				Consumers: []api.ServiceConsumer{
					{Peer: acceptingPeerName},
				},
			},
		},
	}
	ok, _, err = client.ConfigEntries().Set(config, &api.WriteOptions{})
	require.NoError(t, err)
	require.True(t, ok)

	return cluster, client
}

// createDialingClusterAndSetup creates a cluster for peering with a single dev agent
func createDialingClusterAndSetup(t *testing.T) (*libcluster.Cluster, *api.Client, libservice.Service) {
	configs := []libnode.Config{
		{
			HCL: `ports {
  dns = 8600
  http = 8500
  https = 8501
  grpc = 8502
  grpc_tls = 8503
  serf_lan = 8301
  serf_wan = 8302
  server = 8300
}
bind_addr = "0.0.0.0"
advertise_addr = "{{ GetInterfaceIP \"eth0\" }}"
log_level="DEBUG"
server=true
bootstrap = true
peering {
	enabled=true
}
datacenter = "dc2"
connect {
  enabled = true
}`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: *utils.TargetImage,
		},
	}

	cluster, err := libcluster.New(configs)
	require.NoError(t, err)

	node := cluster.Nodes[0]
	client := node.GetClient()
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 1)

	// Default Proxy Settings
	ok, err := utils.ApplyDefaultProxySettings(client)
	require.NoError(t, err)
	require.True(t, ok)

	// Create the mesh gateway for dataplane traffic
	_, err = libservice.NewGatewayService(context.Background(), "mesh", "mesh", node)
	require.NoError(t, err)

	// Create a service and proxy instance
	clientProxyService, err := libservice.CreateAndRegisterStaticClientSidecar(node, dialingPeerName, true)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy")

	return cluster, client, clientProxyService
}

// rotateServer add a new server node to the cluster, then forces the prior node to leave.
func rotateServer(t *testing.T, cluster *libcluster.Cluster, client *api.Client, node libnode.Node) {
	config := libnode.Config{
		HCL: `encrypt="` + cluster.EncryptKey + `"
			node_name="` + utils.RandName("consul-server") + `"
			ports {
			  dns = 8600
			  http = 8500
			  https = 8501
			  grpc = 8502
			  grpc_tls = 8503
			  serf_lan = 8301
			  serf_wan = 8302
			  server = 8300
			}	
			bind_addr = "0.0.0.0"
			advertise_addr = "{{ GetInterfaceIP \"eth0\" }}"
			log_level="DEBUG"
			server=true
			peering {
				enabled=true
			}
			connect {
			  enabled = true
			}`,
		Cmd:     []string{"agent", "-client=0.0.0.0"},
		Version: *utils.TargetImage,
	}

	c, err := libnode.NewConsulContainer(context.Background(), config)
	require.NoError(t, err, "could not start new node")

	require.NoError(t, cluster.AddNodes([]libnode.Node{c}))

	libcluster.WaitForMembers(t, client, 5)

	require.NoError(t, cluster.RemoveNode(node))

	libcluster.WaitForMembers(t, client, 4)
}
