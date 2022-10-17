package peering

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/integration/consul-container/libs/cluster"
	libnode "github.com/hashicorp/consul/integration/consul-container/libs/node"
	libservice "github.com/hashicorp/consul/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/integration/consul-container/libs/utils"
)

// TEST SUMMARY
// Purpose: This test makes sure that the peering stream send server address updates between peers.
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

const (
	acceptingPeerName = "accepting-to-dialer"
	dialingPeerName   = "dialing-to-acceptor"
)

func TestServerRotation(t *testing.T) {

	var wg sync.WaitGroup
	var acceptingCluster, dialingCluster *libcluster.Cluster
	var acceptingClient, dialingClient *api.Client
	var clientService libservice.Service

	wg.Add(1)
	go func() {
		acceptingCluster, acceptingClient = creatingAcceptingCluster(t)
		wg.Done()
	}()
	defer func() {
		terminate(t, acceptingCluster)
	}()

	wg.Add(1)
	go func() {
		dialingCluster, dialingClient, clientService = createDialingCluster(t)
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

	time.Sleep(20 * time.Second)

	_, _, err = dialingClient.Peerings().Establish(context.Background(), establishReq, &api.WriteOptions{})
	require.NoError(t, err)

	libassert.PeeringStatus(t, acceptingClient, acceptingPeerName, api.PeeringStateActive)
	//libassert.PeeringExports(t, acceptingClient, acceptingPeerName, 2) // Service + Sidecar

	time.Sleep(3600 * time.Second)

	ip, port := clientService.GetAddr()
	libassert.HTTPResponseContains(t, ip, port, "hello")

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
	//libassert.PeeringExports(t, acceptingClient, acceptingPeerName, 2) // Service + Sidecar

	// TODO (dans): verify the service works as expected
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}

// creatingAcceptingCluster creates a cluster with 3 servers and 1 client.
// It also creates and registers a service+sidecar.
// The API client returned is pointed at the client agent.
func creatingAcceptingCluster(t *testing.T) (*libcluster.Cluster, *api.Client) {
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
	_, err = libservice.NewGatewayService(context.Background(), "dc1-mesh", "mesh", clientNode)
	require.NoError(t, err)

	// Create a service and proxy instance
	serverService, err := libservice.NewExampleService(context.Background(), "static-server", 8080, 8079, clientNode)
	require.NoError(t, err)

	serverConnectProxy, err := libservice.NewConnectService(context.Background(), "static-server-sidecar", "static-server", 8080, clientNode) // bindPort not used
	require.NoError(t, err)

	serverServiceIP, _ := serverService.GetAddr()
	serverConnectProxyIP, _ := serverConnectProxy.GetAddr()

	// Register the service and sidecar
	req := &api.AgentServiceRegistration{
		Name: "static-server",
		Port: 8080,
		//Address: serverServiceIP,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Name: "static-server-sidecar-proxy",
				Port: 20000,
				Kind: api.ServiceKindConnectProxy,
				Checks: api.AgentServiceChecks{
					&api.AgentServiceCheck{
						Name:     "Connect Sidecar Listening",
						TCP:      fmt.Sprintf("%s:%d", serverConnectProxyIP, 20000),
						Interval: "10s",
					},
					&api.AgentServiceCheck{
						Name:         "Connect Sidecar Aliasing Static Server",
						AliasService: "static-server",
					},
				},
				Proxy: &api.AgentServiceConnectProxyConfig{
					DestinationServiceName: "static-server",
					LocalServiceAddress:    serverServiceIP,
					LocalServicePort:       8080,
				},
			},
		},
		Check: &api.AgentServiceCheck{
			Name:     "Connect Sidecar Listening",
			TCP:      fmt.Sprintf("%s:%d", serverServiceIP, 8080),
			Interval: "10s",
		},
	}

	require.NoError(t, client.Agent().ServiceRegister(req))

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

// createDialingCluster creates a cluster for peering with a single dev agent
func createDialingCluster(t *testing.T) (*libcluster.Cluster, *api.Client, libservice.Service) {
	configs := []libnode.Config{
		{
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
	_, err = libservice.NewGatewayService(context.Background(), "dc2-mesh", "mesh", node)
	require.NoError(t, err)

	// Create a service and proxy instance
	clientService, err := libservice.NewConnectService(context.Background(), "static-client-sidecar", "static-client", 5000, node)
	require.NoError(t, err)

	clientConnectProxyIP, _ := clientService.GetAddr()

	// Register the service and sidecar
	req := &api.AgentServiceRegistration{
		Name: "static-client",
		Port: 8080,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Name: "static-client-sidecar-proxy",
				Port: 20000,
				Kind: api.ServiceKindConnectProxy,
				Checks: api.AgentServiceChecks{
					&api.AgentServiceCheck{
						Name:     "Connect Sidecar Listening",
						TCP:      fmt.Sprintf("%s:%d", clientConnectProxyIP, 20000),
						Interval: "10s",
					},
				},
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: []api.Upstream{
						{
							DestinationName:  "static-server",
							DestinationPeer:  dialingPeerName,
							LocalBindAddress: "0.0.0.0",
							LocalBindPort:    5000,
							MeshGateway: api.MeshGatewayConfig{
								Mode: api.MeshGatewayModeLocal,
							},
						},
					},
				},
			},
		},
		Checks: api.AgentServiceChecks{},
	}
	require.NoError(t, client.Agent().ServiceRegister(req))

	return cluster, client, clientService
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
