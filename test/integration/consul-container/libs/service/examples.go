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

// exampleContainer
type exampleContainer struct {
	ctx         context.Context
	container   testcontainers.Container
	ip          string
	httpPort    int
	grpcPort    int
	serviceName string
}

func (g exampleContainer) GetName() string {
	name, err := g.container.Name(g.ctx)
	if err != nil {
		return ""
	}
	return name
}

func (g exampleContainer) GetAddr() (string, int) {
	return g.ip, g.httpPort
}

func (g exampleContainer) Start() error {
	if g.container == nil {
		return fmt.Errorf("container has not been initialized")
	}
	return g.container.Start(context.Background())
}

// Terminate attempts to terminate the container. On failure, an error will be
// returned and the reaper process (RYUK) will handle cleanup.
func (c exampleContainer) Terminate() error {
	if c.container == nil {
		return nil
	}

	var err error
	if *utils.FollowLog {
		err = c.container.StopLogProducer()
		if err1 := c.container.Terminate(c.ctx); err1 == nil {
			err = err1
		}
	} else {
		err = c.container.Terminate(c.ctx)
	}

	c.container = nil

	return err
}

func (g exampleContainer) Export(partition, peerName string, client *api.Client) error {
	config := &api.ExportedServicesConfigEntry{
		Name: partition,
		Services: []api.ExportedService{
			{
				Name: g.GetServiceName(),
				Consumers: []api.ServiceConsumer{
					// TODO: need to handle the changed field name in 1.13
					{Peer: peerName},
				},
			},
		},
	}

	_, _, err := client.ConfigEntries().Set(config, &api.WriteOptions{})
	return err
}

func (g exampleContainer) GetServiceName() string {
	return g.serviceName
}

func NewExampleService(ctx context.Context, name string, httpPort int, grpcPort int, node libnode.Agent) (Service, error) {
	namePrefix := fmt.Sprintf("%s-service-example-%s", node.GetDatacenter(), name)
	containerName := utils.RandName(namePrefix)

	req := testcontainers.ContainerRequest{
		Image:      hashicorpDockerProxy + "/fortio/fortio",
		WaitingFor: wait.ForLog("").WithStartupTimeout(10 * time.Second),
		AutoRemove: false,
		Name:       containerName,
		Cmd:        []string{"server", "-http-port", fmt.Sprintf("%d", httpPort), "-grpc-port", fmt.Sprintf("%d", grpcPort), "-redirect-port", "-disabled"},
		Env:        map[string]string{"FORTIO_NAME": name},
		ExposedPorts: []string{
			fmt.Sprintf("%d/tcp", httpPort), // HTTP Listener
			fmt.Sprintf("%d/tcp", grpcPort), // GRPC Listener
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
	mappedHTPPPort, err := container.MappedPort(ctx, nat.Port(fmt.Sprintf("%d", httpPort)))
	if err != nil {
		return nil, err
	}

	mappedGRPCPort, err := container.MappedPort(ctx, nat.Port(fmt.Sprintf("%d", grpcPort)))
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

	terminate := func() error {
		return container.Terminate(context.Background())
	}
	node.RegisterTermination(terminate)

	fmt.Printf("Example service exposed http port %d, gRPC port %d\n", mappedHTPPPort.Int(), mappedGRPCPort.Int())
	return &exampleContainer{container: container, ip: ip, httpPort: mappedHTPPPort.Int(), grpcPort: mappedGRPCPort.Int(), serviceName: name}, nil
}
