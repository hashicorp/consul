package service

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	libnode "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// gatewayContainer
type gatewayContainer struct {
	ctx       context.Context
	container testcontainers.Container
	ip        string
	port      int
	req       testcontainers.ContainerRequest
}

func (g gatewayContainer) GetName() string {
	name, err := g.container.Name(g.ctx)
	if err != nil {
		return ""
	}
	return name
}

func (g gatewayContainer) GetAddr() (string, int) {
	return g.ip, g.port
}

func (g gatewayContainer) Start() error {
	if g.container == nil {
		return fmt.Errorf("container has not been initialized")
	}
	return g.container.Start(context.Background())
}

// Terminate attempts to terminate the container. On failure, an error will be
// returned and the reaper process (RYUK) will handle cleanup.
func (c gatewayContainer) Terminate() error {
	if c.container == nil {
		return nil
	}

	err := c.container.StopLogProducer()

	if err1 := c.container.Terminate(c.ctx); err == nil {
		err = err1
	}

	c.container = nil

	return err
}

func NewGatewayService(ctx context.Context, name string, kind string, node libnode.Agent) (Service, error) {
	namePrefix := fmt.Sprintf("%s-service-gateway-%s", node.GetDatacenter(), name)
	containerName := utils.RandName(namePrefix)

	envoyVersion := getEnvoyVersion()
	agentConfig := node.GetConfig()
	buildargs := map[string]*string{
		"ENVOY_VERSION": utils.StringToPointer(envoyVersion),
		"CONSUL_IMAGE": utils.StringToPointer(agentConfig.Image),
	}

	dockerfileCtx, err := getDevContainerDockerfile()
	if err != nil {
		return nil, err
	}
	dockerfileCtx.BuildArgs = buildargs

	nodeIP, _ := node.GetAddr()

	req := testcontainers.ContainerRequest{
		FromDockerfile: dockerfileCtx,
		WaitingFor:     wait.ForLog("").WithStartupTimeout(100 * time.Second),
		AutoRemove:     false,
		Name:           containerName,
		Cmd: []string{
			"consul", "connect", "envoy",
			fmt.Sprintf("-gateway=%s", kind),
			"-register",
			"-service", name,
			"-address", "{{ GetInterfaceIP \"eth0\" }}:8443",
			fmt.Sprintf("-grpc-addr=%s:%d", nodeIP, 8502),
			"-admin-bind", "0.0.0.0:19000",
			"--",
			"--log-level", envoyLogLevel},
		Env: map[string]string{"CONSUL_HTTP_ADDR": fmt.Sprintf("%s:%d", nodeIP, 8500)},
		ExposedPorts: []string{
			"8443/tcp",  // Envoy Gateway Listener
			"19000/tcp", // Envoy Admin Port
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
	mappedPort, err := container.MappedPort(ctx, "8443")
	if err != nil {
		return nil, err
	}

	if err := container.StartLogProducer(ctx); err != nil {
		return nil, err
	}
	container.FollowOutput(&LogConsumer{
		Prefix: containerName,
	})

	terminate := func() error {
		return container.Terminate(context.Background())
	}
	node.RegisterTermination(terminate)

	return &gatewayContainer{container: container, ip: ip, port: mappedPort.Int()}, nil
}
