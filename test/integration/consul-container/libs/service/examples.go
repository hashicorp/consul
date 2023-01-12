package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
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

var _ Service = (*exampleContainer)(nil)

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

func (c exampleContainer) Terminate() error {
	return cluster.TerminateContainer(c.ctx, c.container, true)
}

func (g exampleContainer) Export(partition, peerName string, client *api.Client) error {
	config := &api.ExportedServicesConfigEntry{
		Name: partition,
		Services: []api.ExportedService{{
			Name: g.GetServiceName(),
			Consumers: []api.ServiceConsumer{
				// TODO: need to handle the changed field name in 1.13
				{Peer: peerName},
			},
		}},
	}

	_, _, err := client.ConfigEntries().Set(config, &api.WriteOptions{})
	return err
}

func (g exampleContainer) GetServiceName() string {
	return g.serviceName
}

func NewExampleService(ctx context.Context, name string, httpPort int, grpcPort int, node libcluster.Agent) (Service, error) {
	namePrefix := fmt.Sprintf("%s-service-example-%s", node.GetDatacenter(), name)
	containerName := utils.RandName(namePrefix)

	pod := node.GetPod()
	if pod == nil {
		return nil, fmt.Errorf("node Pod is required")
	}

	var (
		httpPortStr = strconv.Itoa(httpPort)
		grpcPortStr = strconv.Itoa(grpcPort)
	)

	req := testcontainers.ContainerRequest{
		Image:      hashicorpDockerProxy + "/fortio/fortio",
		WaitingFor: wait.ForLog("").WithStartupTimeout(10 * time.Second),
		AutoRemove: false,
		Name:       containerName,
		Cmd: []string{
			"server",
			"-http-port", httpPortStr,
			"-grpc-port", grpcPortStr,
			"-redirect-port", "-disabled",
		},
		Env: map[string]string{"FORTIO_NAME": name},
	}

	info, err := cluster.LaunchContainerOnNode(ctx, node, req, []string{httpPortStr, grpcPortStr})
	if err != nil {
		return nil, err
	}

	out := &exampleContainer{
		ctx:         ctx,
		container:   info.Container,
		ip:          info.IP,
		httpPort:    info.MappedPorts[httpPortStr].Int(),
		grpcPort:    info.MappedPorts[grpcPortStr].Int(),
		serviceName: name,
	}

	fmt.Printf("Example service exposed http port %d, gRPC port %d\n", out.httpPort, out.grpcPort)

	return out, nil
}
