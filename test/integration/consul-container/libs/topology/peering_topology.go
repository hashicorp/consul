// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package topology

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const (
	AcceptingPeerName = "accepting-to-dialer"
	DialingPeerName   = "dialing-to-acceptor"
)

type BuiltCluster struct {
	Cluster   *libcluster.Cluster
	Context   *libcluster.BuildContext
	Service   libservice.Service
	Container libservice.Service
	Gateway   libservice.Service
}

type PeeringClusterSize struct {
	AcceptingNumServers int
	AcceptingNumClients int
	DialingNumServers   int
	DialingNumClients   int
}

// BasicPeeringTwoClustersSetup sets up a scenario for testing peering, which consists of
//
//   - an accepting cluster with 3 servers and 1 client agent. The client should be used to
//     host a service for export: staticServerSvc.
//   - an dialing cluster with 1 server and 1 client. The client should be used to host a
//     service connecting to staticServerSvc.
//   - Create the peering, export the service from accepting cluster, and verify service
//     connectivity.
//
// It returns objects of the accepting cluster, dialing cluster, staticServerSvc, and staticClientSvcSidecar
func BasicPeeringTwoClustersSetup(
	t *testing.T,
	consulImage string,
	consulVersion string,
	pcs PeeringClusterSize,
	peeringThroughMeshgateway bool,
) (*BuiltCluster, *BuiltCluster) {
	acceptingCluster, acceptingCtx, acceptingClient := NewCluster(t, &ClusterConfig{
		NumServers: pcs.AcceptingNumServers,
		NumClients: pcs.AcceptingNumClients,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			ConsulImageName:      consulImage,
			ConsulVersion:        consulVersion,
			InjectAutoEncryption: true,
		},
		ApplyDefaultProxySettings: true,
	})

	dialingCluster, dialingCtx, dialingClient := NewCluster(t, &ClusterConfig{
		NumServers: pcs.DialingNumServers,
		NumClients: pcs.DialingNumClients,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc2",
			ConsulImageName:      consulImage,
			ConsulVersion:        consulVersion,
			InjectAutoEncryption: true,
		},
		ApplyDefaultProxySettings: true,
	})

	// Create the mesh gateway for dataplane traffic and peering control plane traffic (if enabled)
	gwCfg := libservice.GatewayConfig{
		Name: "mesh",
		Kind: "mesh",
	}
	acceptingClusterGateway, err := libservice.NewGatewayService(context.Background(), gwCfg, acceptingCluster.Clients()[0])
	require.NoError(t, err)
	dialingClusterGateway, err := libservice.NewGatewayService(context.Background(), gwCfg, dialingCluster.Clients()[0])
	require.NoError(t, err)

	// Enable peering control plane traffic through mesh gateway
	if peeringThroughMeshgateway {
		req := &api.MeshConfigEntry{
			Peering: &api.PeeringMeshConfig{
				PeerThroughMeshGateways: true,
			},
		}
		configCluster := func(cli *api.Client) error {
			libassert.CatalogServiceExists(t, cli, "mesh", nil)
			ok, _, err := cli.ConfigEntries().Set(req, &api.WriteOptions{})
			if !ok {
				return fmt.Errorf("config entry is not set")
			}

			if err != nil {
				return fmt.Errorf("error writing config entry: %s", err)
			}
			return nil
		}
		err = configCluster(dialingClient)
		require.NoError(t, err)
		err = configCluster(acceptingClient)
		require.NoError(t, err)
	}

	require.NoError(t, dialingCluster.PeerWithCluster(acceptingClient, AcceptingPeerName, DialingPeerName))

	libassert.PeeringStatus(t, acceptingClient, AcceptingPeerName, api.PeeringStateActive)
	// libassert.PeeringExports(t, acceptingClient, acceptingPeerName, 1)

	// Register an static-server service in acceptingCluster and export to dialing cluster
	var serverService, serverSidecarService libservice.Service
	{
		clientNode := acceptingCluster.Clients()[0]

		// Create a service and proxy instance
		var err error
		// Create a service and proxy instance
		serviceOpts := libservice.ServiceOpts{
			Name:     libservice.StaticServerServiceName,
			ID:       "static-server",
			Meta:     map[string]string{"version": ""},
			HTTPPort: 8080,
			GRPCPort: 8079,
		}
		serverService, serverSidecarService, err = libservice.CreateAndRegisterStaticServerAndSidecar(clientNode, &serviceOpts)
		require.NoError(t, err)

		libassert.CatalogServiceExists(t, acceptingClient, libservice.StaticServerServiceName, nil)
		libassert.CatalogServiceExists(t, acceptingClient, "static-server-sidecar-proxy", nil)

		require.NoError(t, serverService.Export("default", AcceptingPeerName, acceptingClient))
	}

	// Register an static-client service in dialing cluster and set upstream to static-server service
	var clientSidecarService *libservice.ConnectContainer
	{
		clientNode := dialingCluster.Clients()[0]

		// Create a service and proxy instance
		var err error
		clientSidecarService, err = libservice.CreateAndRegisterStaticClientSidecar(clientNode, DialingPeerName, true, false)
		require.NoError(t, err)

		libassert.CatalogServiceExists(t, dialingClient, "static-client-sidecar-proxy", nil)

	}

	_, adminPort := clientSidecarService.GetAdminAddr()
	libassert.AssertUpstreamEndpointStatus(t, adminPort, fmt.Sprintf("static-server.default.%s.external", DialingPeerName), "HEALTHY", 1)
	_, port := clientSidecarService.GetAddr()
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")

	return &BuiltCluster{
			Cluster:   acceptingCluster,
			Context:   acceptingCtx,
			Service:   serverSidecarService,
			Container: serverSidecarService,
			Gateway:   acceptingClusterGateway,
		},
		&BuiltCluster{
			Cluster:   dialingCluster,
			Context:   dialingCtx,
			Service:   nil,
			Container: clientSidecarService,
			Gateway:   dialingClusterGateway,
		}
}

type ClusterConfig struct {
	NumServers                int
	NumClients                int
	ApplyDefaultProxySettings bool
	BuildOpts                 *libcluster.BuildOptions
	Cmd                       string
	LogConsumer               *TestLogConsumer

	// Exposed Ports are available on the cluster's pause container for the purposes
	// of adding external communication to the cluster. An example would be a listener
	// on a gateway.
	ExposedPorts []int
}

// NewCluster creates a cluster with peering enabled. It also creates
// and registers a mesh-gateway at the client agent. The API client returned is
// pointed at the client agent.
// - proxy-defaults.protocol = tcp
func NewCluster(
	t *testing.T,
	config *ClusterConfig,
) (*libcluster.Cluster, *libcluster.BuildContext, *api.Client) {
	var (
		cluster *libcluster.Cluster
		err     error
	)
	require.NotEmpty(t, config.BuildOpts.Datacenter)
	require.True(t, config.NumServers > 0)

	opts := libcluster.BuildOptions{
		Datacenter:             config.BuildOpts.Datacenter,
		InjectAutoEncryption:   config.BuildOpts.InjectAutoEncryption,
		InjectGossipEncryption: true,
		AllowHTTPAnyway:        true,
		ConsulVersion:          config.BuildOpts.ConsulVersion,
		ACLEnabled:             config.BuildOpts.ACLEnabled,
		LogStore:               config.BuildOpts.LogStore,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	serverConf := libcluster.NewConfigBuilder(ctx).
		Bootstrap(config.NumServers).
		Peering(true).
		ToAgentConfig(t)
	t.Logf("%s server config: \n%s", opts.Datacenter, serverConf.JSON)

	// optional
	if config.LogConsumer != nil {
		serverConf.LogConsumer = config.LogConsumer
	}

	t.Logf("Cluster config:\n%s", serverConf.JSON)

	// optional custom cmd
	if config.Cmd != "" {
		serverConf.Cmd = append(serverConf.Cmd, config.Cmd)
	}

	if config.ExposedPorts != nil {
		cluster, err = libcluster.New(t, []libcluster.Config{*serverConf}, config.ExposedPorts...)
	} else {
		cluster, err = libcluster.NewN(t, *serverConf, config.NumServers)
	}
	require.NoError(t, err)
	// builder generates certs for us, so copy them back
	if opts.InjectAutoEncryption {
		cluster.CACert = serverConf.CACert
	}

	var retryJoin []string
	for i := 0; i < config.NumServers; i++ {
		retryJoin = append(retryJoin, fmt.Sprintf("agent-%d", i))
	}

	// Add numClients static clients to register the service
	configbuiilder := libcluster.NewConfigBuilder(ctx).
		Client().
		Peering(true).
		RetryJoin(retryJoin...)
	clientConf := configbuiilder.ToAgentConfig(t)
	t.Logf("%s client config: \n%s", opts.Datacenter, clientConf.JSON)

	require.NoError(t, cluster.AddN(*clientConf, config.NumClients, true))

	// Use the client agent as the HTTP endpoint since we will not rotate it in many tests.
	var client *api.Client
	if config.NumClients > 0 {
		clientNode := cluster.Agents[config.NumServers]
		client = clientNode.GetClient()
	} else {
		client = cluster.Agents[0].GetClient()
	}
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, config.NumServers+config.NumClients)

	// Default Proxy Settings
	if config.ApplyDefaultProxySettings {
		ok, err := utils.ApplyDefaultProxySettings(client)
		require.NoError(t, err)
		require.True(t, ok)
	}

	return cluster, ctx, client
}

type TestLogConsumer struct {
	Msgs []string
}

func (g *TestLogConsumer) Accept(l testcontainers.Log) {
	g.Msgs = append(g.Msgs, string(l.Content))
}
