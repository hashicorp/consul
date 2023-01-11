package service

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/consul/api"
)

// grpc, if true, will configure and advertise the GRPC port (8079) instead of the HTTP port
func CreateAndRegisterStaticServerAndSidecar(node Agent, grpc bool) (Service, Service, error) {
	// Create a service and proxy instance
	serverService, err := NewExampleService(context.Background(), "static-server", 8080, 8079, node)
	if err != nil {
		return nil, nil, err
	}

	port := 8080
	if grpc {
		port = 8079
	}

	serverConnectProxy, err := NewConnectService(context.Background(), "static-server-sidecar", "static-server", port, node) // bindPort not used
	if err != nil {
		return nil, nil, err
	}

	serverServiceIP, _ := serverService.GetAddr()
	serverConnectProxyIP, _ := serverConnectProxy.GetAddr()
	agentCheck := api.AgentServiceCheck{
		Name:     "Static Server Listening",
		HTTP:     fmt.Sprintf("%s:%d", serverServiceIP, port),
		Interval: "10s",
		Status:   api.HealthPassing,
	}
	if grpc {
		agentCheck.HTTP = ""
		agentCheck.GRPC = fmt.Sprintf("%s:%d", serverServiceIP, 8079)
	}

	// Register the static-server service and sidecar
	req := &api.AgentServiceRegistration{
		Name:    "static-server",
		Port:    port,
		Address: serverServiceIP,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Name:    "static-server-sidecar-proxy",
				Port:    20000,
				Address: serverConnectProxyIP,
				Kind:    api.ServiceKindConnectProxy,
				Checks: api.AgentServiceChecks{
					&api.AgentServiceCheck{
						Name:     "Connect Sidecar Listening",
						TCP:      fmt.Sprintf("%s:%d", serverConnectProxyIP, 20000),
						Interval: "10s",
						Status:   api.HealthPassing,
					},
					&api.AgentServiceCheck{
						Name:         "Connect Sidecar Aliasing Static Server",
						AliasService: "static-server",
						Status:       api.HealthPassing,
					},
				},
				Proxy: &api.AgentServiceConnectProxyConfig{
					DestinationServiceName: "static-server",
					LocalServiceAddress:    serverServiceIP,
					LocalServicePort:       port,
				},
			},
		},
		Check: &agentCheck,
	}

	err = node.GetClient().Agent().ServiceRegister(req)
	if err != nil {
		return serverService, serverConnectProxy, err
	}

	return serverService, serverConnectProxy, nil
}

func CreateAndRegisterStaticClientSidecar(node Agent, peerName string, localMeshGateway bool) (*ConnectContainer, error) {
	// Create a service and proxy instance
	clientConnectProxy, err := NewConnectService(context.Background(), "static-client-sidecar", "static-client", 5000, node)
	if err != nil {
		return nil, err
	}

	clientConnectProxyIP, _ := clientConnectProxy.GetAddr()

	mgwMode := api.MeshGatewayModeRemote
	if localMeshGateway {
		mgwMode = api.MeshGatewayModeLocal
	}

	// Register the static-client service and sidecar
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
						Status:   api.HealthPassing,
					},
				},
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: []api.Upstream{
						{
							DestinationName:  "static-server",
							DestinationPeer:  peerName,
							LocalBindAddress: "0.0.0.0",
							LocalBindPort:    5000,
							MeshGateway: api.MeshGatewayConfig{
								Mode: mgwMode,
							},
						},
					},
				},
			},
		},
		Checks: api.AgentServiceChecks{},
	}

	err = node.GetClient().Agent().ServiceRegister(req)
	if err != nil {
		return clientConnectProxy, err
	}

	return clientConnectProxy, nil
}

func GetEnvoyConfigDump(port int) (string, error) {
	client := http.DefaultClient
	url := fmt.Sprintf("http://localhost:%d/config_dump?include_eds", port)

	res, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
