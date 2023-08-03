package upgrade

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// CreateAndRegisterStaticClientSidecarWith2Upstreams creates a static-client that
// has two upstreams connecting to destinationNames: local bind addresses are 5000
// and 5001.
// - crossCluster: true if upstream is in another cluster
func CreateAndRegisterStaticClientSidecarWith2Upstreams(c *cluster.Cluster, destinationNames []string, crossCluster bool) (*libservice.ConnectContainer, error) {
	// Do some trickery to ensure that partial completion is correctly torn
	// down, but successful execution is not.
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	node := c.Servers()[0]
	mgwMode := api.MeshGatewayModeLocal

	// Register the static-client service and sidecar first to prevent race with sidecar
	// trying to get xDS before it's ready
	req := &api.AgentServiceRegistration{
		Name: libservice.StaticClientServiceName,
		Port: 8080,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: []api.Upstream{
						{
							DestinationName:  destinationNames[0],
							LocalBindAddress: "0.0.0.0",
							LocalBindPort:    cluster.ServiceUpstreamLocalBindPort,
						},
						{
							DestinationName:  destinationNames[1],
							LocalBindAddress: "0.0.0.0",
							LocalBindPort:    cluster.ServiceUpstreamLocalBindPort2,
						},
					},
				},
			},
		},
	}

	if crossCluster {
		for _, upstream := range req.Connect.SidecarService.Proxy.Upstreams {
			upstream.MeshGateway = api.MeshGatewayConfig{
				Mode: mgwMode,
			}
		}
	}

	if err := node.GetClient().Agent().ServiceRegister(req); err != nil {
		return nil, err
	}

	// Create a service and proxy instance
	sidecarCfg := libservice.SidecarConfig{
		Name:      fmt.Sprintf("%s-sidecar", libservice.StaticClientServiceName),
		ServiceID: libservice.StaticClientServiceName,
	}

	clientConnectProxy, err := libservice.NewConnectService(context.Background(), sidecarCfg, []int{cluster.ServiceUpstreamLocalBindPort, cluster.ServiceUpstreamLocalBindPort2}, node)
	if err != nil {
		return nil, err
	}
	deferClean.Add(func() {
		_ = clientConnectProxy.Terminate()
	})

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return clientConnectProxy, nil
}
