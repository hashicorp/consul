// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const (
	StaticServerServiceName  = "static-server"
	StaticServer2ServiceName = "static-server-2"
	StaticClientServiceName  = "static-client"
)

type Checks struct {
	Name string
	TTL  string
}

type SidecarService struct {
	Port  int
	Proxy ConnectProxy
}

type ConnectProxy struct {
	Mode string
}

type ServiceOpts struct {
	Name     string
	ID       string
	Meta     map[string]string
	HTTPPort int
	GRPCPort int
	// if true, register GRPC port instead of HTTP (default)
	RegisterGRPC bool
	Checks       Checks
	Connect      SidecarService
	Namespace    string
	Locality     *api.Locality
}

// createAndRegisterStaticServerAndSidecar register the services and launch static-server containers
func createAndRegisterStaticServerAndSidecar(node libcluster.Agent, httpPort int, grpcPort int, svc *api.AgentServiceRegistration, customContainerCfg func(testcontainers.ContainerRequest) testcontainers.ContainerRequest, containerArgs ...string) (Service, Service, error) {
	// Do some trickery to ensure that partial completion is correctly torn
	// down, but successful execution is not.
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	if err := node.GetClient().Agent().ServiceRegister(svc); err != nil {
		return nil, nil, err
	}

	// Create a service and proxy instance
	serverService, err := NewExampleService(context.Background(), svc.ID, httpPort, grpcPort, node, containerArgs...)
	if err != nil {
		return nil, nil, err
	}
	deferClean.Add(func() {
		_ = serverService.Terminate()
	})
	sidecarCfg := SidecarConfig{
		Name:      fmt.Sprintf("%s-sidecar", svc.ID),
		ServiceID: svc.ID,
		Namespace: svc.Namespace,
		EnableTProxy: svc.Connect != nil &&
			svc.Connect.SidecarService != nil &&
			svc.Connect.SidecarService.Proxy != nil &&
			svc.Connect.SidecarService.Proxy.Mode == api.ProxyModeTransparent,
	}
	serverConnectProxy, err := NewConnectService(context.Background(), sidecarCfg, []int{svc.Port}, node, customContainerCfg) // bindPort not used
	if err != nil {
		return nil, nil, err
	}

	deferClean.Add(func() {
		_ = serverConnectProxy.Terminate()
	})

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return serverService, serverConnectProxy, nil
}

// createAndRegisterCustomServiceAndSidecar creates a custom service from the given testcontainers.ContainerRequest
// and a sidecar proxy for the service. The customContainerCfg parameter is used to mutate the
// testcontainers.ContainerRequest for the sidecar proxy.
func createAndRegisterCustomServiceAndSidecar(node libcluster.Agent,
	httpPort int,
	grpcPort int,
	svc *api.AgentServiceRegistration,
	request testcontainers.ContainerRequest,
	customContainerCfg func(testcontainers.ContainerRequest) testcontainers.ContainerRequest,
) (Service, Service, error) {
	// Do some trickery to ensure that partial completion is correctly torn
	// down, but successful execution is not.
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	if err := node.GetClient().Agent().ServiceRegister(svc); err != nil {
		return nil, nil, err
	}

	// Create a service and proxy instance
	serverService, err := NewCustomService(context.Background(), svc.ID, httpPort, grpcPort, node, request)
	if err != nil {
		return nil, nil, err
	}
	deferClean.Add(func() {
		_ = serverService.Terminate()
	})
	sidecarCfg := SidecarConfig{
		Name:      fmt.Sprintf("%s-sidecar", svc.ID),
		ServiceID: svc.ID,
		Namespace: svc.Namespace,
		EnableTProxy: svc.Connect != nil &&
			svc.Connect.SidecarService != nil &&
			svc.Connect.SidecarService.Proxy != nil &&
			svc.Connect.SidecarService.Proxy.Mode == api.ProxyModeTransparent,
	}
	serverConnectProxy, err := NewConnectService(context.Background(), sidecarCfg, []int{svc.Port}, node, customContainerCfg) // bindPort not used
	if err != nil {
		return nil, nil, err
	}

	deferClean.Add(func() {
		_ = serverConnectProxy.Terminate()
	})

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return serverService, serverConnectProxy, nil
}

func CreateAndRegisterCustomServiceAndSidecar(node libcluster.Agent,
	serviceOpts *ServiceOpts,
	request testcontainers.ContainerRequest,
	customContainerCfg func(testcontainers.ContainerRequest) testcontainers.ContainerRequest) (Service, Service, error) {
	// Register the static-server service and sidecar first to prevent race with sidecar
	// trying to get xDS before it's ready
	p := serviceOpts.HTTPPort
	agentCheck := api.AgentServiceCheck{
		Name:     "Static Server Listening",
		TCP:      fmt.Sprintf("127.0.0.1:%d", p),
		Interval: "10s",
		Status:   api.HealthPassing,
	}
	if serviceOpts.RegisterGRPC {
		p = serviceOpts.GRPCPort
		agentCheck.TCP = ""
		agentCheck.GRPC = fmt.Sprintf("127.0.0.1:%d", p)
	}
	req := &api.AgentServiceRegistration{
		Name: serviceOpts.Name,
		ID:   serviceOpts.ID,
		Port: p,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Mode: api.ProxyMode(serviceOpts.Connect.Proxy.Mode),
				},
			},
		},
		Namespace: serviceOpts.Namespace,
		Meta:      serviceOpts.Meta,
		Check:     &agentCheck,
	}
	return createAndRegisterCustomServiceAndSidecar(node, serviceOpts.HTTPPort, serviceOpts.GRPCPort, req, request, customContainerCfg)
}

// CreateAndRegisterStaticServerAndSidecarWithCustomContainerConfig creates an example static server and a sidecar for
// the service. The customContainerCfg parameter is a function of testcontainers.ContainerRequest to
// testcontainers.ContainerRequest which can be used to mutate the container request for the sidecar proxy and inject
// custom configuration and lifecycle hooks.
func CreateAndRegisterStaticServerAndSidecarWithCustomContainerConfig(node libcluster.Agent,
	serviceOpts *ServiceOpts,
	customContainerCfg func(testcontainers.ContainerRequest) testcontainers.ContainerRequest,
	containerArgs ...string) (Service, Service, error) {
	// Register the static-server service and sidecar first to prevent race with sidecar
	// trying to get xDS before it's ready
	p := serviceOpts.HTTPPort
	agentCheck := api.AgentServiceCheck{
		Name:     "Static Server Listening",
		TCP:      fmt.Sprintf("127.0.0.1:%d", p),
		Interval: "10s",
		Status:   api.HealthPassing,
	}
	if serviceOpts.RegisterGRPC {
		p = serviceOpts.GRPCPort
		agentCheck.TCP = ""
		agentCheck.GRPC = fmt.Sprintf("127.0.0.1:%d", p)
	}
	req := &api.AgentServiceRegistration{
		Name: serviceOpts.Name,
		ID:   serviceOpts.ID,
		Port: p,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Mode: api.ProxyMode(serviceOpts.Connect.Proxy.Mode),
				},
			},
		},
		Namespace: serviceOpts.Namespace,
		Meta:      serviceOpts.Meta,
		Check:     &agentCheck,
		Locality:  serviceOpts.Locality,
	}
	return createAndRegisterStaticServerAndSidecar(node, serviceOpts.HTTPPort, serviceOpts.GRPCPort, req, customContainerCfg, containerArgs...)
}

func CreateAndRegisterStaticServerAndSidecar(node libcluster.Agent, serviceOpts *ServiceOpts, containerArgs ...string) (Service, Service, error) {
	return CreateAndRegisterStaticServerAndSidecarWithCustomContainerConfig(node, serviceOpts, nil, containerArgs...)
}

func CreateAndRegisterStaticServerAndSidecarWithChecks(node libcluster.Agent, serviceOpts *ServiceOpts) (Service, Service, error) {
	// Register the static-server service and sidecar first to prevent race with sidecar
	// trying to get xDS before it's ready
	req := &api.AgentServiceRegistration{
		Name: serviceOpts.Name,
		ID:   serviceOpts.ID,
		Port: serviceOpts.HTTPPort,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Mode: api.ProxyMode(serviceOpts.Connect.Proxy.Mode),
				},
				Port: serviceOpts.Connect.Port,
			},
		},
		Checks: api.AgentServiceChecks{
			{
				Name: serviceOpts.Checks.Name,
				TTL:  serviceOpts.Checks.TTL,
			},
		},
		Meta: serviceOpts.Meta,
	}

	return createAndRegisterStaticServerAndSidecar(node, serviceOpts.HTTPPort, serviceOpts.GRPCPort, req, nil)
}

func CreateAndRegisterStaticClientSidecar(
	node libcluster.Agent,
	peerName string,
	localMeshGateway bool,
	enableTProxy bool,
) (*ConnectContainer, error) {
	// Do some trickery to ensure that partial completion is correctly torn
	// down, but successful execution is not.
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	var proxy *api.AgentServiceConnectProxyConfig
	if enableTProxy {
		proxy = &api.AgentServiceConnectProxyConfig{
			Mode: "transparent",
		}
	} else {
		mgwMode := api.MeshGatewayModeRemote
		if localMeshGateway {
			mgwMode = api.MeshGatewayModeLocal
		}
		proxy = &api.AgentServiceConnectProxyConfig{
			Upstreams: []api.Upstream{{
				DestinationName:  StaticServerServiceName,
				DestinationPeer:  peerName,
				LocalBindAddress: "0.0.0.0",
				LocalBindPort:    libcluster.ServiceUpstreamLocalBindPort,
				MeshGateway: api.MeshGatewayConfig{
					Mode: mgwMode,
				},
			}},
		}
	}

	// Register the static-client service and sidecar first to prevent race with sidecar
	// trying to get xDS before it's ready
	req := &api.AgentServiceRegistration{
		Name: StaticClientServiceName,
		Port: 8080,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: proxy,
			},
		},
	}

	if err := node.GetClient().Agent().ServiceRegister(req); err != nil {
		return nil, err
	}

	// Create a service and proxy instance
	sidecarCfg := SidecarConfig{
		Name:         fmt.Sprintf("%s-sidecar", StaticClientServiceName),
		ServiceID:    StaticClientServiceName,
		EnableTProxy: enableTProxy,
	}

	clientConnectProxy, err := NewConnectService(context.Background(), sidecarCfg, []int{libcluster.ServiceUpstreamLocalBindPort}, node, nil)
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

func ClientsCreate(t *testing.T, numClients int, image, version string, cluster *libcluster.Cluster) {
	opts := libcluster.BuildOptions{
		ConsulImageName: image,
		ConsulVersion:   version,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	conf := libcluster.NewConfigBuilder(ctx).
		Client().
		ToAgentConfig(t)
	t.Logf("Cluster client config:\n%s", conf.JSON)

	require.NoError(t, cluster.AddN(*conf, numClients, true))
}

func ServiceCreate(t *testing.T, client *api.Client, serviceName string) uint64 {
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: serviceName,
		Port: 9999,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Port: 22005,
			},
		},
	}))

	service, meta, err := client.Catalog().Service(serviceName, "", &api.QueryOptions{})
	require.NoError(t, err)
	require.Len(t, service, 1)
	require.Equal(t, serviceName, service[0].ServiceName)
	require.Equal(t, 9999, service[0].ServicePort)

	return meta.LastIndex
}

func ServiceHealthBlockingQuery(client *api.Client, serviceName string, waitIndex uint64) (chan []*api.ServiceEntry, chan error) {
	var (
		ch    = make(chan []*api.ServiceEntry, 1)
		errCh = make(chan error, 1)
	)
	go func() {
		opts := &api.QueryOptions{WaitIndex: waitIndex}
		service, q, err := client.Health().Service(serviceName, "", false, opts)
		if err == nil && q.QueryBackend != api.QueryBackendStreaming {
			err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
		}
		if err != nil {
			errCh <- err
		} else {
			ch <- service
		}
	}()

	return ch, errCh
}
