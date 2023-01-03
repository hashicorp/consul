package service

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/consul/api"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	libnode "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// ConnectContainer
type ConnectContainer struct {
	ctx         context.Context
	container   testcontainers.Container
	ip          string
	appPort     int
	adminPort   int
	req         testcontainers.ContainerRequest
	serviceName string
}

func (g ConnectContainer) GetName() string {
	name, err := g.container.Name(g.ctx)
	if err != nil {
		return ""
	}
	return name
}

func (g ConnectContainer) GetAddr() (string, int) {
	return g.ip, g.appPort
}

func (g ConnectContainer) Start() error {
	if g.container == nil {
		return fmt.Errorf("container has not been initialized")
	}
	return g.container.Start(context.Background())
}

func (g ConnectContainer) GetAdminAddr() (string, int) {
	return "localhost", g.adminPort
}

// Terminate attempts to terminate the container. On failure, an error will be
// returned and the reaper process (RYUK) will handle cleanup.
func (c ConnectContainer) Terminate() error {
	if c.container == nil {
		return nil
	}

	var err error
	if *utils.FollowLog {
		err := c.container.StopLogProducer()
		if err1 := c.container.Terminate(c.ctx); err == nil {
			err = err1
		}
	} else {
		err = c.container.Terminate(c.ctx)
	}

	c.container = nil

	return err
}

func (g ConnectContainer) Export(partition, peer string, client *api.Client) error {
	return fmt.Errorf("ConnectContainer export unimplemented")
}

func (g ConnectContainer) GetServiceName() string {
	return g.serviceName
}

// NewConnectService returns a container that runs envoy sidecar, launched by
// "consul connect envoy", for service name (serviceName) on the specified
// node. The container exposes port serviceBindPort and envoy admin port (19000)
// by mapping them onto host ports. The container's name has a prefix
// combining datacenter and name.
func NewConnectService(ctx context.Context, name string, serviceName string, serviceBindPort int, node libnode.Agent) (*ConnectContainer, error) {
	namePrefix := fmt.Sprintf("%s-service-connect-%s", node.GetDatacenter(), name)
	containerName := utils.RandName(namePrefix)

	envoyVersion := getEnvoyVersion()
	buildargs := map[string]*string{
		"ENVOY_VERSION": utils.StringToPointer(envoyVersion),
	}

	dockerfileCtx, err := getDevContainerDockerfile()
	if err != nil {
		return nil, err
	}
	dockerfileCtx.BuildArgs = buildargs

	nodeIP, _ := node.GetAddr()

	req := testcontainers.ContainerRequest{
		FromDockerfile: dockerfileCtx,
		WaitingFor:     wait.ForLog("").WithStartupTimeout(10 * time.Second),
		AutoRemove:     false,
		Name:           containerName,
		Cmd: []string{
			"consul", "connect", "envoy",
			"-sidecar-for", serviceName,
			"-admin-bind", "0.0.0.0:19000",
			"-grpc-addr", fmt.Sprintf("%s:8502", nodeIP),
			"-http-addr", fmt.Sprintf("%s:8500", nodeIP),
			"--",
			"--log-level", envoyLogLevel},
		ExposedPorts: []string{
			fmt.Sprintf("%d/tcp", serviceBindPort), // Envoy Listener
			"19000/tcp",                            // Envoy Admin Port
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	ip, err := container.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}

	mappedAppPort, err := container.MappedPort(ctx, nat.Port(fmt.Sprintf("%d", serviceBindPort)))
	if err != nil {
		return nil, err
	}
	mappedAdminPort, err := container.MappedPort(ctx, nat.Port(fmt.Sprintf("%d", 19000)))
	if err != nil {
		return nil, err
	}

	if *utils.FollowLog {
		if err := container.StartLogProducer(ctx); err != nil {
			return nil, err
		}
		container.FollowOutput(&LogConsumer{
			Prefix: containerName,
		})
	}

	// Register the termination function the agent so the containers can stop together
	terminate := func() error {
		return container.Terminate(context.Background())
	}
	node.RegisterTermination(terminate)

	fmt.Printf("NewConnectService: name %s, mappedAppPort %d, bind port %d\n",
		serviceName, mappedAppPort.Int(), serviceBindPort)

	return &ConnectContainer{
		container:   container,
		ip:          ip,
		appPort:     mappedAppPort.Int(),
		adminPort:   mappedAdminPort.Int(),
		serviceName: name,
	}, nil
}
